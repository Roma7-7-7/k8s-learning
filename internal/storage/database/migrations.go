package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	pgxv5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
)

func RunMigrations(db *sqlx.DB, databaseURL string, logger *slog.Logger) error {
	ctx := context.Background()
	
	logger.DebugContext(ctx, "creating migration driver instance")
	driver, err := pgxv5.WithInstance(db.DB, &pgxv5.Config{})
	if err != nil {
		return fmt.Errorf("failed to create pgx driver: %w", err)
	}

	logger.DebugContext(ctx, "opening migration files from migrations directory")
	sourceDriver, err := (&file.File{}).Open("file://migrations")
	if err != nil {
		return fmt.Errorf("failed to open migrations source: %w", err)
	}

	logger.DebugContext(ctx, "creating migration instance")
	m, err := migrate.NewWithInstance("file", sourceDriver, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	logger.DebugContext(ctx, "running pending migrations")
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		logger.InfoContext(ctx, "no new migrations to apply")
	} else {
		logger.InfoContext(ctx, "migrations completed successfully")
	}

	return nil
}
