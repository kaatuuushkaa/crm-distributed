package legalentity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"crm-distributed/shared/domain"
)

type CreateEntityCommand struct {
	CompanyUUID   uuid.UUID
	Name          string
	INN           string
	KPP           string
	LegalAddress  string
	ActualAddress string
}

type CreateAccountCommand struct {
	LegalEntityUUID uuid.UUID
	Bank            string
	BIK             string
	CorrAcc         string
	PayAcc          string
	Address         string
	Currency        string
	Comment         string
	IsPrimary       bool
}

type Usecase struct {
	repo *Repository
	log  *slog.Logger
}

func NewUsecase(repo *Repository, log *slog.Logger) *Usecase {
	return &Usecase{repo: repo, log: log}
}

func (u *Usecase) CreateEntity(ctx context.Context, cmd CreateEntityCommand) (*domain.LegalEntity, error) {
	if err := validateINN(cmd.INN); err != nil {
		return nil, fmt.Errorf("validate inn: %w", err)
	}

	if cmd.Name == "" {
		return nil, errors.New("name is required")
	}

	if cmd.CompanyUUID == uuid.Nil {
		return nil, errors.New("company uuid is required")
	}

	exists, err := u.repo.ExistsByINN(ctx, cmd.INN)
	if err != nil {
		return nil, fmt.Errorf("check inn: %w", err)
	}

	if exists {
		return nil, domain.ErrLegalEntityAlreadyExists
	}

	le := &domain.LegalEntity{
		UUID:          uuid.New(),
		CompanyUUID:   cmd.CompanyUUID,
		Name:          cmd.Name,
		INN:           cmd.INN,
		KPP:           cmd.KPP,
		LegalAddress:  cmd.LegalAddress,
		ActualAddress: cmd.ActualAddress,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := u.repo.CreateEntity(ctx, le); err != nil {
		return nil, fmt.Errorf("create entity: %w", err)
	}

	u.log.InfoContext(ctx, "legal entity created",
		"uuid", le.UUID,
		"company_uuid", le.CompanyUUID,
		"inn", le.INN,
	)

	return le, nil
}

func (u *Usecase) GetEntity(ctx context.Context, uid uuid.UUID) (*domain.LegalEntity, error) {
	return u.repo.GetEntity(ctx, uid)
}

func (u *Usecase) ListByCompany(ctx context.Context, companyUUID uuid.UUID) ([]domain.LegalEntity, error) {
	return u.repo.ListByCompany(ctx, companyUUID)
}

func (u *Usecase) DeleteEntity(ctx context.Context, uid uuid.UUID) error {
	return u.repo.SoftDeleteEntity(ctx, uid)
}

func (u *Usecase) CreateAccount(ctx context.Context, cmd CreateAccountCommand) (*domain.BankAccount, error) {
	if err := validateBIK(cmd.BIK); err != nil {
		return nil, fmt.Errorf("validate bik: %w", err)
	}

	if err := validatePayAcc(cmd.PayAcc); err != nil {
		return nil, fmt.Errorf("validate pay account: %w", err)
	}

	if _, err := u.repo.GetEntity(ctx, cmd.LegalEntityUUID); err != nil {
		return nil, fmt.Errorf("get entity: %w", err)
	}

	currency := cmd.Currency
	if currency == "" {
		currency = "RUB"
	}

	ba := &domain.BankAccount{
		UUID:            uuid.New(),
		LegalEntityUUID: cmd.LegalEntityUUID,
		Bank:            cmd.Bank,
		BIK:             cmd.BIK,
		CorrAcc:         cmd.CorrAcc,
		PayAcc:          cmd.PayAcc,
		Address:         cmd.Address,
		Currency:        currency,
		Comment:         cmd.Comment,
		IsPrimary:       cmd.IsPrimary,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := u.repo.CreateAccount(ctx, ba); err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}

	u.log.InfoContext(ctx, "bank account created",
		"uuid", ba.UUID,
		"legal_entity_uuid", ba.LegalEntityUUID,
		"bik", ba.BIK,
	)

	return ba, nil
}

func (u *Usecase) ListAccounts(ctx context.Context, legalEntityUUID uuid.UUID) ([]domain.BankAccount, error) {
	return u.repo.ListAccountsByEntity(ctx, legalEntityUUID)
}

func validateINN(inn string) error {
	inn = strings.TrimSpace(inn)

	if len(inn) != 10 && len(inn) != 12 {
		return errors.New("inn must be 10 or 12 digits")
	}

	for _, r := range inn {
		if r < '0' || r > '9' {
			return errors.New("inn must contain only digits")
		}
	}

	return nil
}

func validateBIK(bik string) error {
	bik = strings.TrimSpace(bik)

	if len(bik) != 9 {
		return errors.New("bik must be 9 digits")
	}

	for _, r := range bik {
		if r < '0' || r > '9' {
			return errors.New("bik must contain only digits")
		}
	}

	return nil
}

func validatePayAcc(acc string) error {
	acc = strings.TrimSpace(acc)

	if len(acc) != 20 {
		return errors.New("pay account must be 20 digits")
	}

	for _, r := range acc {
		if r < '0' || r > '9' {
			return errors.New("pay account must contain only digits")
		}
	}

	return nil
}
