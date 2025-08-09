package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rsav/k8s-learning/internal/config"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(conf config.Database, logger *slog.Logger) (*Repository, error) {
	ctx := context.Background()

	logger.InfoContext(ctx, "connecting to PostgreSQL database", "host", conf.Host, "port", conf.Port, "database", conf.Database)

	db, err := sqlx.Connect("pgx", conf.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	db.SetMaxOpenConns(conf.MaxConns)
	db.SetMaxIdleConns(conf.MaxIdle)
	db.SetConnMaxLifetime(time.Hour)

	logger.DebugContext(ctx, "connection pool configured", "max_conns", conf.MaxConns, "max_idle", conf.MaxIdle)

	logger.InfoContext(ctx, "running database migrations")
	if err := RunMigrations(conf.ConnectionString(), logger); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Repository{
		db: db,
	}, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second) //nolint: mnd // Use a short timeout for health check
	defer cancel()

	return r.db.PingContext(ctx)
}
