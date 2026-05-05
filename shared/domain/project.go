package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrProjectNotFound = errors.New("проект не найден")
	ErrProjectInvalid  = errors.New("некорректные данные проекта")
)

type Project struct {
	UUID           uuid.UUID
	FederationUUID uuid.UUID
	CompanyUUID    uuid.UUID
	Name           string `validate:"lte=100,gte=3"`
	Description    string `validate:"lte=5000"`
	CreatedBy      string
	ResponsibleBy  string

	StatusGraph *StatusGraph
	Options     ProjectOptions
	Fields      []CompanyField

	Users      []ProjectUser
	StatusSort []int
	FieldsSort []string

	StatusCode      int
	StatusUpdatedAt *time.Time
	Meta            []byte

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewProject(
	name, description string,
	federationUUID, companyUUID uuid.UUID,
	createdBy, responsibleBy string,
) (*Project, error) {
	if len(name) < 3 || len(name) > 100 {
		return nil, fmt.Errorf("%w: название от 3 до 100 символов", ErrProjectInvalid)
	}

	if createdBy == "" {
		return nil, fmt.Errorf("%w: создатель обязателен", ErrProjectInvalid)
	}

	return &Project{
		UUID:           uuid.New(),
		Name:           name,
		Description:    description,
		FederationUUID: federationUUID,
		CompanyUUID:    companyUUID,
		CreatedBy:      createdBy,
		ResponsibleBy:  responsibleBy,
		Meta:           []byte("{}"),
		CreatedAt:      time.Now(),
	}, nil
}

func (p *Project) ChangeName(name string) error {
	if len(name) < 3 || len(name) > 100 {
		return errors.New("название проекта от 3 до 100 символов")
	}

	p.Name = name

	return nil
}

func (p *Project) ChangeDescription(description string) error {
	if len(description) > 5000 {
		return errors.New("описание проекта до 5000 символов")
	}

	p.Description = description

	return nil
}

type ProjectOptions struct {
	RequireCancelationComment *bool   `json:"require_cancelation_comment,omitempty"`
	RequireDoneComment        *bool   `json:"require_done_comment,omitempty"`
	StatusEnable              *bool   `json:"status_enable,omitempty"`
	Color                     *string `json:"color,omitempty"`
}

func (o ProjectOptions) NeedsCancelComment() bool {
	return o.RequireCancelationComment != nil && *o.RequireCancelationComment
}

func (o ProjectOptions) NeedsDoneComment() bool {
	return o.RequireDoneComment != nil && *o.RequireDoneComment
}

func (o *ProjectOptions) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("ProjectOptions.Scan: expected []byte, got %T", value)
	}

	defaultColor := "#111111"
	defaultFalse := false
	result := ProjectOptions{
		Color:                     &defaultColor,
		RequireCancelationComment: &defaultFalse,
		RequireDoneComment:        &defaultFalse,
		StatusEnable:              &defaultFalse,
	}

	if err := json.Unmarshal(bytes, &result); err != nil {
		return fmt.Errorf("ProjectOptions.Scan: %w", err)
	}

	*o = result

	return nil
}

func (o ProjectOptions) Value() (driver.Value, error) {
	return json.Marshal(o)
}

type FieldDataType int

const (
	FieldInteger   FieldDataType = 0
	FieldFloat     FieldDataType = 1
	FieldString    FieldDataType = 2
	FieldText      FieldDataType = 3
	FieldBool      FieldDataType = 4
	FieldSwitch    FieldDataType = 5
	FieldArray     FieldDataType = 6
	FieldData      FieldDataType = 7
	FieldDataArray FieldDataType = 8
	FieldPhone     FieldDataType = 9
	FieldLink      FieldDataType = 10
	FieldEmail     FieldDataType = 11
	FieldTime      FieldDataType = 12
	FieldDateTime  FieldDataType = 13
	FieldPeople    FieldDataType = 14
)

func (f FieldDataType) String() string {
	names := map[FieldDataType]string{
		FieldInteger: "integer", FieldFloat: "float", FieldString: "string",
		FieldText: "text", FieldBool: "bool", FieldSwitch: "switch",
		FieldArray: "array", FieldData: "data", FieldDataArray: "data_array",
		FieldPhone: "phone", FieldLink: "link", FieldEmail: "email",
		FieldTime: "time", FieldDateTime: "datetime", FieldPeople: "people",
	}

	if name, ok := names[f]; ok {
		return name
	}

	return "unknown"
}

// CompanyField — кастомное поле задачи определённое на уровне компании.
type CompanyField struct {
	UUID        uuid.UUID
	Hash        string
	Name        string        `validate:"lte=30,gte=1"`
	Description string        `validate:"lte=5000"`
	Icon        string        `validate:"lte=50"`
	DataType    FieldDataType `validate:"lte=14,gte=0"`
	CompanyUUID uuid.UUID     `validate:"required"`
	ProjectUUID []uuid.UUID
	Style       string `validate:"lte=20"`
	CreatedBy   string

	RequiredOnStatuses []int

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
	Meta      []byte
}

type ProjectUser struct {
	UUID           uuid.UUID `validate:"required"`
	User           User
	FederationUUID uuid.UUID `validate:"required"`
	CompanyUUID    uuid.UUID `validate:"required"`
	ProjectUUID    uuid.UUID `validate:"required"`
	AddedAt        time.Time `json:"added_at"`
}

func NewProjectUser(
	federationUUID, companyUUID, projectUUID, userUUID uuid.UUID,
) (*ProjectUser, error) {
	if federationUUID == uuid.Nil || companyUUID == uuid.Nil ||
		projectUUID == uuid.Nil || userUUID == uuid.Nil {
		return nil, errors.New("все UUID обязательны для создания участника проекта")
	}

	return &ProjectUser{
		UUID:           uuid.New(),
		User:           User{UUID: userUUID},
		FederationUUID: federationUUID,
		CompanyUUID:    companyUUID,
		ProjectUUID:    projectUUID,
		AddedAt:        time.Now(),
	}, nil
}
