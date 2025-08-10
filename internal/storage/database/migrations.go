package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	pgxv5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
)

func RunMigrations(connStr string, log *slog.Logger) error {
	ctx := context.Background()

	log.DebugContext(ctx, "creating separate database connection for migrations")
	migrationDB, err := sqlx.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("open migration database connection: %w", err)
	}
	defer migrationDB.Close()

	log.DebugContext(ctx, "creating migration driver instance")
	driver, err := pgxv5.WithInstance(migrationDB.DB, &pgxv5.Config{})
	if err != nil {
		return fmt.Errorf("create pgx driver: %w", err)
	}

	log.DebugContext(ctx, "opening migration files from migrations directory")
	sourceDriver, err := (&file.File{}).Open("file://migrations")
	if err != nil {
		return fmt.Errorf("open migrations source: %w", err)
	}

	log.DebugContext(ctx, "creating migration instance")
	m, err := migrate.NewWithInstance("file", sourceDriver, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer m.Close()

	log.DebugContext(ctx, "running pending migrations")
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		log.InfoContext(ctx, "no new migrations to apply")
	} else {
		log.InfoContext(ctx, "migrations completed successfully")
	}

	return nil
}
