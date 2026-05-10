package legalentity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/IBM/sarama"

	"crm-distributed/shared/domain"
	"crm-distributed/shared/pkg/kafka"
)

type KafkaConsumer struct {
	uc  *Usecase
	log *slog.Logger
}

func NewKafkaConsumer(uc *Usecase, log *slog.Logger) *KafkaConsumer {
	return &KafkaConsumer{uc: uc, log: log}
}

func (c *KafkaConsumer) HandleMessage(ctx context.Context, msg *sarama.ConsumerMessage) error {
	switch msg.Topic {
	case kafka.TopicCompanyCreated:
		return c.handleCompanyCreated(ctx, msg.Value)
	default:
		c.log.WarnContext(ctx, "unknown topic", "topic", msg.Topic)
		return nil
	}
}

func (c *KafkaConsumer) handleCompanyCreated(ctx context.Context, data []byte) error {
	var event kafka.CompanyCreatedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal company.created: %w", err)
	}

	c.log.InfoContext(ctx, "company.created received",
		"company_uuid", event.CompanyUUID,
		"company_name", event.CompanyName,
	)

	cmd := CreateEntityCommand{
		CompanyUUID:   event.CompanyUUID,
		Name:          event.CompanyName,
		INN:           generatePlaceholderINN(event.CompanyUUID.String()),
		LegalAddress:  "",
		ActualAddress: "",
	}

	_, err := c.uc.CreateEntity(ctx, cmd)
	if err != nil {
		if errors.Is(err, domain.ErrLegalEntityAlreadyExists) {
			c.log.DebugContext(ctx, "legal entity already exists, skipping",
				"company_uuid", event.CompanyUUID,
			)
			return nil
		}

		return fmt.Errorf("auto create legal entity: %w", err)
	}

	return nil
}

func generatePlaceholderINN(uuidStr string) string {
	digits := make([]byte, 0, 12)

	for i := 0; i < len(uuidStr) && len(digits) < 12; i++ {
		c := uuidStr[i]
		if c >= '0' && c <= '9' {
			digits = append(digits, c)
		}
	}

	for len(digits) < 12 {
		digits = append(digits, '0')
	}

	return string(digits)
}
