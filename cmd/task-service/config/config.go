package config

import (
	"fmt"

	"github.com/caarlos0/env/v6"

	"crm-distributed/shared/pkg/kafka"
	pkgpostgres "crm-distributed/shared/pkg/postgres"
	pkgredis "crm-distributed/shared/pkg/redis"
)

type Config struct {
	Env              string `env:"APP_ENV"                          envDefault:"development"`
	Port             int    `env:"TASK_SERVICE_PORT"                envDefault:"8080"`
	GracefulTimeout  int    `env:"TASK_SERVICE_GRACEFUL_TIMEOUT"    envDefault:"30"`
	NotifServiceAddr string `env:"TASK_SERVICE_GRPC_ADDR"          envDefault:"localhost:50051"`
	JWTSecret        string `env:"JWT_SECRET,required"`

	Postgres pkgpostgres.Config
	Redis    pkgredis.Config
	Kafka    kafka.Config
}

func Load() (*Config, error) {
	cfg := &Config{}

	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

func (c *Config) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}
