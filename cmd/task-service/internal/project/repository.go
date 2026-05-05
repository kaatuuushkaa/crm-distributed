package project

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"crm-distributed/shared/domain"
	"crm-distributed/shared/pkg/postgres"
)

type ProjectRepository interface {
	Create(ctx context.Context, p domain.Project) error
	GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.Project, error)
	GetByCompany(ctx context.Context, companyUUID uuid.UUID) ([]domain.Project, error)
	Update(ctx context.Context, uid uuid.UUID, fields map[string]any) error
	Delete(ctx context.Context, uid uuid.UUID) error
	AddUser(ctx context.Context, pu domain.ProjectUser) error
}

type pgProjectRepository struct {
	db  *postgres.DB
	log *slog.Logger
}

func NewRepository(db *postgres.DB, log *slog.Logger) ProjectRepository {
	return &pgProjectRepository{db: db, log: log}
}

func (r *pgProjectRepository) Create(ctx context.Context, p domain.Project) error {
	optionsJSON, err := json.Marshal(p.Options)
	if err != nil {
		return fmt.Errorf("marshal project options: %w", err)
	}

	_, err = r.db.ExecContext(ctx, "create_project",
		`INSERT INTO projects (
			uuid, name, description,
			federation_uuid, company_uuid,
			created_by, responsible_by,
			options, task_id,
			created_at, updated_at
		) VALUES (
			$1, $2, $3,
			$4, $5,
			$6, $7,
			$8, 0,
			$9, $9
		)`,
		p.UUID, p.Name, p.Description,
		p.FederationUUID, p.CompanyUUID,
		p.CreatedBy, p.ResponsibleBy,
		optionsJSON, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}

	return nil
}

func (r *pgProjectRepository) GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.Project, error) {
	var p domain.Project
	var optionsJSON []byte
	var deletedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, "get_project_by_uuid",
		`SELECT
			uuid, name, description,
			federation_uuid, company_uuid,
			created_by, responsible_by,
			options,
			created_at, updated_at, deleted_at
		FROM projects
		WHERE uuid = $1 AND deleted_at IS NULL`,
		uid,
	).Scan(
		&p.UUID, &p.Name, &p.Description,
		&p.FederationUUID, &p.CompanyUUID,
		&p.CreatedBy, &p.ResponsibleBy,
		&optionsJSON,
		&p.CreatedAt, &p.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrProjectNotFound
		}

		return nil, fmt.Errorf("get project: %w", err)
	}

	if deletedAt.Valid {
		p.DeletedAt = &deletedAt.Time
	}

	if err = p.Options.Scan(optionsJSON); err != nil {
		return nil, fmt.Errorf("scan project options: %w", err)
	}

	return &p, nil
}

func (r *pgProjectRepository) GetByCompany(ctx context.Context, companyUUID uuid.UUID) ([]domain.Project, error) {
	rows, err := r.db.QueryContext(ctx, "get_projects_by_company",
		`SELECT
			uuid, name, description,
			federation_uuid, company_uuid,
			created_by, responsible_by,
			options,
			created_at, updated_at
		FROM projects
		WHERE company_uuid = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`,
		companyUUID,
	)
	if err != nil {
		return nil, fmt.Errorf("get projects by company: %w", err)
	}

	defer rows.Close()

	var projects []domain.Project

	for rows.Next() {
		var p domain.Project
		var optionsJSON []byte

		if err = rows.Scan(
			&p.UUID, &p.Name, &p.Description,
			&p.FederationUUID, &p.CompanyUUID,
			&p.CreatedBy, &p.ResponsibleBy,
			&optionsJSON,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}

		if err = p.Options.Scan(optionsJSON); err != nil {
			return nil, fmt.Errorf("scan project options: %w", err)
		}

		projects = append(projects, p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}

	return projects, nil
}

func (r *pgProjectRepository) Update(ctx context.Context, uid uuid.UUID, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}

	allowedFields := map[string]bool{
		"name": true, "description": true,
		"responsible_by": true, "options": true,
	}

	setClauses := make([]string, 0, len(fields)+1)
	args := make([]any, 0, len(fields)+1)
	argIdx := 1

	for col, val := range fields {
		if !allowedFields[col] {
			continue
		}

		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, argIdx))
		args = append(args, val)
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIdx))
	args = append(args, time.Now())
	argIdx++

	args = append(args, uid)

	query := fmt.Sprintf(
		`UPDATE projects SET %s WHERE uuid = $%d AND deleted_at IS NULL`,
		joinClauses(setClauses), argIdx,
	)

	result, err := r.db.ExecContext(ctx, "update_project", query, args...)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if affected == 0 {
		return domain.ErrProjectNotFound
	}

	return nil
}

func (r *pgProjectRepository) Delete(ctx context.Context, uid uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, "delete_project",
		`UPDATE projects
		 SET deleted_at = NOW(), updated_at = NOW()
		 WHERE uuid = $1 AND deleted_at IS NULL`,
		uid,
	)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if affected == 0 {
		return domain.ErrProjectNotFound
	}

	return nil
}

func (r *pgProjectRepository) AddUser(ctx context.Context, pu domain.ProjectUser) error {
	_, err := r.db.ExecContext(ctx, "add_project_user",
		`INSERT INTO project_users (uuid, federation_uuid, company_uuid, project_uuid, user_uuid, added_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (project_uuid, user_uuid) DO NOTHING`,
		pu.UUID, pu.FederationUUID, pu.CompanyUUID, pu.ProjectUUID, pu.User.UUID, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("add project user: %w", err)
	}

	return nil
}

func joinClauses(clauses []string) string {
	result := ""
	for i, c := range clauses {
		if i > 0 {
			result += ", "
		}

		result += c
	}

	return result
}
