package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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
	db            *sql.DB
	log           *slog.Logger
	queryDuration *prometheus.HistogramVec
}

func New(cfg Config, log *slog.Logger) (*DB, error) {
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(10 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	queryDuration := promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "postgres_query_duration_seconds",
			Help:    "Длительность выполнения SQL запросов в секундах.",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		},
		[]string{"operation"},
	)

	return &DB{
		db:            db,
		log:           log,
		queryDuration: queryDuration,
	}, nil
}

func (d *DB) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) QueryContext(ctx context.Context, operation, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()

	rows, err := d.db.QueryContext(ctx, query, args...)

	d.observe(ctx, operation, time.Since(start), err)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}

	return rows, nil
}

func (d *DB) QueryRowContext(ctx context.Context, operation, query string, args ...any) *sql.Row {
	start := time.Now()

	row := d.db.QueryRowContext(ctx, query, args...)

	d.observe(ctx, operation, time.Since(start), nil)

	return row
}

func (d *DB) ExecContext(ctx context.Context, operation, query string, args ...any) (sql.Result, error) {
	start := time.Now()

	result, err := d.db.ExecContext(ctx, query, args...)

	d.observe(ctx, operation, time.Since(start), err)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}

	return result, nil
}

func (d *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	tx, err := d.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	return tx, nil
}

func (d *DB) observe(ctx context.Context, operation string, elapsed time.Duration, err error) {
	d.queryDuration.WithLabelValues(operation).Observe(elapsed.Seconds())

	const slowThreshold = 200 * time.Millisecond

	switch {
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		d.log.ErrorContext(ctx, "postgres query error",
			"operation", operation,
			"elapsed_ms", elapsed.Milliseconds(),
			"error", err,
		)
	case elapsed > slowThreshold:
		d.log.WarnContext(ctx, "postgres slow query",
			"operation", operation,
			"elapsed_ms", elapsed.Milliseconds(),
			"threshold_ms", slowThreshold.Milliseconds(),
		)
	default:
		d.log.DebugContext(ctx, "postgres query",
			"operation", operation,
			"elapsed_ms", elapsed.Milliseconds(),
		)
	}
}
