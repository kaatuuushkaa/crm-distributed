package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var (
	ErrFederationNotFound = errors.New("федерация не найдена")
	ErrFederationInvalid  = errors.New("некорректные данные федерации")
)

type Federation struct {
	UUID          uuid.UUID
	Name          string `validate:"lte=100,gte=1"`
	CreatedBy     string // email создателя
	CreatedByUUID uuid.UUID
	Meta          datatypes.JSON

	Users          []FederationUser
	UsersTotal     int
	Companies      []Company
	CompaniesTotal int

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewFederation(name, createdBy string, createdByUUID uuid.UUID) (*Federation, error) {
	if len(name) < 1 || len(name) > 100 {
		return nil, fmt.Errorf("%w: название от 1 до 100 символов", ErrFederationInvalid)
	}

	if createdBy == "" {
		return nil, fmt.Errorf("%w: email создателя обязателен", ErrFederationInvalid)
	}

	uid := uuid.New()

	firstMember, err := NewFederationUser(uid, createdByUUID)
	if err != nil {
		return nil, fmt.Errorf("create first federation member: %w", err)
	}

	return &Federation{
		UUID:          uid,
		Name:          name,
		CreatedBy:     createdBy,
		CreatedByUUID: createdByUUID,
		Meta:          datatypes.JSON("{}"),
		Users:         []FederationUser{*firstMember},
		CreatedAt:     time.Now(),
	}, nil
}

func NewFederationByUUID(uid uuid.UUID) *Federation {
	return &Federation{UUID: uid}
}

func (f *Federation) ChangeName(name string) error {
	if len(name) < 1 || len(name) > 100 {
		return errors.New("название федерации от 1 до 100 символов")
	}

	f.Name = name

	return nil
}

type FederationUser struct {
	UUID           uuid.UUID `validate:"required"`
	User           User
	FederationUUID uuid.UUID `validate:"required"`
	AddedAt        time.Time `json:"added_at"`
}

func NewFederationUser(federationUUID, userUUID uuid.UUID) (*FederationUser, error) {
	if federationUUID == uuid.Nil {
		return nil, errors.New("federation UUID обязателен")
	}

	if userUUID == uuid.Nil {
		return nil, errors.New("user UUID обязателен")
	}

	return &FederationUser{
		UUID:           uuid.New(),
		User:           User{UUID: userUUID},
		FederationUUID: federationUUID,
		AddedAt:        time.Now(),
	}, nil
}
