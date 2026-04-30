package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var (
	ErrCompanyNotFound = errors.New("компания не найдена")
	ErrCompanyInvalid  = errors.New("некорректные данные компании")
)

type Company struct {
	UUID           uuid.UUID
	FederationUUID uuid.UUID
	Name           string `validate:"gte=3,lte=100"`

	CreatedBy     string
	CreatedByUUID uuid.UUID
	Meta          datatypes.JSON

	Users         []CompanyUser
	UserTotal     int
	Fields        []CompanyField
	Projects      []Project
	ProjectsTotal int

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewCompany(name string, federationUUID uuid.UUID, createdBy string, createdByUUID uuid.UUID) (*Company, error) {
	if len(name) < 3 || len(name) > 100 {
		return nil, fmt.Errorf("%w: название от 3 до 100 символов", ErrCompanyInvalid)
	}

	if federationUUID == uuid.Nil {
		return nil, fmt.Errorf("%w: federation UUID обязателен", ErrCompanyInvalid)
	}

	return &Company{
		UUID:           uuid.New(),
		Name:           name,
		FederationUUID: federationUUID,
		CreatedBy:      createdBy,
		CreatedByUUID:  createdByUUID,
		Meta:           datatypes.JSON("{}"),
		CreatedAt:      time.Now(),
	}, nil
}

func NewCompanyByUUID(uid uuid.UUID) *Company {
	return &Company{UUID: uid}
}

func (c *Company) ChangeName(name string) error {
	if len(name) < 3 || len(name) > 100 {
		return errors.New("название компании от 3 до 100 символов")
	}

	c.Name = name

	return nil
}

type CompanyUser struct {
	UUID           uuid.UUID `validate:"required"`
	User           User
	FederationUUID uuid.UUID `validate:"required"`
	CompanyUUID    uuid.UUID `validate:"required"`
	AddedAt        time.Time `json:"added_at"`
}

func NewCompanyUser(federationUUID, companyUUID, userUUID uuid.UUID) (*CompanyUser, error) {
	if federationUUID == uuid.Nil || companyUUID == uuid.Nil || userUUID == uuid.Nil {
		return nil, errors.New("все UUID обязательны для создания участника компании")
	}

	return &CompanyUser{
		UUID:           uuid.New(),
		User:           User{UUID: userUUID},
		FederationUUID: federationUUID,
		CompanyUUID:    companyUUID,
		AddedAt:        time.Now(),
	}, nil
}

type CompanyPriority struct {
	CompanyUUID uuid.UUID `validate:"required"`
	UUID        uuid.UUID `json:"uuid"`
	Name        string    `json:"name"`
	Number      int       `json:"priority"`
	Color       string    `json:"color"`
}
