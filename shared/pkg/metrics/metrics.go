package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type KafkaMetrics struct {
	MessagesProcessed  *prometheus.CounterVec
	ProcessingDuration *prometheus.HistogramVec
}

func NewKafkaMetrics(namespace string) *KafkaMetrics {
	return &KafkaMetrics{
		MessagesProcessed: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "kafka_messages_processed_total",
			Help:      "Total number of processed Kafka messages.",
		}, []string{"topic", "status"}),

		ProcessingDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "kafka_message_processing_duration_seconds",
			Help:      "Kafka message processing duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"topic"}),
	}
}
