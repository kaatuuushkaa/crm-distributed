package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrLegalEntityNotFound = errors.New("юридическое лицо не найдено")
	ErrLegalEntityInvalid  = errors.New("некорректные данные юридического лица")
	ErrBankAccountNotFound = errors.New("банковский счёт не найден")
)

type LegalEntity struct {
	UUID        uuid.UUID
	CompanyUUID uuid.UUID

	Name string
	INN  string
	KPP  string

	LegalAddress  string
	ActualAddress string

	BankAccounts []BankAccount

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewLegalEntity(
	companyUUID uuid.UUID,
	name, inn, kpp, legalAddress, actualAddress string,
) (*LegalEntity, error) {
	if companyUUID == uuid.Nil {
		return nil, fmt.Errorf("%w: company UUID обязателен", ErrLegalEntityInvalid)
	}

	if name == "" {
		return nil, fmt.Errorf("%w: название обязательно", ErrLegalEntityInvalid)
	}

	if err := validateINN(inn); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrLegalEntityInvalid, err)
	}

	now := time.Now()

	return &LegalEntity{
		UUID:          uuid.New(),
		CompanyUUID:   companyUUID,
		Name:          name,
		INN:           inn,
		KPP:           kpp,
		LegalAddress:  legalAddress,
		ActualAddress: actualAddress,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func validateINN(inn string) error {
	if len(inn) != 10 && len(inn) != 12 {
		return errors.New("ИНН должен содержать 10 или 12 цифр")
	}

	for _, r := range inn {
		if r < '0' || r > '9' {
			return errors.New("ИНН должен содержать только цифры")
		}
	}

	return nil
}

type LegalEntityBank struct {
	UUID         uuid.UUID
	Name         string
	BankAccounts []BankAccount

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type BankAccount struct {
	UUID            uuid.UUID
	LegalEntityUUID uuid.UUID

	Bank    string
	BIK     string
	CorrAcc string
	PayAcc  string

	Address  string
	Currency string
	Comment  string

	IsPrimary bool

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewBankAccount(
	legalEntityUUID uuid.UUID,
	bank, bik, corrAcc, payAcc, currency string,
) (*BankAccount, error) {
	if legalEntityUUID == uuid.Nil {
		return nil, errors.New("legal entity UUID обязателен")
	}

	if len(bik) != 9 {
		return nil, errors.New("БИК должен содержать 9 цифр")
	}

	if len(corrAcc) != 20 {
		return nil, errors.New("корреспондентский счёт должен содержать 20 цифр")
	}

	if len(payAcc) != 20 {
		return nil, errors.New("расчётный счёт должен содержать 20 цифр")
	}

	now := time.Now()

	return &BankAccount{
		UUID:            uuid.New(),
		LegalEntityUUID: legalEntityUUID,
		Bank:            bank,
		BIK:             bik,
		CorrAcc:         corrAcc,
		PayAcc:          payAcc,
		Currency:        currency,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}
