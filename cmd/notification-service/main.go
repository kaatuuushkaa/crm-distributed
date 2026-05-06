package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"crm-distributed/cmd/notification-service/internal/handler"
	"crm-distributed/cmd/notification-service/internal/repository"
	"crm-distributed/cmd/notification-service/internal/usecase"
	pb "crm-distributed/proto/notification"
	"crm-distributed/shared/pkg/health"
	"crm-distributed/shared/pkg/kafka"
	"crm-distributed/shared/pkg/logger"
	"crm-distributed/shared/pkg/metrics"
	"crm-distributed/shared/pkg/redis"
)

const (
	httpReadTimeout  = 15 * time.Second
	httpWriteTimeout = 30 * time.Second
	httpIdleTimeout  = 60 * time.Second
	gracefulTimeout  = 30 * time.Second
)

type config struct {
	Env      string `env:"APP_ENV"        envDefault:"development"`
	HTTPPort int    `env:"HTTP_PORT"      envDefault:"8081"`
	GRPCPort int    `env:"GRPC_PORT"      envDefault:"50051"`
	LogLevel string `env:"LOG_LEVEL"      envDefault:"info"`
	Redis    redis.Config
	Kafka    kafka.ConsumerConfig
}

func main() {
	cfg := &config{}
	if err := env.Parse(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Env)
	log = log.With("service", "notification-service")

	if err := run(cfg, log); err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(cfg *config, log *slog.Logger) error {

	rdb, err := redis.New(cfg.Redis)
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}

	defer func() {
		if closeErr := rdb.Close(); closeErr != nil {
			log.Error("redis close", "error", closeErr)
		}
	}()

	log.Info("redis connected", "host", cfg.Redis.Host)

	notifRepo := repository.NewNotificationRepository(rdb)
	kafkaMetrics := metrics.NewKafkaMetrics("notification_service")
	notifUC := usecase.NewNotificationUsecase(notifRepo, rdb, kafkaMetrics, log)

	httpSrv := newHTTPServer(cfg.HTTPPort, log, rdb)

	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			handler.LoggingInterceptor(log),
		),
	)
	pb.RegisterNotificationServiceServer(grpcSrv, handler.NewGRPCHandler(notifUC, log))

	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("grpc listen: %w", err)
	}

	consumer, err := kafka.NewConsumer(cfg.Kafka, notifUC.HandleKafkaMessage, log)
	if err != nil {
		return fmt.Errorf("kafka consumer: %w", err)
	}

	defer func() {
		if closeErr := consumer.Close(); closeErr != nil {
			log.Error("kafka consumer close", "error", closeErr)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Info("http server started", "port", cfg.HTTPPort)

		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		log.Info("grpc server started", "port", cfg.GRPCPort)

		if err := grpcSrv.Serve(grpcLis); err != nil {
			return fmt.Errorf("grpc: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		log.Info("kafka consumer started", "topics", cfg.Kafka.Topics)

		return consumer.Run(gCtx)
	})

	g.Go(func() error {
		<-gCtx.Done()
		log.Info("shutting down")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulTimeout)
		defer cancel()

		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Error("http shutdown", "error", err)
		}

		grpcSrv.GracefulStop()

		return nil
	})

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("service stopped: %w", err)
	}

	log.Info("notification-service stopped gracefully")

	return nil
}

func newHTTPServer(port int, log *slog.Logger, rdb *redis.Client) *http.Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Recover())
	e.Use(echoprometheus.NewMiddleware("notification_service"))

	healthHandler := health.New(map[string]health.Checker{
		"redis": rdb,
	})

	e.GET("/healthz", func(c echo.Context) error {
		healthHandler.Liveness(c.Response(), c.Request())
		return nil
	})
	e.GET("/readyz", func(c echo.Context) error {
		healthHandler.Readiness(c.Response(), c.Request())
		return nil
	})
	e.GET("/metrics", echoprometheus.NewHandler())

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      e,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
		IdleTimeout:  httpIdleTimeout,
	}
}
