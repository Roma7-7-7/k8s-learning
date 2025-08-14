package database

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rsav/k8s-learning/internal/config"
)

type Repository struct {
	db *sqlx.DB
}

// JSONB handles PostgreSQL JSONB columns by implementing sql.Scanner and driver.Valuer.
type JSONB map[string]any

func NewRepository(conf config.Database, log *slog.Logger) (*Repository, error) {
	ctx := context.Background()

	log.InfoContext(ctx, "connecting to PostgreSQL database", "host", conf.Host, "port", conf.Port, "database", conf.Database)

	db, err := sqlx.Connect("pgx", conf.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	db.SetMaxOpenConns(conf.MaxConns)
	db.SetMaxIdleConns(conf.MaxIdle)
	db.SetConnMaxLifetime(time.Hour)

	log.DebugContext(ctx, "connection pool configured", "max_conns", conf.MaxConns, "max_idle", conf.MaxIdle)

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

// Scan implements the sql.Scanner interface for JSONB.
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONB)
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into JSONB", value)
	}

	var result map[string]any
	if err := json.Unmarshal(bytes, &result); err != nil {
		return fmt.Errorf("cannot unmarshal JSONB: %w", err)
	}

	*j = result
	return nil
}

// Value implements the driver.Valuer interface for JSONB.
func (j *JSONB) Value() (driver.Value, error) {
	if j == nil {
		return []byte("{}"), nil
	}

	return json.Marshal(*j)
}
