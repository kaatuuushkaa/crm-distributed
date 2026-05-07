package main

import (
	"crm-distributed/cmd/task-service/internal/federation"
	"crm-distributed/cmd/task-service/internal/project"
	"github.com/go-playground/validator/v10"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"crm-distributed/cmd/task-service/config"
	"crm-distributed/cmd/task-service/internal/auth"
	internaljwt "crm-distributed/cmd/task-service/internal/jwt"
	internalmiddleware "crm-distributed/cmd/task-service/internal/middleware"
	"crm-distributed/cmd/task-service/internal/task"
	"crm-distributed/shared/pkg/health"
	"crm-distributed/shared/pkg/kafka"
	"crm-distributed/shared/pkg/postgres"
	"crm-distributed/shared/pkg/redis"
)

func Server(
	cfg *config.Config,
	log *slog.Logger,
	db *postgres.DB,
	rdb *redis.Client,
	kafkaProducer *kafka.Producer,
) *http.Server {
	e := echo.New()
	e.Validator = &requestValidator{v: validator.New()}
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			log.Error("panic recovered",
				"error", err,
				"stack", string(stack),
				"path", c.Path(),
				"method", c.Request().Method,
			)
			return nil
		},
	}))
	e.Use(middleware.RequestID())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			http.MethodGet, http.MethodPost, http.MethodPut,
			http.MethodPatch, http.MethodDelete, http.MethodOptions,
		},
		AllowHeaders: []string{echo.HeaderContentType, echo.HeaderAuthorization},
	}))
	e.Use(echoprometheus.NewMiddleware("task_service"))
	e.Use(slogMiddleware(log))

	healthHandler := health.New(map[string]health.Checker{
		"postgres": db,
		"redis":    rdb,
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

	jwtService := internaljwt.New(cfg.JWTSecret)

	api := e.Group("/api/v1")
	protected := api.Group("", internalmiddleware.Auth(jwtService))

	authRepo := auth.NewUserRepository(db, log)
	authUsecase := auth.NewUsecase(authRepo, jwtService, rdb, log, cfg.Env != "production")
	authHandler := auth.NewHandler(authUsecase)
	authHandler.Register(api)

	taskRepo := task.NewRepository(db, log)
	taskUsecase := task.NewUsecase(taskRepo, kafkaProducer, log)
	taskHandler := task.NewHandler(taskUsecase)
	taskHandler.Register(protected)

	projectRepo := project.NewRepository(db, log)
	projectUsecase := project.NewUsecase(projectRepo, kafkaProducer, log)
	projectHandler := project.NewHandler(projectUsecase)
	projectHandler.Register(protected)

	federationRepo := federation.NewRepository(db, log)
	federationUsecase := federation.NewUsecase(federationRepo, kafkaProducer, log)
	federationHandler := federation.NewHandler(federationUsecase)
	federationHandler.Register(protected)

	return &http.Server{
		Addr:         cfg.Addr(),
		Handler:      e,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func slogMiddleware(log *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			log.InfoContext(c.Request().Context(), "http request",
				"method", c.Request().Method,
				"path", c.Request().URL.Path,
				"status", c.Response().Status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", c.Response().Header().Get(echo.HeaderXRequestID),
				"ip", c.RealIP(),
			)

			return err
		}
	}
}

type requestValidator struct {
	v *validator.Validate
}

func (rv *requestValidator) Validate(i any) error {
	if err := rv.v.Struct(i); err != nil {
		return err
	}
	return nil
}
