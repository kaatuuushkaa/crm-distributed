package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Config struct {
	Brokers string `env:"KAFKA_BROKERS" envDefault:"localhost:9092"`
}

func (c Config) BrokerList() []string {
	return strings.Split(c.Brokers, ",")
}

type ConsumerConfig struct {
	Brokers []string `env:"KAFKA_BROKERS" envDefault:"localhost:9092"`
	GroupID string   `env:"KAFKA_GROUP_ID,required"`
	Topics  []string `env:"KAFKA_TOPICS,required"`
}

const (
	TopicTaskCreated    = "task.created"
	TopicTaskUpdated    = "task.updated"
	TopicTaskDeleted    = "task.deleted"
	TopicProjectCreated = "project.created"
	TopicCompanyCreated = "company.created"
)

type Producer struct {
	producer      sarama.SyncProducer
	log           *slog.Logger
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

	messagesSent := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_messages_produced_total",
		Help: "Общее количество успешно отправленных Kafka сообщений.",
	}, []string{"topic"})

	messagesError := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_producer_errors_total",
		Help: "Общее количество ошибок при отправке Kafka сообщений.",
	}, []string{"topic"})

	sendDuration := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kafka_producer_duration_seconds",
		Help:    "Время отправки Kafka сообщения в секундах.",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
	}, []string{"topic"})

	log.Info("kafka producer connected", "brokers", cfg.Brokers)

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
		Topic:     topic,
		Key:       sarama.StringEncoder(key),
		Value:     sarama.ByteEncoder(data),
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

type Handler func(ctx context.Context, msg *sarama.ConsumerMessage) error

type Consumer struct {
	cg      sarama.ConsumerGroup
	topics  []string
	handler Handler
	log     *slog.Logger
}

func NewConsumer(cfg ConsumerConfig, handler Handler, log *slog.Logger) (*Consumer, error) {
	c := sarama.NewConfig()
	c.Version = sarama.V3_6_0_0
	c.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}
	c.Consumer.Offsets.Initial = sarama.OffsetNewest

	cg, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, c)
	if err != nil {
		return nil, fmt.Errorf("kafka.NewConsumer: %w", err)
	}

	return &Consumer{cg: cg, topics: cfg.Topics, handler: handler, log: log}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	h := &consumerGroupHandler{handler: c.handler, log: c.log}

	for {
		if err := c.cg.Consume(ctx, c.topics, h); err != nil {
			return fmt.Errorf("consumer group: %w", err)
		}

		if ctx.Err() != nil {
			return nil
		}
	}
}

func (c *Consumer) Close() error { return c.cg.Close() }

type consumerGroupHandler struct {
	handler Handler
	log     *slog.Logger
}

func (h *consumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	for msg := range claim.Messages() {
		if err := h.handler(session.Context(), msg); err != nil {
			h.log.ErrorContext(session.Context(), "kafka handler error",
				"topic", msg.Topic,
				"offset", msg.Offset,
				"error", err,
			)
			continue
		}

		session.MarkMessage(msg, "")
	}

	return nil
}
