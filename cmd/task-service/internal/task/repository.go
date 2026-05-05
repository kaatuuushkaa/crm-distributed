package task

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"crm-distributed/shared/domain"
	"crm-distributed/shared/pkg/postgres"
)

type TaskRepository interface {
	Create(ctx context.Context, task domain.Task) (*domain.Task, error)
	GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.Task, error)
	GetByProject(ctx context.Context, projectUUID uuid.UUID, filter TaskFilter) ([]domain.Task, int64, error)
	Update(ctx context.Context, uid uuid.UUID, fields map[string]any) error
	Delete(ctx context.Context, uid uuid.UUID) error
}

type TaskFilter struct {
	Status   *int
	Search   string
	Page     int
	PageSize int
	OrderBy  string
	Desc     bool
}

type pgRepository struct {
	db  *postgres.DB
	log *slog.Logger
}

func NewRepository(db *postgres.DB, log *slog.Logger) TaskRepository {
	return &pgRepository{db: db, log: log}
}

func (r *pgRepository) Create(ctx context.Context, task domain.Task) (*domain.Task, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			r.log.ErrorContext(ctx, "rollback failed", "error", rbErr)
		}
	}()

	var currentTaskID int

	err = tx.QueryRowContext(ctx,
		`SELECT task_id FROM projects WHERE uuid = $1 FOR UPDATE`,
		task.ProjectUUID,
	).Scan(&currentTaskID)
	if err != nil {
		return nil, fmt.Errorf("lock project row: %w", err)
	}

	task.ID = currentTaskID + 1

	_, err = tx.ExecContext(ctx,
		`INSERT INTO tasks (
			uuid, id, name, description,
			federation_uuid, company_uuid, project_uuid,
			created_by, responsible_by, implement_by, managed_by,
			co_workers_by, watch_by, all_people, tags,
			status, priority, is_epic, icon,
			path, finish_to, created_at, updated_at, activity_at
		) VALUES (
			$1,  $2,  $3,  $4,
			$5,  $6,  $7,
			$8,  $9,  $10, $11,
			$12, $13, $14, $15,
			$16, $17, $18, $19,
			$20::ltree, $21, $22, $22, $22
		)`,
		task.UUID, task.ID, task.Name, task.Description,
		task.FederationUUID, task.CompanyUUID, task.ProjectUUID,
		task.CreatedBy, task.ResponsibleBy, task.ImplementBy, task.ManagedBy,
		pq.Array(task.CoWorkersBy), pq.Array(task.WatchBy),
		pq.Array(task.People), pq.Array(task.Tags),
		task.Status, task.Priority, task.IsEpic, task.Icon,
		strings.Join(task.Path, "."), task.FinishTo, time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE projects SET task_id = $1 WHERE uuid = $2`,
		task.ID, task.ProjectUUID,
	)
	if err != nil {
		return nil, fmt.Errorf("update project task counter: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &task, nil
}

func (r *pgRepository) GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.Task, error) {
	row := r.db.QueryRowContext(ctx, "get_task_by_uuid",
		`SELECT
			uuid, id, name, description,
			federation_uuid, company_uuid, project_uuid,
			created_by, responsible_by, implement_by, managed_by, finished_by,
			co_workers_by, watch_by, all_people, tags,
			status, priority, is_epic, icon,
			path::text,
			finish_to, finished_at,
			created_at, updated_at, activity_at, deleted_at
		FROM tasks
		WHERE uuid = $1
		  AND deleted_at IS NULL`,
		uid,
	)

	return scanTask(row)
}

func (r *pgRepository) GetByProject(
	ctx context.Context,
	projectUUID uuid.UUID,
	f TaskFilter,
) ([]domain.Task, int64, error) {
	if f.PageSize <= 0 {
		f.PageSize = 20
	}

	if f.Page <= 0 {
		f.Page = 1
	}

	offset := (f.Page - 1) * f.PageSize

	args := []any{projectUUID}
	argIdx := 2

	whereClause := "WHERE project_uuid = $1 AND deleted_at IS NULL"

	if f.Status != nil {
		whereClause += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *f.Status)
		argIdx++
	}

	if f.Search != "" {
		whereClause += fmt.Sprintf(" AND name ILIKE $%d", argIdx)
		args = append(args, "%"+f.Search+"%")
		argIdx++
	}

	allowedOrderFields := map[string]string{
		"created_at":  "created_at",
		"priority":    "priority",
		"status":      "status",
		"updated_at":  "updated_at",
		"activity_at": "activity_at",
	}

	orderField, ok := allowedOrderFields[f.OrderBy]
	if !ok {
		orderField = "created_at"
	}

	orderDir := "ASC"
	if f.Desc {
		orderDir = "DESC"
	}

	args = append(args, f.PageSize, offset)
	limitClause := fmt.Sprintf("LIMIT $%d OFFSET $%d", argIdx, argIdx+1)

	query := fmt.Sprintf(`
		SELECT
			uuid, id, name, description,
			federation_uuid, company_uuid, project_uuid,
			created_by, responsible_by, implement_by, managed_by, finished_by,
			co_workers_by, watch_by, all_people, tags,
			status, priority, is_epic, icon,
			path::text,
			finish_to, finished_at,
			created_at, updated_at, activity_at, deleted_at,
			COUNT(*) OVER() AS total
		FROM tasks
		%s
		ORDER BY %s %s
		%s`,
		whereClause, orderField, orderDir, limitClause,
	)

	rows, err := r.db.QueryContext(ctx, "get_tasks_by_project", query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query tasks: %w", err)
	}

	defer rows.Close() //nolint:errcheck // Close error здесь не несёт смысла

	var tasks []domain.Task
	var total int64

	for rows.Next() {
		var t domain.Task
		var pathStr string
		var finishedBy sql.NullString
		var finishTo, finishedAt, deletedAt sql.NullTime
		var rowTotal int64

		err = rows.Scan(
			&t.UUID, &t.ID, &t.Name, &t.Description,
			&t.FederationUUID, &t.CompanyUUID, &t.ProjectUUID,
			&t.CreatedBy, &t.ResponsibleBy, &t.ImplementBy, &t.ManagedBy, &finishedBy,
			pq.Array(&t.CoWorkersBy), pq.Array(&t.WatchBy),
			pq.Array(&t.People), pq.Array(&t.Tags),
			&t.Status, &t.Priority, &t.IsEpic, &t.Icon,
			&pathStr,
			&finishTo, &finishedAt,
			&t.CreatedAt, &t.UpdatedAt, &t.ActivityAt, &deletedAt,
			&rowTotal,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan task row: %w", err)
		}

		if finishedBy.Valid {
			t.FinishedBy = finishedBy.String
		}

		if finishTo.Valid {
			t.FinishTo = &finishTo.Time
		}

		if finishedAt.Valid {
			t.FinishedAt = &finishedAt.Time
		}

		if deletedAt.Valid {
			t.DeletedAt = &deletedAt.Time
		}

		if pathStr != "" {
			t.Path = strings.Split(pathStr, ".")
		}

		total = rowTotal
		tasks = append(tasks, t)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate task rows: %w", err)
	}

	return tasks, total, nil
}

func (r *pgRepository) Update(ctx context.Context, uid uuid.UUID, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}

	allowedFields := map[string]bool{
		"name": true, "description": true, "status": true,
		"priority": true, "icon": true, "finish_to": true,
		"responsible_by": true, "implement_by": true,
		"managed_by": true, "co_workers_by": true,
		"watch_by": true, "tags": true, "is_epic": true,
	}

	setClauses := make([]string, 0, len(fields)+2)
	args := make([]any, 0, len(fields)+2)
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

	now := time.Now()
	setClauses = append(setClauses,
		fmt.Sprintf("updated_at = $%d", argIdx),
		fmt.Sprintf("activity_at = $%d", argIdx+1),
	)
	args = append(args, now, now)
	argIdx += 2

	args = append(args, uid)

	query := fmt.Sprintf(
		`UPDATE tasks SET %s WHERE uuid = $%d AND deleted_at IS NULL`,
		strings.Join(setClauses, ", "), argIdx,
	)

	result, err := r.db.ExecContext(ctx, "update_task", query, args...)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if affected == 0 {
		return domain.ErrTaskNotFound
	}

	return nil
}

func (r *pgRepository) Delete(ctx context.Context, uid uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, "delete_task",
		`UPDATE tasks
		 SET deleted_at = NOW(),
		     updated_at = NOW()
		 WHERE uuid = $1
		   AND deleted_at IS NULL`,
		uid,
	)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if affected == 0 {
		return domain.ErrTaskNotFound
	}

	return nil
}

func scanTask(row *sql.Row) (*domain.Task, error) {
	var t domain.Task
	var pathStr string
	var finishedBy sql.NullString
	var finishTo, finishedAt, deletedAt sql.NullTime

	err := row.Scan(
		&t.UUID, &t.ID, &t.Name, &t.Description,
		&t.FederationUUID, &t.CompanyUUID, &t.ProjectUUID,
		&t.CreatedBy, &t.ResponsibleBy, &t.ImplementBy, &t.ManagedBy, &finishedBy,
		pq.Array(&t.CoWorkersBy), pq.Array(&t.WatchBy),
		pq.Array(&t.People), pq.Array(&t.Tags),
		&t.Status, &t.Priority, &t.IsEpic, &t.Icon,
		&pathStr,
		&finishTo, &finishedAt,
		&t.CreatedAt, &t.UpdatedAt, &t.ActivityAt, &deletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTaskNotFound
		}

		return nil, fmt.Errorf("scan task: %w", err)
	}

	if finishedBy.Valid {
		t.FinishedBy = finishedBy.String
	}

	if finishTo.Valid {
		t.FinishTo = &finishTo.Time
	}

	if finishedAt.Valid {
		t.FinishedAt = &finishedAt.Time
	}

	if deletedAt.Valid {
		t.DeletedAt = &deletedAt.Time
	}

	if pathStr != "" {
		t.Path = strings.Split(pathStr, ".")
	}

	return &t, nil
}
