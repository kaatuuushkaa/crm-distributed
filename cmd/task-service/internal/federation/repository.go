package federation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"crm-distributed/shared/domain"
	"crm-distributed/shared/pkg/postgres"
)

type FederationRepository interface {
	Create(ctx context.Context, f domain.Federation) error
	CreateCompany(ctx context.Context, companyUUID, federationUUID uuid.UUID, name, callerEmail string, callerUUID uuid.UUID) error
	GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.Federation, error)
	GetByUserUUID(ctx context.Context, userUUID uuid.UUID) ([]domain.Federation, error)
	AddUser(ctx context.Context, fu domain.FederationUser) error
	Delete(ctx context.Context, uid uuid.UUID) error
}

type pgFederationRepository struct {
	db  *postgres.DB
	log *slog.Logger
}

func NewRepository(db *postgres.DB, log *slog.Logger) FederationRepository {
	return &pgFederationRepository{db: db, log: log}
}

func (r *pgFederationRepository) Create(ctx context.Context, f domain.Federation) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			r.log.ErrorContext(ctx, "rollback failed", "error", rbErr)
		}
	}()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO federations (uuid, name, created_by, created_by_uuid, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $5)`,
		f.UUID, f.Name, f.CreatedBy, f.CreatedByUUID, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("insert federation: %w", err)
	}

	for _, u := range f.Users {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO federation_users (uuid, federation_uuid, user_uuid, added_at)
			 VALUES ($1, $2, $3, $4)`,
			u.UUID, f.UUID, u.User.UUID, time.Now(),
		)
		if err != nil {
			return fmt.Errorf("insert federation user: %w", err)
		}
	}

	return tx.Commit()
}

func (r *pgFederationRepository) GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.Federation, error) {
	var f domain.Federation

	err := r.db.QueryRowContext(ctx, "get_federation_by_uuid",
		`SELECT uuid, name, created_by, created_by_uuid, created_at, updated_at
		 FROM federations
		 WHERE uuid = $1 AND deleted_at IS NULL`,
		uid,
	).Scan(
		&f.UUID, &f.Name, &f.CreatedBy, &f.CreatedByUUID,
		&f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrFederationNotFound
		}

		return nil, fmt.Errorf("get federation: %w", err)
	}

	users, err := r.getFederationUsers(ctx, uid)
	if err != nil {
		return nil, err
	}

	f.Users = users
	f.UsersTotal = len(users)

	return &f, nil
}

func (r *pgFederationRepository) GetByUserUUID(ctx context.Context, userUUID uuid.UUID) ([]domain.Federation, error) {
	rows, err := r.db.QueryContext(ctx, "get_federations_by_user",
		`SELECT f.uuid, f.name, f.created_by, f.created_by_uuid, f.created_at, f.updated_at
		 FROM federations f
		 INNER JOIN federation_users fu ON fu.federation_uuid = f.uuid
		 WHERE fu.user_uuid = $1
		   AND f.deleted_at IS NULL
		 ORDER BY f.created_at DESC`,
		userUUID,
	)
	if err != nil {
		return nil, fmt.Errorf("get federations by user: %w", err)
	}

	defer rows.Close()

	var federations []domain.Federation

	for rows.Next() {
		var f domain.Federation

		if err = rows.Scan(
			&f.UUID, &f.Name, &f.CreatedBy, &f.CreatedByUUID,
			&f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan federation: %w", err)
		}

		federations = append(federations, f)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate federations: %w", err)
	}

	return federations, nil
}

func (r *pgFederationRepository) AddUser(ctx context.Context, fu domain.FederationUser) error {
	_, err := r.db.ExecContext(ctx, "add_federation_user",
		`INSERT INTO federation_users (uuid, federation_uuid, user_uuid, added_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (federation_uuid, user_uuid) DO NOTHING`,
		fu.UUID, fu.FederationUUID, fu.User.UUID, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("add federation user: %w", err)
	}

	return nil
}

func (r *pgFederationRepository) Delete(ctx context.Context, uid uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, "delete_federation",
		`UPDATE federations SET deleted_at = NOW() WHERE uuid = $1 AND deleted_at IS NULL`,
		uid,
	)
	if err != nil {
		return fmt.Errorf("delete federation: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if affected == 0 {
		return domain.ErrFederationNotFound
	}

	return nil
}

func (r *pgFederationRepository) getFederationUsers(ctx context.Context, federationUUID uuid.UUID) ([]domain.FederationUser, error) {
	rows, err := r.db.QueryContext(ctx, "get_federation_users",
		`SELECT fu.uuid, fu.user_uuid, u.name, u.lname, u.email, u.color, fu.added_at
		 FROM federation_users fu
		 INNER JOIN users u ON u.uuid = fu.user_uuid
		 WHERE fu.federation_uuid = $1
		 ORDER BY fu.added_at ASC`,
		federationUUID,
	)
	if err != nil {
		return nil, fmt.Errorf("get federation users: %w", err)
	}

	defer rows.Close()

	var users []domain.FederationUser

	for rows.Next() {
		var fu domain.FederationUser

		if err = rows.Scan(
			&fu.UUID, &fu.User.UUID,
			&fu.User.Name, &fu.User.Lname, &fu.User.Email, &fu.User.Color,
			&fu.AddedAt,
		); err != nil {
			return nil, fmt.Errorf("scan federation user: %w", err)
		}

		fu.FederationUUID = federationUUID
		users = append(users, fu)
	}

	return users, rows.Err()
}
func (r *pgFederationRepository) CreateCompany(ctx context.Context, companyUUID, federationUUID uuid.UUID, name, callerEmail string, callerUUID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, "company.create",
		`INSERT INTO companies (uuid, federation_uuid, name, created_by, created_by_uuid, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, now(), now())`,
		companyUUID, federationUUID, name, callerEmail, callerUUID,
	)
	if err != nil {
		return fmt.Errorf("create company: %w", err)
	}

	return nil
}
