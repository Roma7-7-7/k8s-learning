package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rsav/k8s-learning/internal/config"
)

type DB struct {
	*sqlx.DB
	jobStore *JobStore
}

func NewDB(config config.DatabaseConfig, logger *slog.Logger) (*DB, error) {
	ctx := context.Background()
	
	logger.InfoContext(ctx, "connecting to PostgreSQL database", "host", config.Host, "port", config.Port, "database", config.Database)
	
	db, err := sqlx.Connect("pgx", config.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(config.MaxConns)
	db.SetMaxIdleConns(config.MaxIdle)
	db.SetConnMaxLifetime(time.Hour)

	logger.DebugContext(ctx, "connection pool configured", "max_conns", config.MaxConns, "max_idle", config.MaxIdle)

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger.DebugContext(pingCtx, "pinging database connection")
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.InfoContext(ctx, "running database migrations")
	if err := RunMigrations(db, config.ConnectionString(), logger); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DB{
		DB:       db,
		jobStore: NewJobStore(db),
	}, nil
}

// Jobs returns the job repository
func (db *DB) Jobs() *JobStore {
	return db.jobStore
}

func (db *DB) Close() error {
	return db.DB.Close()
}

func (db *DB) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return db.PingContext(ctx)
}
