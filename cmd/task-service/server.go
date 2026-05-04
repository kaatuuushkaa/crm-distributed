package main

import (
	"crm-distributed/cmd/task-service/config"
	internaljwt "crm-distributed/cmd/task-service/internal/jwt"
	internalmiddleware "crm-distributed/cmd/task-service/internal/middleware"
	"crm-distributed/cmd/task-service/internal/task"
	"crm-distributed/shared/pkg/health"
	"crm-distributed/shared/pkg/postgres"
	"crm-distributed/shared/pkg/redis"
	"github.com/labstack/echo/v4"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4/middleware"
)

func server(
	cfg *config.Config,
	log *slog.Logger,
	db *postgres.DB,
	rdb *redis.Client,
) *http.Server {
	e := echo.New()

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
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut,
			http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderContentType, echo.HeaderAuthorization},
	}))

	e.Use(echoprometheus.NewMiddleware("task_service"))

	e.Use(slogMiddleware(log))

	h := health.New(map[string]health.Checker{
		"postgres": db,
		"redis":    rdb,
	})

	e.GET("/healthz", func(c echo.Context) error {
		h.Liveness(c.Response(), c.Request())
		return nil
	})

	e.GET("/readyz", func(c echo.Context) error {
		h.Readiness(c.Response(), c.Request())
		return nil
	})

	e.GET("/metrics", echoprometheus.NewHandler())
	jwtService := internaljwt.New(cfg.JWTSecret)
	api := e.Group("/api/v1")
	protected := api.Group("", internalmiddleware.Auth(jwtService))
	// TODO: хэндлеры по мере их написания:
	// registerTaskRoutes(api, taskHandler)
	// registerProjectRoutes(api, projectHandler)
	// registerUserRoutes(api, userHandler)
	taskRepo := task.NewRepository(db, log)
	taskUsecase := task.NewUsecase(taskRepo, log)
	taskHandler := task.NewHandler(taskUsecase)
	taskHandler.Register(protected)

	return &http.Server{
		Addr:    cfg.Addr(),
		Handler: e,

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
