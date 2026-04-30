// Package postgres provides a GORM-based PostgreSQL client with structured
// logging via slog, Prometheus metrics, and context-aware helpers.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
	"gorm.io/plugin/prometheus"
)

type Config struct {
	Host     string `env:"POSTGRES_HOST,required"`
	Port     int    `env:"POSTGRES_PORT"         envDefault:"5432"`
	User     string `env:"POSTGRES_USER,required"`
	Password string `env:"POSTGRES_PASSWORD,required"`
	DBName   string `env:"POSTGRES_DB,required"`
	SSLMode  string `env:"POSTGRES_SSLMODE"      envDefault:"disable"`
}

func (c Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

type DB struct {
	*gorm.DB
}

func New(cfg Config, enableMetrics bool) (*DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		TranslateError:  true, // gorm.ErrRecordNotFound вместо raw pg ошибки
		CreateBatchSize: 1000,
		Logger:          newSlogAdapter(),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	if enableMetrics {
		if err = db.Use(prometheus.New(prometheus.Config{
			DBName:          cfg.DBName,
			RefreshInterval: 15, // секунды между обновлением метрик
		})); err != nil {
			return nil, fmt.Errorf("register prometheus plugin: %w", err)
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return &DB{db}, nil
}

func (d *DB) Ping(ctx context.Context) error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}

	return sqlDB.PingContext(ctx)
}

func (d *DB) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}

	return sqlDB.Close()
}

type slogAdapter struct {
	slowThreshold time.Duration
}

func newSlogAdapter() gormlogger.Interface {
	return &slogAdapter{
		slowThreshold: 200 * time.Millisecond,
	}
}

func (a *slogAdapter) LogMode(_ gormlogger.LogLevel) gormlogger.Interface {
	return a
}

func (a *slogAdapter) Info(ctx context.Context, msg string, args ...any) {
	slog.InfoContext(ctx, fmt.Sprintf(msg, args...))
}

func (a *slogAdapter) Warn(ctx context.Context, msg string, args ...any) {
	slog.WarnContext(ctx, fmt.Sprintf(msg, args...))
}

func (a *slogAdapter) Error(ctx context.Context, msg string, args ...any) {
	slog.ErrorContext(ctx, fmt.Sprintf(msg, args...))
}

func (a *slogAdapter) Trace(
	ctx context.Context,
	begin time.Time,
	fc func() (sql string, rowsAffected int64),
	err error,
) {
	elapsed := time.Since(begin)
	sql, rows := fc()

	attrs := []any{
		"elapsed_ms", elapsed.Milliseconds(),
		"rows", rows,
		"sql", sql,
		"caller", utils.FileWithLineNum(),
	}

	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		slog.ErrorContext(ctx, "gorm query error",
			append(attrs, "error", err)...)
	case elapsed > a.slowThreshold:
		slog.WarnContext(ctx, "gorm slow query",
			append(attrs, "threshold_ms", a.slowThreshold.Milliseconds())...)
	default:
		slog.DebugContext(ctx, "gorm query", attrs...)
	}
}
