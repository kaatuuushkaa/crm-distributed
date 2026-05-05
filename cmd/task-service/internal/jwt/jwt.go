package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	AccessTokenTTL  = 10 * time.Minute
	RefreshTokenTTL = 30 * 24 * time.Hour
)

var (
	ErrTokenExpired = errors.New("токен истёк")
	ErrTokenInvalid = errors.New("невалидный токен")
)

type Claims struct {
	UUID    uuid.UUID `json:"uuid"`
	Email   string    `json:"email"`
	Name    string    `json:"name"`
	IsValid bool      `json:"is_valid"`
	jwt.RegisteredClaims
}

func (c *Claims) IsRefresh() bool {
	return c.Subject == "refresh"
}

type Service struct {
	secret string
}

func New(secret string) *Service {
	return &Service{secret: secret}
}

func (s *Service) GenerateAccessToken(uid uuid.UUID, email, name string, isValid bool) (string, error) {
	return s.generate(uid, email, name, isValid, "user", AccessTokenTTL)
}

func (s *Service) GenerateRefreshToken(uid uuid.UUID, email, name string, isValid bool) (string, time.Time, error) {
	exp := time.Now().Add(RefreshTokenTTL)

	token, err := s.generate(uid, email, name, isValid, "refresh", RefreshTokenTTL)
	if err != nil {
		return "", time.Time{}, err
	}

	return token, exp, nil
}

func (s *Service) Parse(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			return []byte(s.secret), nil
		},
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, ErrTokenExpired
		}

		return nil, fmt.Errorf("%w: %s", ErrTokenInvalid, err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}

func (s *Service) generate(
	uid uuid.UUID,
	email, name string,
	isValid bool,
	subject string,
	ttl time.Duration,
) (string, error) {
	claims := Claims{
		UUID:    uid,
		Email:   email,
		Name:    name,
		IsValid: isValid,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "crm-distributed",
			Subject:   subject,
			Audience:  jwt.ClaimStrings{"crm-distributed"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString([]byte(s.secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return signed, nil
}
