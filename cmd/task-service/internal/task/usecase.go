package task

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"crm-distributed/shared/domain"
)

type TaskUsecase interface {
	Create(ctx context.Context, cmd CreateTaskCommand) (*domain.Task, error)
	GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.Task, error)
	List(ctx context.Context, projectUUID uuid.UUID, filter TaskFilter) ([]domain.Task, int64, error)
	UpdateStatus(ctx context.Context, uid uuid.UUID, cmd UpdateStatusCommand) error
	Delete(ctx context.Context, uid uuid.UUID, callerEmail string) error
}

type CreateTaskCommand struct {
	Name           string
	Description    string
	FederationUUID uuid.UUID
	CompanyUUID    uuid.UUID
	ProjectUUID    uuid.UUID
	CallerEmail    string
	ImplementBy    string
	ResponsibleBy  string
	ManagedBy      string
	CoWorkersBy    []string
	Tags           []string
	Priority       int
	Icon           string
}

type UpdateStatusCommand struct {
	NewStatus   int
	Comment     string
	CallerEmail string
}

type taskUsecase struct {
	repo TaskRepository
	log  *slog.Logger

	onTaskCreated func(ctx context.Context, task *domain.Task) error
}

func NewUsecase(repo TaskRepository, log *slog.Logger) TaskUsecase {
	return &taskUsecase{
		repo: repo,
		log:  log,
	}
}

func (u *taskUsecase) WithTaskCreatedCallback(fn func(ctx context.Context, task *domain.Task) error) {
	u.onTaskCreated = fn
}

func (u *taskUsecase) Create(ctx context.Context, cmd CreateTaskCommand) (*domain.Task, error) {
	if cmd.ProjectUUID == uuid.Nil {
		return nil, fmt.Errorf("project UUID обязателен")
	}

	if cmd.FederationUUID == uuid.Nil {
		return nil, fmt.Errorf("federation UUID обязателен")
	}

	task, err := domain.NewTask(
		cmd.Name,
		cmd.FederationUUID,
		cmd.CompanyUUID,
		cmd.ProjectUUID,
		cmd.CallerEmail,
		domain.TaskCreateOptions{
			Description:   cmd.Description,
			Tags:          cmd.Tags,
			CoWorkersBy:   cmd.CoWorkersBy,
			ImplementBy:   cmd.ImplementBy,
			ResponsibleBy: cmd.ResponsibleBy,
			ManagedBy:     cmd.ManagedBy,
			Priority:      cmd.Priority,
			Icon:          cmd.Icon,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("invalid task data: %w", err)
	}

	created, err := u.repo.Create(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("save task: %w", err)
	}

	u.log.InfoContext(ctx, "task created",
		"task_uuid", created.UUID,
		"project_uuid", created.ProjectUUID,
		"created_by", created.CreatedBy,
	)

	if u.onTaskCreated != nil {
		if err = u.onTaskCreated(ctx, created); err != nil {
			u.log.ErrorContext(ctx, "failed to publish task.created event",
				"task_uuid", created.UUID,
				"error", err,
			)
		}
	}

	return created, nil
}

func (u *taskUsecase) GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.Task, error) {
	task, err := u.repo.GetByUUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	return task, nil
}

func (u *taskUsecase) List(ctx context.Context, projectUUID uuid.UUID, filter TaskFilter) ([]domain.Task, int64, error) {
	tasks, total, err := u.repo.GetByProject(ctx, projectUUID, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}

	return tasks, total, nil
}

func (u *taskUsecase) UpdateStatus(ctx context.Context, uid uuid.UUID, cmd UpdateStatusCommand) error {
	task, err := u.repo.GetByUUID(ctx, uid)
	if err != nil {
		return fmt.Errorf("get task for status update: %w", err)
	}

	_, err = task.PatchStatus(cmd.NewStatus, domain.ProjectOptions{}, cmd.Comment, nil)
	if err != nil {
		return fmt.Errorf("patch status: %w", err)
	}

	if err = u.repo.Update(ctx, uid, map[string]any{
		"status": task.Status,
	}); err != nil {
		return fmt.Errorf("save status update: %w", err)
	}

	u.log.InfoContext(ctx, "task status updated",
		"task_uuid", uid,
		"new_status", cmd.NewStatus,
		"caller", cmd.CallerEmail,
	)

	return nil
}

func (u *taskUsecase) Delete(ctx context.Context, uid uuid.UUID, callerEmail string) error {
	if err := u.repo.Delete(ctx, uid); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	u.log.InfoContext(ctx, "task deleted",
		"task_uuid", uid,
		"caller", callerEmail,
	)

	return nil
}
