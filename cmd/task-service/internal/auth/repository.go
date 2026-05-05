package auth

import (
	"context"
	"crm-distributed/shared/pkg/postgres"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"crm-distributed/shared/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.User, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

type pgUserRepository struct {
	db  *postgres.DB
	log *slog.Logger
}

func NewUserRepository(db *postgres.DB, log *slog.Logger) UserRepository {
	return &pgUserRepository{db: db, log: log}
}

func (r *pgUserRepository) Create(ctx context.Context, user domain.User) error {
	_, err := r.db.ExecContext(ctx, "create_user",
		`INSERT INTO users (
			uuid, name, lname, pname,
			email, phone, password,
			is_valid, color, provider,
			created_at, updated_at
		) VALUES (
			$1,  $2,  $3,  $4,
			$5,  $6,  $7,
			$8,  $9,  $10,
			$11, $11
		)`,
		user.UUID, user.Name, user.Lname, user.Pname,
		user.Email, user.Phone, user.Password,
		user.IsValid, user.Color, user.Provider,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

func (r *pgUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User

	err := r.db.QueryRowContext(ctx, "get_user_by_email",
		`SELECT
			uuid, name, lname, pname,
			email, phone, password,
			is_valid, color, has_photo, provider,
			created_at, updated_at
		FROM users
		WHERE email = $1
		  AND deleted_at IS NULL`,
		email,
	).Scan(
		&u.UUID, &u.Name, &u.Lname, &u.Pname,
		&u.Email, &u.Phone, &u.Password,
		&u.IsValid, &u.Color, &u.HasPhoto, &u.Provider,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}

		return nil, fmt.Errorf("get user by email: %w", err)
	}

	return &u, nil
}

func (r *pgUserRepository) GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.User, error) {
	var u domain.User

	err := r.db.QueryRowContext(ctx, "get_user_by_uuid",
		`SELECT
			uuid, name, lname, pname,
			email, phone, password,
			is_valid, color, has_photo, provider,
			created_at, updated_at
		FROM users
		WHERE uuid = $1
		  AND deleted_at IS NULL`,
		uid,
	).Scan(
		&u.UUID, &u.Name, &u.Lname, &u.Pname,
		&u.Email, &u.Phone, &u.Password,
		&u.IsValid, &u.Color, &u.HasPhoto, &u.Provider,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}

		return nil, fmt.Errorf("get user by uuid: %w", err)
	}

	return &u, nil
}

func (r *pgUserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool

	err := r.db.QueryRowContext(ctx, "check_email_exists",
		`SELECT EXISTS(
			SELECT 1 FROM users
			WHERE email = $1
			  AND deleted_at IS NULL
		)`,
		email,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check email exists: %w", err)
	}

	return exists, nil
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hash), nil
}

func VerifyPassword(hashedPassword, plainPassword string) error {
	if err := bcrypt.CompareHashAndPassword(
		[]byte(hashedPassword),
		[]byte(plainPassword),
	); err != nil {
		return errors.New("неверный пароль")
	}

	return nil
}
