package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	internaljwt "crm-distributed/cmd/task-service/internal/jwt"
	"crm-distributed/shared/domain"
	"crm-distributed/shared/pkg/redis"
)

const (
	refreshTokenPrefix   = "refresh:"
	validationCodePrefix = "validation:"
)

var (
	ErrEmailAlreadyExists  = errors.New("пользователь с таким email уже существует")
	ErrInvalidCredentials  = errors.New("неверный email или пароль")
	ErrEmailNotConfirmed   = errors.New("необходимо подтвердить email")
	ErrInvalidRefreshToken = errors.New("невалидный refresh токен")
)

type AuthUsecase interface {
	Register(ctx context.Context, cmd RegisterCommand) (uuid.UUID, error)
	Login(ctx context.Context, cmd LoginCommand) (*TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (string, error)
	Logout(ctx context.Context, refreshToken string) error
}

type RegisterCommand struct {
	Name     string
	Lname    string
	Pname    string
	Email    string
	Phone    int64
	Password string
}

type LoginCommand struct {
	Email      string
	Password   string
	RememberMe bool
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	UserUUID     uuid.UUID
}

type authUsecase struct {
	userRepo   UserRepository
	jwtService *internaljwt.Service
	redis      *redis.Client
	log        *slog.Logger
	isDev      bool
}

func NewUsecase(
	userRepo UserRepository,
	jwtService *internaljwt.Service,
	rdb *redis.Client,
	log *slog.Logger,
	isDev bool,
) AuthUsecase {
	return &authUsecase{
		userRepo:   userRepo,
		jwtService: jwtService,
		redis:      rdb,
		log:        log,
		isDev:      isDev,
	}
}

func (u *authUsecase) Register(ctx context.Context, cmd RegisterCommand) (uuid.UUID, error) {
	if len(cmd.Password) < 8 {
		return uuid.Nil, errors.New("пароль должен быть не менее 8 символов")
	}

	exists, err := u.userRepo.ExistsByEmail(ctx, cmd.Email)
	if err != nil {
		return uuid.Nil, fmt.Errorf("check email: %w", err)
	}

	if exists {
		return uuid.Nil, ErrEmailAlreadyExists
	}

	hashedPassword, err := HashPassword(cmd.Password)
	if err != nil {
		return uuid.Nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := domain.NewUser(
		cmd.Name, cmd.Lname, cmd.Pname,
		cmd.Email, cmd.Phone,
		hashedPassword,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create user: %w", err)
	}

	if u.isDev {
		user.IsValid = true
	}

	if err = u.userRepo.Create(ctx, *user); err != nil {
		return uuid.Nil, fmt.Errorf("save user: %w", err)
	}

	u.log.InfoContext(ctx, "user registered",
		"user_uuid", user.UUID,
		"email", user.Email,
	)

	return user.UUID, nil
}

func (u *authUsecase) Login(ctx context.Context, cmd LoginCommand) (*TokenPair, error) {
	if cmd.Email == "" || cmd.Password == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := u.userRepo.GetByEmail(ctx, cmd.Email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}

		return nil, fmt.Errorf("get user: %w", err)
	}

	if !u.isDev && !user.IsValid {
		return nil, ErrEmailNotConfirmed
	}

	if err = VerifyPassword(user.Password, cmd.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	accessToken, err := u.jwtService.GenerateAccessToken(
		user.UUID, user.Email, user.Name, user.IsValid,
	)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, expiresAt, err := u.jwtService.GenerateRefreshToken(
		user.UUID, user.Email, user.Name, user.IsValid,
	)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	ttl := time.Until(expiresAt)
	if err = u.redis.Set(
		ctx,
		refreshTokenPrefix+refreshToken,
		user.UUID.String(),
		ttl,
	); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	u.log.InfoContext(ctx, "user logged in",
		"user_uuid", user.UUID,
		"email", user.Email,
	)

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		UserUUID:     user.UUID,
	}, nil
}

func (u *authUsecase) Refresh(ctx context.Context, refreshToken string) (string, error) {
	claims, err := u.jwtService.Parse(refreshToken)
	if err != nil {
		return "", ErrInvalidRefreshToken
	}

	if !claims.IsRefresh() {
		return "", ErrInvalidRefreshToken
	}

	var storedUUID string

	found, err := u.redis.Get(ctx, refreshTokenPrefix+refreshToken, &storedUUID)
	if err != nil {
		return "", fmt.Errorf("check refresh token: %w", err)
	}

	if !found {
		return "", ErrInvalidRefreshToken
	}

	accessToken, err := u.jwtService.GenerateAccessToken(
		claims.UUID, claims.Email, claims.Name, claims.IsValid,
	)
	if err != nil {
		return "", fmt.Errorf("generate access token: %w", err)
	}

	u.log.InfoContext(ctx, "access token refreshed", "user_uuid", claims.UUID)

	return accessToken, nil
}

func (u *authUsecase) Logout(ctx context.Context, refreshToken string) error {
	if err := u.redis.Delete(ctx, refreshTokenPrefix+refreshToken); err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}

	u.log.InfoContext(ctx, "user logged out")

	return nil
}
