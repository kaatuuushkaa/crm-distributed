package project

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"crm-distributed/shared/domain"
	"crm-distributed/shared/pkg/kafka"
)

type ProjectUsecase interface {
	Create(ctx context.Context, cmd CreateProjectCommand) (*domain.Project, error)
	GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.Project, error)
	GetByCompany(ctx context.Context, companyUUID uuid.UUID) ([]domain.Project, error)
	Update(ctx context.Context, uid uuid.UUID, cmd UpdateProjectCommand) error
	Delete(ctx context.Context, uid uuid.UUID) error
	AddUser(ctx context.Context, projectUUID, userUUID uuid.UUID) error
}

type CreateProjectCommand struct {
	Name           string
	Description    string
	CompanyUUID    uuid.UUID
	FederationUUID uuid.UUID
	CallerEmail    string
	ResponsibleBy  string
}

type UpdateProjectCommand struct {
	Name          *string
	Description   *string
	ResponsibleBy *string
}

type projectUsecase struct {
	repo     ProjectRepository
	producer *kafka.Producer
	log      *slog.Logger
}

func NewUsecase(repo ProjectRepository, producer *kafka.Producer, log *slog.Logger) ProjectUsecase {
	return &projectUsecase{
		repo:     repo,
		producer: producer,
		log:      log,
	}
}

func (u *projectUsecase) Create(ctx context.Context, cmd CreateProjectCommand) (*domain.Project, error) {
	if cmd.CompanyUUID == uuid.Nil {
		return nil, fmt.Errorf("company UUID обязателен")
	}

	if cmd.FederationUUID == uuid.Nil {
		return nil, fmt.Errorf("federation UUID обязателен")
	}

	p, err := domain.NewProject(
		cmd.Name, cmd.Description,
		cmd.FederationUUID, cmd.CompanyUUID,
		cmd.CallerEmail, cmd.ResponsibleBy,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid project data: %w", err)
	}

	if err = u.repo.Create(ctx, *p); err != nil {
		return nil, fmt.Errorf("save project: %w", err)
	}

	u.log.InfoContext(ctx, "project created",
		"project_uuid", p.UUID,
		"company_uuid", cmd.CompanyUUID,
		"created_by", cmd.CallerEmail,
	)

	u.publishEvent(ctx, kafka.TopicProjectCreated, p.CompanyUUID.String(),
		kafka.ProjectCreatedEvent{
			ProjectUUID:    p.UUID,
			ProjectName:    p.Name,
			FederationUUID: p.FederationUUID,
			CompanyUUID:    p.CompanyUUID,
			CreatedBy:      p.CreatedBy,
			CreatedAt:      p.CreatedAt,
		},
	)

	return p, nil
}

func (u *projectUsecase) GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.Project, error) {
	p, err := u.repo.GetByUUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	return p, nil
}

func (u *projectUsecase) GetByCompany(ctx context.Context, companyUUID uuid.UUID) ([]domain.Project, error) {
	projects, err := u.repo.GetByCompany(ctx, companyUUID)
	if err != nil {
		return nil, fmt.Errorf("get company projects: %w", err)
	}

	return projects, nil
}

func (u *projectUsecase) Update(ctx context.Context, uid uuid.UUID, cmd UpdateProjectCommand) error {
	fields := make(map[string]any)

	if cmd.Name != nil {
		if err := validateName(*cmd.Name); err != nil {
			return err
		}

		fields["name"] = *cmd.Name
	}

	if cmd.Description != nil {
		fields["description"] = *cmd.Description
	}

	if cmd.ResponsibleBy != nil {
		fields["responsible_by"] = *cmd.ResponsibleBy
	}

	return u.repo.Update(ctx, uid, fields)
}

func (u *projectUsecase) Delete(ctx context.Context, uid uuid.UUID) error {
	if err := u.repo.Delete(ctx, uid); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}

	return nil
}

func (u *projectUsecase) AddUser(ctx context.Context, projectUUID, userUUID uuid.UUID) error {
	p, err := u.repo.GetByUUID(ctx, projectUUID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}

	pu, err := domain.NewProjectUser(p.FederationUUID, p.CompanyUUID, projectUUID, userUUID)
	if err != nil {
		return fmt.Errorf("create project user: %w", err)
	}

	if err = u.repo.AddUser(ctx, *pu); err != nil {
		return fmt.Errorf("add user to project: %w", err)
	}

	return nil
}

func (u *projectUsecase) publishEvent(ctx context.Context, topic, key string, event any) {
	if u.producer == nil {
		return
	}

	if err := u.producer.Send(ctx, topic, key, event); err != nil {
		u.log.ErrorContext(ctx, "failed to publish kafka event",
			"topic", topic,
			"error", err,
		)
	}
}

func validateName(name string) error {
	if len(name) < 3 || len(name) > 100 {
		return fmt.Errorf("название проекта от 3 до 100 символов")
	}

	return nil
}
