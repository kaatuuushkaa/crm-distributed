package main

import (
	"context"
	"crm-distributed/cmd/task-service/config"
	"crm-distributed/shared/pkg/logger"
	"crm-distributed/shared/pkg/postgres"
	"crm-distributed/shared/pkg/redis"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	version = "dev"
	commit  = "none"
	builtAt = "unknown"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Env)

	log = log.With(
		"service", "task-service",
		"version", version,
		"commit", commit,
	)

	log.Info("starting task-service",
		"addr", cfg.Addr(),
		"env", cfg.Env,
		"built_at", builtAt,
	)

	db, err := postgres.New(cfg.Postgres, log)
	if err != nil {
		log.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}

	defer func() {
		if err = db.Close(); err != nil {
			log.Error("failed to close postgres connection", "error", err)
		}
	}()

	log.Info("connected to postgres", "host", cfg.Postgres.Host, "db", cfg.Postgres.DBName)

	rdb, err := redis.New(cfg.Redis)
	if err != nil {
		log.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}

	defer func() {
		if err = rdb.Close(); err != nil {
			log.Error("failed to close redis connection", "error", err)
		}
	}()

	log.Info("connected to redis", "host", cfg.Redis.Host)

	srv := server(cfg, log, db, rdb)

	log.Info("http server listening", "addr", cfg.Addr())

	go func() {
		log.Info("http server listening", "addr", cfg.Addr())

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	sig := <-quit

	log.Info("shutting down", "signal", sig.String())

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(cfg.GracefulTimeout)*time.Second,
	)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	log.Info("task-service stopped gracefully")
}
