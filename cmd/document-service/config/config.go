package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v6"

	"crm-distributed/shared/pkg/kafka"
	mn "crm-distributed/shared/pkg/minio"
	"crm-distributed/shared/pkg/postgres"
)

type Config struct {
	Env             string        `env:"APP_ENV"          envDefault:"development"`
	HTTPPort        int           `env:"HTTP_PORT"        envDefault:"8082"`
	GRPCPort        int           `env:"GRPC_PORT"        envDefault:"50052"`
	GracefulTimeout time.Duration `env:"GRACEFUL_TIMEOUT" envDefault:"30s"`
	JWTSecret       string        `env:"JWT_SECRET,required"`

	Postgres postgres.Config
	MinIO    mn.Config
	Kafka    kafka.ConsumerConfig
}

func Load() (*Config, error) {
	cfg := &Config{}

	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}

	return cfg, nil
}
