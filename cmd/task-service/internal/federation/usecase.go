package federation

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"crm-distributed/shared/domain"
	"crm-distributed/shared/pkg/kafka"
)

type FederationUsecase interface {
	Create(ctx context.Context, cmd CreateFederationCommand) (*domain.Federation, error)
	CreateCompany(ctx context.Context, cmd CreateCompanyCommand) (uuid.UUID, error)
	GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.Federation, error)
	GetByUserUUID(ctx context.Context, userUUID uuid.UUID) ([]domain.Federation, error)
	AddUser(ctx context.Context, federationUUID, userUUID uuid.UUID) error
}

type CreateFederationCommand struct {
	Name        string
	CallerEmail string
	CallerUUID  uuid.UUID
}

type CreateCompanyCommand struct {
	FederationUUID uuid.UUID
	Name           string
	CallerEmail    string
	CallerUUID     uuid.UUID
}

type federationUsecase struct {
	repo     FederationRepository
	producer *kafka.Producer
	log      *slog.Logger
}

func NewUsecase(repo FederationRepository, producer *kafka.Producer, log *slog.Logger) FederationUsecase {
	return &federationUsecase{
		repo:     repo,
		producer: producer,
		log:      log,
	}
}

func (u *federationUsecase) Create(ctx context.Context, cmd CreateFederationCommand) (*domain.Federation, error) {
	if cmd.CallerUUID == uuid.Nil {
		return nil, fmt.Errorf("caller UUID обязателен")
	}

	federation, err := domain.NewFederation(cmd.Name, cmd.CallerEmail, cmd.CallerUUID)
	if err != nil {
		return nil, fmt.Errorf("invalid federation data: %w", err)
	}

	if err = u.repo.Create(ctx, *federation); err != nil {
		return nil, fmt.Errorf("save federation: %w", err)
	}

	u.log.InfoContext(ctx, "federation created",
		"federation_uuid", federation.UUID,
		"created_by", cmd.CallerEmail,
	)

	return federation, nil
}
func (uc *federationUsecase) CreateCompany(ctx context.Context, cmd CreateCompanyCommand) (uuid.UUID, error) {
	companyUUID := uuid.New()

	if err := uc.repo.CreateCompany(ctx,
		companyUUID,
		cmd.FederationUUID,
		cmd.Name,
		cmd.CallerEmail,
		cmd.CallerUUID,
	); err != nil {
		return uuid.Nil, fmt.Errorf("create company: %w", err)
	}

	uc.log.InfoContext(ctx, "company created",
		"company_uuid", companyUUID,
		"federation_uuid", cmd.FederationUUID,
		"created_by", cmd.CallerEmail,
	)

	return companyUUID, nil
}

func (u *federationUsecase) GetByUUID(ctx context.Context, uid uuid.UUID) (*domain.Federation, error) {
	f, err := u.repo.GetByUUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get federation: %w", err)
	}

	return f, nil
}

func (u *federationUsecase) GetByUserUUID(ctx context.Context, userUUID uuid.UUID) ([]domain.Federation, error) {
	federations, err := u.repo.GetByUserUUID(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("get user federations: %w", err)
	}

	return federations, nil
}

func (u *federationUsecase) AddUser(ctx context.Context, federationUUID, userUUID uuid.UUID) error {
	fu, err := domain.NewFederationUser(federationUUID, userUUID)
	if err != nil {
		return fmt.Errorf("create federation user: %w", err)
	}

	if err = u.repo.AddUser(ctx, *fu); err != nil {
		return fmt.Errorf("add user to federation: %w", err)
	}

	u.log.InfoContext(ctx, "user added to federation",
		"federation_uuid", federationUUID,
		"user_uuid", userUUID,
	)

	return nil
}
