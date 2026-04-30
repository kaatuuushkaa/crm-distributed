package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type NotificationType string

const (
	NotifTaskAssigned NotificationType = "task.assigned"

	NotifTaskStatusChanged NotificationType = "task.status_changed"

	NotifTaskCommentAdded NotificationType = "task.comment_added"

	NotifTaskDeadlineReminder NotificationType = "task.deadline_reminder"

	NotifProjectInvited NotificationType = "project.invited"

	NotifProjectArchived NotificationType = "project.archived"
)

type NotificationStatus int

const (
	NotifStatusPending   NotificationStatus = 0
	NotifStatusDelivered NotificationStatus = 1
	NotifStatusRead      NotificationStatus = 2
)

type Notification struct {
	UUID uuid.UUID

	RecipientUUID uuid.UUID

	Type NotificationType

	Title string
	Body  string

	EntityUUID uuid.UUID
	EntityType string

	Meta map[string]any

	Status NotificationStatus

	CreatedAt   time.Time
	DeliveredAt *time.Time
	ReadAt      *time.Time
}

func NewNotification(
	recipientUUID uuid.UUID,
	notifType NotificationType,
	title, body string,
	entityUUID uuid.UUID,
	entityType string,
) (*Notification, error) {
	if recipientUUID == uuid.Nil {
		return nil, errors.New("recipient UUID обязателен")
	}

	if notifType == "" {
		return nil, errors.New("тип уведомления обязателен")
	}

	if title == "" {
		return nil, errors.New("заголовок уведомления обязателен")
	}

	return &Notification{
		UUID:          uuid.New(),
		RecipientUUID: recipientUUID,
		Type:          notifType,
		Title:         title,
		Body:          body,
		EntityUUID:    entityUUID,
		EntityType:    entityType,
		Meta:          make(map[string]any),
		Status:        NotifStatusPending,
		CreatedAt:     time.Now(),
	}, nil
}

func (n *Notification) MarkDelivered() {
	now := time.Now()
	n.DeliveredAt = &now
	n.Status = NotifStatusDelivered
}

func (n *Notification) MarkRead() {
	now := time.Now()
	n.ReadAt = &now
	n.Status = NotifStatusRead
}

func (n *Notification) IsRead() bool {
	return n.Status == NotifStatusRead
}

type Reminder struct {
	UUID          uuid.UUID
	TaskUUID      uuid.UUID `validate:"required"`
	CreatedBy     string    `validate:"lte=100,gte=3"`
	CreatedByUUID uuid.UUID `validate:"required"`
	Description   string    `validate:"lte=5000"`
	Comment       string    `validate:"lte=5000"`
	Type          string    `validate:"gte=1,lte=50"`
	DateFrom      *time.Time
	DateTo        *time.Time
	UserUUID      *uuid.UUID
	Status        int `validate:"gte=0,lte=10"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
