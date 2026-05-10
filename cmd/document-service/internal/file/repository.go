package file

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"crm-distributed/shared/domain"
	"crm-distributed/shared/pkg/postgres"
)

type Repository struct {
	db *postgres.DB
}

func NewRepository(db *postgres.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, f *domain.File) error {
	const query = `
		INSERT INTO files (
			uuid, owner_uuid, owner_type,
			original_name, storage_key, content_type, size_bytes,
			created_by, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.ExecContext(ctx, "file.create", query,
		f.UUID, f.OwnerUUID, f.OwnerType,
		f.OriginalName, f.StorageKey, f.ContentType, f.SizeBytes,
		f.CreatedBy, f.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert file metadata: %w", err)
	}

	return nil
}

func (r *Repository) Get(ctx context.Context, uid uuid.UUID) (*domain.File, error) {
	const query = `
		SELECT uuid, owner_uuid, owner_type,
		       original_name, storage_key, content_type, size_bytes,
		       created_by, created_at
		FROM files
		WHERE uuid = $1 AND deleted_at IS NULL`

	var f domain.File

	err := r.db.QueryRowContext(ctx, "file.get", query, uid).Scan(
		&f.UUID, &f.OwnerUUID, &f.OwnerType,
		&f.OriginalName, &f.StorageKey, &f.ContentType, &f.SizeBytes,
		&f.CreatedBy, &f.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrFileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query file: %w", err)
	}

	return &f, nil
}

func (r *Repository) ListByOwner(ctx context.Context, ownerUUID uuid.UUID, ownerType domain.FileOwnerType) ([]domain.File, error) {
	const query = `
		SELECT uuid, owner_uuid, owner_type,
		       original_name, storage_key, content_type, size_bytes,
		       created_by, created_at
		FROM files
		WHERE owner_uuid = $1 AND owner_type = $2 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, "file.list_by_owner", query, ownerUUID, ownerType)
	if err != nil {
		return nil, fmt.Errorf("query files: %w", err)
	}
	defer rows.Close()

	var files []domain.File

	for rows.Next() {
		var f domain.File

		if err := rows.Scan(
			&f.UUID, &f.OwnerUUID, &f.OwnerType,
			&f.OriginalName, &f.StorageKey, &f.ContentType, &f.SizeBytes,
			&f.CreatedBy, &f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan file: %w", err)
		}

		files = append(files, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return files, nil
}

func (r *Repository) SoftDelete(ctx context.Context, uid uuid.UUID) error {
	const query = `
		UPDATE files
		SET deleted_at = $1
		WHERE uuid = $2 AND deleted_at IS NULL`

	res, err := r.db.ExecContext(ctx, "file.delete", query, time.Now(), uid)
	if err != nil {
		return fmt.Errorf("soft delete file: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if rows == 0 {
		return domain.ErrFileNotFound
	}

	return nil
}
