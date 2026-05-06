package repository

import (
	"context"
	"fmt"
	"time"

	"crm-distributed/shared/pkg/redis"
)

type Notification struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      int32     `json:"type"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	RefID     string    `json:"ref_id"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

type NotificationRepository struct {
	rdb *redis.Client
}

func NewNotificationRepository(rdb *redis.Client) *NotificationRepository {
	return &NotificationRepository{rdb: rdb}
}

func (r *NotificationRepository) Save(ctx context.Context, n Notification) error {
	key := fmt.Sprintf("notif:%s:%s", n.UserID, n.ID)

	if err := r.rdb.Set(ctx, key, n, 30*24*time.Hour); err != nil {
		return fmt.Errorf("save notification: %w", err)
	}

	return nil
}

func (r *NotificationRepository) GetUnread(ctx context.Context, userID string, limit int) ([]Notification, error) {
	if limit <= 0 {
		limit = 50
	}

	pattern := fmt.Sprintf("notif:%s:*", userID)

	var notifications []Notification
	var cursor uint64

	for {
		keys, newCursor, err := r.rdb.Scan(ctx, cursor, pattern, int64(limit))
		if err != nil {
			return nil, fmt.Errorf("scan notifications: %w", err)
		}

		for _, key := range keys {
			var n Notification

			found, err := r.rdb.Get(ctx, key, &n)
			if err != nil || !found {
				continue
			}

			if !n.IsRead {
				notifications = append(notifications, n)
			}
		}

		cursor = newCursor
		if cursor == 0 {
			break
		}
	}

	return notifications, nil
}

func (r *NotificationRepository) MarkRead(ctx context.Context, userID string, notifIDs []string) (int32, error) {
	var count int32

	if len(notifIDs) == 0 {
		pattern := fmt.Sprintf("notif:%s:*", userID)
		var cursor uint64

		for {
			keys, newCursor, err := r.rdb.Scan(ctx, cursor, pattern, 100)
			if err != nil {
				break
			}

			for _, key := range keys {
				if r.markOneRead(ctx, key) {
					count++
				}
			}

			cursor = newCursor
			if cursor == 0 {
				break
			}
		}

		return count, nil
	}

	for _, id := range notifIDs {
		key := fmt.Sprintf("notif:%s:%s", userID, id)
		if r.markOneRead(ctx, key) {
			count++
		}
	}

	return count, nil
}

func (r *NotificationRepository) markOneRead(ctx context.Context, key string) bool {
	var n Notification

	found, err := r.rdb.Get(ctx, key, &n)
	if err != nil || !found || n.IsRead {
		return false
	}

	n.IsRead = true

	if err = r.rdb.Set(ctx, key, n, 30*24*time.Hour); err != nil {
		return false
	}

	return true
}
