package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/IBM/sarama"
	"log/slog"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Config struct {
	Brokers string `env:"KAFKA_BROKERS" envDefault:"localhost:9092"`
}

func (c Config) BrokerList() []string {
	return strings.Split(c.Brokers, ",")
}

const (
	TopicTaskCreated = "task.created"
	TopicTaskUpdated = "task.updated"
	TopicTaskDeleted = "task.deleted"

	TopicProjectCreated = "project.created"

	TopicCompanyCreated = "company.created"
)

type Producer struct {
	producer sarama.SyncProducer
	log      *slog.Logger

	messagesSent  *prometheus.CounterVec
	messagesError *prometheus.CounterVec
	sendDuration  *prometheus.HistogramVec
}

func NewProducer(cfg Config, log *slog.Logger) (*Producer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_6_0_0
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Retry.Max = 3
	config.Producer.Retry.Backoff = 100 * time.Millisecond
	config.Producer.RequiredAcks = sarama.WaitForLocal
	config.Producer.Compression = sarama.CompressionSnappy

	producer, err := sarama.NewSyncProducer(cfg.BrokerList(), config)
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}

	messagesSent := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_messages_produced_total",
			Help: "Общее количество успешно отправленных Kafka сообщений.",
		},
		[]string{"topic"},
	)

	messagesError := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_producer_errors_total",
			Help: "Общее количество ошибок при отправке Kafka сообщений.",
		},
		[]string{"topic"},
	)

	sendDuration := promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kafka_producer_duration_seconds",
			Help:    "Время отправки Kafka сообщения в секундах.",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"topic"},
	)

	log.Info("kafka producer connected",
		"brokers", cfg.Brokers,
	)

	return &Producer{
		producer:      producer,
		log:           log,
		messagesSent:  messagesSent,
		messagesError: messagesError,
		sendDuration:  sendDuration,
	}, nil
}

func (p *Producer) Send(ctx context.Context, topic, key string, payload any) error {
	start := time.Now()

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal kafka payload: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,

		Key: sarama.StringEncoder(key),

		Value: sarama.ByteEncoder(data),

		Timestamp: time.Now(),
	}

	_, _, err = p.producer.SendMessage(msg)

	elapsed := time.Since(start)
	p.sendDuration.WithLabelValues(topic).Observe(elapsed.Seconds())

	if err != nil {
		p.messagesError.WithLabelValues(topic).Inc()
		p.log.ErrorContext(ctx, "kafka send error",
			"topic", topic,
			"key", key,
			"elapsed_ms", elapsed.Milliseconds(),
			"error", err,
		)

		return fmt.Errorf("send to topic %s: %w", topic, err)
	}

	p.messagesSent.WithLabelValues(topic).Inc()
	p.log.DebugContext(ctx, "kafka message sent",
		"topic", topic,
		"key", key,
		"elapsed_ms", elapsed.Milliseconds(),
		"size_bytes", len(data),
	)

	return nil
}

func (p *Producer) Close() error {
	if err := p.producer.Close(); err != nil {
		return fmt.Errorf("close kafka producer: %w", err)
	}

	return nil
}
