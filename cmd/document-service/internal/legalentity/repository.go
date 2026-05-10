package legalentity

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

func (r *Repository) CreateEntity(ctx context.Context, le *domain.LegalEntity) error {
	const query = `
		INSERT INTO legal_entities (
			uuid, company_uuid, name, inn, kpp,
			legal_address, actual_address,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`

	_, err := r.db.ExecContext(ctx, "legal_entity.create", query,
		le.UUID, le.CompanyUUID, le.Name, le.INN, le.KPP,
		le.LegalAddress, le.ActualAddress,
		le.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert legal entity: %w", err)
	}

	return nil
}

func (r *Repository) GetEntity(ctx context.Context, uid uuid.UUID) (*domain.LegalEntity, error) {
	const query = `
		SELECT uuid, company_uuid, name, inn, kpp,
		       legal_address, actual_address,
		       created_at, updated_at
		FROM legal_entities
		WHERE uuid = $1 AND deleted_at IS NULL`

	var le domain.LegalEntity

	err := r.db.QueryRowContext(ctx, "legal_entity.get", query, uid).Scan(
		&le.UUID, &le.CompanyUUID, &le.Name, &le.INN, &le.KPP,
		&le.LegalAddress, &le.ActualAddress,
		&le.CreatedAt, &le.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrLegalEntityNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query legal entity: %w", err)
	}

	return &le, nil
}

func (r *Repository) ListByCompany(ctx context.Context, companyUUID uuid.UUID) ([]domain.LegalEntity, error) {
	const query = `
		SELECT uuid, company_uuid, name, inn, kpp,
		       legal_address, actual_address,
		       created_at, updated_at
		FROM legal_entities
		WHERE company_uuid = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, "legal_entity.list_by_company", query, companyUUID)
	if err != nil {
		return nil, fmt.Errorf("query legal entities: %w", err)
	}
	defer rows.Close()

	var entities []domain.LegalEntity

	for rows.Next() {
		var le domain.LegalEntity

		if err := rows.Scan(
			&le.UUID, &le.CompanyUUID, &le.Name, &le.INN, &le.KPP,
			&le.LegalAddress, &le.ActualAddress,
			&le.CreatedAt, &le.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan legal entity: %w", err)
		}

		entities = append(entities, le)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return entities, nil
}

func (r *Repository) SoftDeleteEntity(ctx context.Context, uid uuid.UUID) error {
	const query = `
		UPDATE legal_entities
		SET deleted_at = $1, updated_at = $1
		WHERE uuid = $2 AND deleted_at IS NULL`

	res, err := r.db.ExecContext(ctx, "legal_entity.delete", query, time.Now(), uid)
	if err != nil {
		return fmt.Errorf("soft delete legal entity: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if rows == 0 {
		return domain.ErrLegalEntityNotFound
	}

	return nil
}

func (r *Repository) ExistsByINN(ctx context.Context, inn string) (bool, error) {
	const query = `
		SELECT 1 FROM legal_entities
		WHERE inn = $1 AND deleted_at IS NULL
		LIMIT 1`

	var exists int

	err := r.db.QueryRowContext(ctx, "legal_entity.exists_by_inn", query, inn).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check inn: %w", err)
	}

	return true, nil
}

func (r *Repository) CreateAccount(ctx context.Context, ba *domain.BankAccount) error {
	const query = `
		INSERT INTO bank_accounts (
			uuid, legal_entity_uuid, bank, bik,
			corr_acc, pay_acc, address, currency, comment, is_primary,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)`

	_, err := r.db.ExecContext(ctx, "bank_account.create", query,
		ba.UUID, ba.LegalEntityUUID, ba.Bank, ba.BIK,
		ba.CorrAcc, ba.PayAcc, ba.Address, ba.Currency, ba.Comment, ba.IsPrimary,
		ba.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert bank account: %w", err)
	}

	return nil
}

func (r *Repository) ListAccountsByEntity(ctx context.Context, legalEntityUUID uuid.UUID) ([]domain.BankAccount, error) {
	const query = `
		SELECT uuid, legal_entity_uuid, bank, bik,
		       corr_acc, pay_acc, address, currency, comment, is_primary,
		       created_at, updated_at
		FROM bank_accounts
		WHERE legal_entity_uuid = $1 AND deleted_at IS NULL
		ORDER BY is_primary DESC, created_at DESC`

	rows, err := r.db.QueryContext(ctx, "bank_account.list", query, legalEntityUUID)
	if err != nil {
		return nil, fmt.Errorf("query bank accounts: %w", err)
	}
	defer rows.Close()

	var accounts []domain.BankAccount

	for rows.Next() {
		var ba domain.BankAccount

		if err := rows.Scan(
			&ba.UUID, &ba.LegalEntityUUID, &ba.Bank, &ba.BIK,
			&ba.CorrAcc, &ba.PayAcc, &ba.Address, &ba.Currency, &ba.Comment, &ba.IsPrimary,
			&ba.CreatedAt, &ba.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan bank account: %w", err)
		}

		accounts = append(accounts, ba)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return accounts, nil
}
