package task

import (
	"context"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

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

type taskORM struct {
	UUID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ID             int       `gorm:"autoIncrement"`
	Name           string    `gorm:"type:varchar(100);not null;default:''"`
	Description    string    `gorm:"type:text;not null;default:''"`
	FederationUUID uuid.UUID `gorm:"type:uuid;not null"`
	CompanyUUID    uuid.UUID `gorm:"type:uuid;not null;index"`
	ProjectUUID    uuid.UUID `gorm:"type:uuid;not null;index"`

	CreatedBy     string `gorm:"type:varchar(100);not null;default:''"`
	ResponsibleBy string `gorm:"type:varchar(100);not null;default:''"`
	ImplementBy   string `gorm:"type:varchar(100);not null;default:''"`
	ManagedBy     string `gorm:"type:varchar(100);not null;default:''"`
	FinishedBy    string `gorm:"type:varchar(100);not null;default:''"`

	CoWorkersBy pq.StringArray `gorm:"type:text[];not null;default:'{}'"`
	WatchBy     pq.StringArray `gorm:"type:text[];not null;default:'{}'"`
	AllPeople   pq.StringArray `gorm:"type:text[];not null;default:'{}'"`
	Tags        pq.StringArray `gorm:"type:text[];not null;default:'{}'"`

	Status   int    `gorm:"type:int;not null;default:0;index"`
	Priority int    `gorm:"type:int;not null;default:10"`
	IsEpic   bool   `gorm:"type:bool;not null;default:false"`
	Icon     string `gorm:"type:varchar(20);not null;default:''"`

	Path string `gorm:"type:ltree;default:''"`

	FinishTo   *time.Time `gorm:"type:timestamptz"`
	FinishedAt *time.Time `gorm:"type:timestamptz"`

	CreatedAt  time.Time  `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt  time.Time  `gorm:"type:timestamptz;not null;default:now()"`
	ActivityAt time.Time  `gorm:"type:timestamptz;not null;default:now()"`
	DeletedAt  *time.Time `gorm:"type:timestamptz;index"`

	Total int64 `gorm:"->"`
}

func (taskORM) TableName() string {
	return "tasks"
}

func (o *taskORM) toDomain() *domain.Task {
	return &domain.Task{
		UUID:           o.UUID,
		ID:             o.ID,
		Name:           o.Name,
		Description:    o.Description,
		FederationUUID: o.FederationUUID,
		CompanyUUID:    o.CompanyUUID,
		ProjectUUID:    o.ProjectUUID,
		CreatedBy:      o.CreatedBy,
		ResponsibleBy:  o.ResponsibleBy,
		ImplementBy:    o.ImplementBy,
		ManagedBy:      o.ManagedBy,
		FinishedBy:     o.FinishedBy,
		CoWorkersBy:    []string(o.CoWorkersBy),
		WatchBy:        []string(o.WatchBy),
		People:         []string(o.AllPeople),
		Tags:           []string(o.Tags),
		Status:         o.Status,
		Priority:       o.Priority,
		IsEpic:         o.IsEpic,
		Icon:           o.Icon,
		Path:           strings.Split(o.Path, "."),
		FinishTo:       o.FinishTo,
		FinishedAt:     o.FinishedAt,
		CreatedAt:      o.CreatedAt,
		UpdatedAt:      o.UpdatedAt,
		DeletedAt:      o.DeletedAt,
	}
}

func taskORMFromDomain(t domain.Task) *taskORM {
	return &taskORM{
		UUID:           t.UUID,
		Name:           t.Name,
		Description:    t.Description,
		FederationUUID: t.FederationUUID,
		CompanyUUID:    t.CompanyUUID,
		ProjectUUID:    t.ProjectUUID,
		CreatedBy:      t.CreatedBy,
		ResponsibleBy:  t.ResponsibleBy,
		ImplementBy:    t.ImplementBy,
		ManagedBy:      t.ManagedBy,
		CoWorkersBy:    pq.StringArray(t.CoWorkersBy),
		WatchBy:        pq.StringArray(t.WatchBy),
		AllPeople:      pq.StringArray(t.People),
		Tags:           pq.StringArray(t.Tags),
		Status:         t.Status,
		Priority:       t.Priority,
		IsEpic:         t.IsEpic,
		Icon:           t.Icon,
		Path:           strings.Join(t.Path, "."),
		FinishTo:       t.FinishTo,
		CreatedAt:      t.CreatedAt,
	}
}

type pgRepository struct {
	db  *postgres.DB
	log *slog.Logger
}

func NewRepository(db *postgres.DB, log *slog.Logger) TaskRepository {
	return &pgRepository{db: db, log: log}
}

func (r *pgRepository) Create(ctx context.Context, task domain.Task) (*domain.Task, error) {
	orm := taskORMFromDomain(task)

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var projectCounter struct {
			TaskID int
		}

		if err := tx.Raw(
			"SELECT task_id FROM projects WHERE uuid = ? FOR UPDATE",
			task.ProjectUUID,
		).Scan(&projectCounter).Error; err != nil {
			return fmt.Errorf("lock project row: %w", err)
		}

		orm.ID = projectCounter.TaskID + 1

		if err := tx.Create(orm).Error; err != nil {
			return fmt.Errorf("insert task: %w", err)
		}

		if err := tx.Exec(
			"UPDATE projects SET task_id = ? WHERE uuid = ?",
			orm.ID, task.ProjectUUID,
		).Error; err != nil {
			return fmt.Errorf("update project task counter: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create task transaction: %w", err)
	}

	return orm.toDomain(), nil
}

func (r *pgRepository) GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.Task, error) {
	var orm taskORM

	err := r.db.WithContext(ctx).
		Where("uuid = ? AND deleted_at IS NULL", uid).
		First(&orm).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrTaskNotFound
		}

		return nil, fmt.Errorf("get task by uuid %s: %w", uid, err)
	}

	return orm.toDomain(), nil
}

func (r *pgRepository) GetByProject(ctx context.Context, projectUUID uuid.UUID, f TaskFilter) ([]domain.Task, int64, error) {
	if f.PageSize <= 0 {
		f.PageSize = 20
	}

	if f.Page <= 0 {
		f.Page = 1
	}

	q := r.db.WithContext(ctx).
		Model(&taskORM{}).
		Where("project_uuid = ? AND deleted_at IS NULL", projectUUID)

	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}

	if f.Search != "" {
		q = q.Where("name ILIKE ?", "%"+f.Search+"%")
	}

	q = q.Select("*, COUNT(*) OVER() AS total")

	orderDir := "ASC"
	if f.Desc {
		orderDir = "DESC"
	}

	allowedOrderFields := map[string]bool{
		"created_at": true, "priority": true, "status": true, "updated_at": true,
	}

	orderField := "created_at"
	if allowedOrderFields[f.OrderBy] {
		orderField = f.OrderBy
	}

	q = q.Order(fmt.Sprintf("%s %s", orderField, orderDir))

	offset := (f.Page - 1) * f.PageSize
	q = q.Limit(f.PageSize).Offset(offset)

	var orms []taskORM
	if err := q.Find(&orms).Error; err != nil {
		return nil, 0, fmt.Errorf("get tasks by project %s: %w", projectUUID, err)
	}

	tasks := make([]domain.Task, 0, len(orms))
	var total int64

	for _, o := range orms {
		total = o.Total // Total одинаковый во всех строках (оконная функция)
		tasks = append(tasks, *o.toDomain())
	}

	return tasks, total, nil
}

func (r *pgRepository) Update(ctx context.Context, uid uuid.UUID, fields map[string]any) error {
	fields["updated_at"] = time.Now()
	fields["activity_at"] = time.Now()

	result := r.db.WithContext(ctx).
		Model(&taskORM{}).
		Where("uuid = ? AND deleted_at IS NULL", uid).
		Updates(fields)

	if result.Error != nil {
		return fmt.Errorf("update task %s: %w", uid, result.Error)
	}

	if result.RowsAffected == 0 {
		return domain.ErrTaskNotFound
	}

	return nil
}

func (r *pgRepository) Delete(ctx context.Context, uid uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&taskORM{}).
		Where("uuid = ? AND deleted_at IS NULL", uid).
		Update("deleted_at", time.Now())

	if result.Error != nil {
		return fmt.Errorf("delete task %s: %w", uid, result.Error)
	}

	if result.RowsAffected == 0 {
		return domain.ErrTaskNotFound
	}

	return nil
}
