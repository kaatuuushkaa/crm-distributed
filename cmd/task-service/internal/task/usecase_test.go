package task_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"crm-distributed/cmd/task-service/internal/task"
	"crm-distributed/shared/domain"
)

type mockRepository struct {
	tasks     map[uuid.UUID]*domain.Task
	createErr error
	getErr    error
}

func newMockRepository() *mockRepository {
	return &mockRepository{tasks: make(map[uuid.UUID]*domain.Task)}
}

func (m *mockRepository) Create(_ context.Context, t domain.Task) (*domain.Task, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}

	m.tasks[t.UUID] = &t

	return &t, nil
}

func (m *mockRepository) GetByUUID(_ context.Context, uid uuid.UUID) (*domain.Task, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}

	t, ok := m.tasks[uid]
	if !ok {
		return nil, domain.ErrTaskNotFound
	}

	return t, nil
}

func (m *mockRepository) GetByProject(_ context.Context, _ uuid.UUID, _ task.TaskFilter) ([]domain.Task, int64, error) {
	return nil, 0, nil
}

func (m *mockRepository) Update(_ context.Context, _ uuid.UUID, _ map[string]any) error {
	return nil
}

func (m *mockRepository) Delete(_ context.Context, uid uuid.UUID) error {
	delete(m.tasks, uid)
	return nil
}

func TestCreateTask(t *testing.T) {
	tests := []struct {
		name      string
		cmd       task.CreateTaskCommand
		repoErr   error
		wantErr   bool
		checkTask func(*testing.T, *domain.Task)
	}{
		{
			name: "успешное создание",
			cmd: task.CreateTaskCommand{
				Name:           "Сделать дизайн",
				FederationUUID: uuid.New(),
				CompanyUUID:    uuid.New(),
				ProjectUUID:    uuid.New(),
				CallerEmail:    "user@example.com",
				Priority:       5,
			},
			wantErr: false,
			checkTask: func(t *testing.T, task *domain.Task) {
				t.Helper()

				if task.UUID == uuid.Nil {
					t.Error("expected non-nil UUID")
				}

				if task.Name != "Сделать дизайн" {
					t.Errorf("expected name %q, got %q", "Сделать дизайн", task.Name)
				}

				if task.CreatedBy != "user@example.com" {
					t.Errorf("expected created_by %q, got %q", "user@example.com", task.CreatedBy)
				}
			},
		},
		{
			name: "слишком короткое название",
			cmd: task.CreateTaskCommand{
				Name:           "AB",
				FederationUUID: uuid.New(),
				CompanyUUID:    uuid.New(),
				ProjectUUID:    uuid.New(),
				CallerEmail:    "user@example.com",
			},
			wantErr: true,
		},
		{
			name: "пустой project UUID",
			cmd: task.CreateTaskCommand{
				Name:           "Валидная задача",
				FederationUUID: uuid.New(),
				CompanyUUID:    uuid.New(),
				ProjectUUID:    uuid.Nil,
				CallerEmail:    "user@example.com",
			},
			wantErr: true,
		},
		{
			name: "ошибка репозитория",
			cmd: task.CreateTaskCommand{
				Name:           "Валидная задача",
				FederationUUID: uuid.New(),
				CompanyUUID:    uuid.New(),
				ProjectUUID:    uuid.New(),
				CallerEmail:    "user@example.com",
			},
			repoErr: errors.New("db connection lost"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			repo.createErr = tt.repoErr

			uc := task.NewUsecase(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))

			got, err := uc.Create(context.Background(), tt.cmd)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.checkTask != nil {
				tt.checkTask(t, got)
			}
		})
	}
}

func TestGetByUUID_NotFound(t *testing.T) {
	repo := newMockRepository()
	uc := task.NewUsecase(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := uc.GetByUUID(context.Background(), uuid.New())

	if !errors.Is(err, domain.ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}
