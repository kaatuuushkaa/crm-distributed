package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"

	"crm-distributed/cmd/notification-service/internal/repository"
	"crm-distributed/shared/pkg/kafka"
	"crm-distributed/shared/pkg/metrics"
	"crm-distributed/shared/pkg/redis"
)

type NotificationUsecase struct {
	repo    *repository.NotificationRepository
	rdb     *redis.Client
	metrics *metrics.KafkaMetrics
	log     *slog.Logger
}

func NewNotificationUsecase(
	repo *repository.NotificationRepository,
	rdb *redis.Client,
	m *metrics.KafkaMetrics,
	log *slog.Logger,
) *NotificationUsecase {
	return &NotificationUsecase{
		repo:    repo,
		rdb:     rdb,
		metrics: m,
		log:     log,
	}
}

func (uc *NotificationUsecase) Send(
	ctx context.Context,
	userIDs []string,
	notifType int32,
	title, body, refID string,
) (string, int32, error) {
	notifID := uuid.New().String()
	var delivered int32

	for _, userID := range userIDs {
		n := repository.Notification{
			ID:        notifID,
			UserID:    userID,
			Type:      notifType,
			Title:     title,
			Body:      body,
			RefID:     refID,
			IsRead:    false,
			CreatedAt: time.Now(),
		}

		if err := uc.repo.Save(ctx, n); err != nil {
			uc.log.ErrorContext(ctx, "failed to save notification",
				"user_id", userID,
				"error", err,
			)
			continue
		}

		data, _ := json.Marshal(n)
		channel := fmt.Sprintf("ws:notif:%s", userID)

		if err := uc.rdb.Publish(ctx, channel, data); err != nil {
			uc.log.WarnContext(ctx, "failed to publish to pubsub",
				"channel", channel,
				"error", err,
			)
		}

		delivered++
	}

	uc.log.InfoContext(ctx, "notifications sent",
		"notif_id", notifID,
		"recipients", len(userIDs),
		"delivered", delivered,
	)

	return notifID, delivered, nil
}

func (uc *NotificationUsecase) GetUnread(ctx context.Context, userID string, limit int) ([]repository.Notification, error) {
	return uc.repo.GetUnread(ctx, userID, limit)
}

func (uc *NotificationUsecase) MarkRead(ctx context.Context, userID string, notifIDs []string) (int32, error) {
	return uc.repo.MarkRead(ctx, userID, notifIDs)
}

func (uc *NotificationUsecase) HandleKafkaMessage(ctx context.Context, msg *sarama.ConsumerMessage) error {
	start := time.Now()

	var err error

	switch msg.Topic {
	case kafka.TopicTaskCreated:
		err = uc.handleTaskCreated(ctx, msg.Value)
	case kafka.TopicTaskUpdated:
		err = uc.handleTaskUpdated(ctx, msg.Value)
	default:
		uc.log.WarnContext(ctx, "unknown kafka topic", "topic", msg.Topic)
		return nil
	}

	if err != nil {
		uc.metrics.MessagesProcessed.WithLabelValues(msg.Topic, "error").Inc()
		return err
	}

	uc.metrics.MessagesProcessed.WithLabelValues(msg.Topic, "success").Inc()
	uc.metrics.ProcessingDuration.WithLabelValues(msg.Topic).Observe(time.Since(start).Seconds())

	return nil
}

func (uc *NotificationUsecase) handleTaskCreated(ctx context.Context, data []byte) error {
	var event kafka.TaskCreatedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal task.created: %w", err)
	}

	title := "Новая задача"
	body := fmt.Sprintf("Задача #%d «%s» создана", event.TaskID, event.TaskName)

	_, _, err := uc.Send(ctx, event.People, 1, title, body, event.TaskUUID.String())
	return err
}

func (uc *NotificationUsecase) handleTaskUpdated(ctx context.Context, data []byte) error {
	var event kafka.TaskUpdatedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal task.updated: %w", err)
	}

	title := "Задача обновлена"
	body := fmt.Sprintf("Задача #%d «%s» изменена пользователем %s",
		event.TaskID, event.TaskName, event.ChangedBy)

	_, _, err := uc.Send(ctx, event.People, 2, title, body, event.TaskUUID.String())
	return err
}
