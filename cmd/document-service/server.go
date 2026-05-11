package main

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"net"
	"net/http"
	"time"

	_ "crm-distributed/cmd/task-service/docs"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"
	"google.golang.org/grpc"

	"crm-distributed/cmd/document-service/config"
	"crm-distributed/cmd/document-service/internal/file"
	"crm-distributed/cmd/document-service/internal/legalentity"
	pb "crm-distributed/proto/document"
	"crm-distributed/shared/pkg/health"
	"crm-distributed/shared/pkg/kafka"
	mn "crm-distributed/shared/pkg/minio"
	"crm-distributed/shared/pkg/postgres"
)

type requestValidator struct {
	v *validator.Validate
}

func (rv *requestValidator) Validate(i any) error {
	return rv.v.Struct(i)
}

func Run(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	db, err := postgres.New(cfg.Postgres, log)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error("postgres close", "error", closeErr)
		}
	}()
	log.Info("postgres connected", "host", cfg.Postgres.Host)

	minioClient, err := mn.New(ctx, cfg.MinIO, log)
	if err != nil {
		return fmt.Errorf("minio: %w", err)
	}
	log.Info("minio connected", "endpoint", cfg.MinIO.Endpoint)

	leRepo := legalentity.NewRepository(db)
	leUC := legalentity.NewUsecase(leRepo, log)
	leHTTP := legalentity.NewHTTPHandler(leUC, log)
	leGRPC := legalentity.NewGRPCHandler(leUC, log)

	fileStorage := file.NewStorage(minioClient)
	fileRepo := file.NewRepository(db)
	fileUC := file.NewUsecase(fileRepo, fileStorage, log)
	fileHTTP := file.NewHTTPHandler(fileUC, log)

	kafkaConsumer := legalentity.NewKafkaConsumer(leUC, log)

	consumer, err := kafka.NewConsumer(cfg.Kafka, kafkaConsumer.HandleMessage, log)
	if err != nil {
		return fmt.Errorf("kafka consumer: %w", err)
	}
	defer func() {
		if closeErr := consumer.Close(); closeErr != nil {
			log.Error("kafka consumer close", "error", closeErr)
		}
	}()

	httpSrv := newHTTPServer(cfg, log, db, minioClient, leHTTP, fileHTTP)

	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			legalentity.LoggingInterceptor(log),
		),
	)
	pb.RegisterDocumentServiceServer(grpcSrv, leGRPC)

	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("grpc listen: %w", err)
	}

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
		log.Info("shutting down document-service...")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(), cfg.GracefulTimeout,
		)
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

	log.Info("document-service stopped gracefully")
	return nil
}

func newHTTPServer(
	cfg *config.Config,
	log *slog.Logger,
	db *postgres.DB,
	minioClient *mn.Client,
	leHTTP *legalentity.HTTPHandler,
	fileHTTP *file.HTTPHandler,
) *http.Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Validator = &requestValidator{v: validator.New()}

	e.Use(middleware.Recover())
	e.Use(echoprometheus.NewMiddleware("document_service"))

	healthHandler := health.New(map[string]health.Checker{
		"postgres": db,
		"minio":    minioClient,
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

	api := e.Group("/api/v1")
	leHTTP.Register(api)
	fileHTTP.Register(api)

	if cfg.Env == "development" {
		e.GET("/swagger/*", echoSwagger.WrapHandler)
	}

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      e,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second, // увеличен для upload файлов
		IdleTimeout:  60 * time.Second,
	}
}
