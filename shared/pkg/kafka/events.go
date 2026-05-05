package kafka

import (
	"time"

	"github.com/google/uuid"
)

type TaskCreatedEvent struct {
	TaskUUID       uuid.UUID `json:"task_uuid"`
	TaskID         int       `json:"task_id"`
	TaskName       string    `json:"task_name"`
	FederationUUID uuid.UUID `json:"federation_uuid"`
	CompanyUUID    uuid.UUID `json:"company_uuid"`
	ProjectUUID    uuid.UUID `json:"project_uuid"`
	CreatedBy      string    `json:"created_by"`
	ImplementBy    string    `json:"implement_by"`
	People         []string  `json:"people"`
	CreatedAt      time.Time `json:"created_at"`
}

type TaskUpdatedEvent struct {
	TaskUUID      uuid.UUID      `json:"task_uuid"`
	TaskID        int            `json:"task_id"`
	TaskName      string         `json:"task_name"`
	ProjectUUID   uuid.UUID      `json:"project_uuid"`
	CompanyUUID   uuid.UUID      `json:"company_uuid"`
	ChangedBy     string         `json:"changed_by"`
	ChangedFields map[string]any `json:"changed_fields"`
	People        []string       `json:"people"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type TaskDeletedEvent struct {
	TaskUUID    uuid.UUID `json:"task_uuid"`
	TaskID      int       `json:"task_id"`
	TaskName    string    `json:"task_name"`
	ProjectUUID uuid.UUID `json:"project_uuid"`
	DeletedBy   string    `json:"deleted_by"`
	DeletedAt   time.Time `json:"deleted_at"`
}

type ProjectCreatedEvent struct {
	ProjectUUID    uuid.UUID `json:"project_uuid"`
	ProjectName    string    `json:"project_name"`
	FederationUUID uuid.UUID `json:"federation_uuid"`
	CompanyUUID    uuid.UUID `json:"company_uuid"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

type CompanyCreatedEvent struct {
	CompanyUUID    uuid.UUID `json:"company_uuid"`
	CompanyName    string    `json:"company_name"`
	FederationUUID uuid.UUID `json:"federation_uuid"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}
