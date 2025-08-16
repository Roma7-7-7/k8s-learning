package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type API struct {
	Server   Server
	Database Database
	Redis    Redis
	Storage  Storage
	Logging  Logging
}

type Worker struct {
	Database          Database
	Redis             Redis
	Storage           Storage
	Logging           Logging
	WorkerID          string        `envconfig:"WORKER_ID"`
	ConcurrentJobs    int           `envconfig:"CONCURRENT_JOBS" default:"5"`
	HeartbeatInterval time.Duration `envconfig:"HEARTBEAT_INTERVAL" default:"30s"`
	PollInterval      time.Duration `envconfig:"POLL_INTERVAL" default:"5s"`
}

type Controller struct {
	Redis                     Redis
	Logging                   Logging
	ReconcileInterval         time.Duration `envconfig:"RECONCILE_INTERVAL" default:"30s"`
	MetricsCollectionInterval time.Duration `envconfig:"METRICS_COLLECTION_INTERVAL" default:"15s"`
}
type Server struct {
	Port            int           `envconfig:"PORT" default:"8080"`
	Host            string        `envconfig:"HOST" default:"0.0.0.0"`
	ReadTimeout     time.Duration `envconfig:"READ_TIMEOUT" default:"10s"`
	WriteTimeout    time.Duration `envconfig:"WRITE_TIMEOUT" default:"10s"`
	IdleTimeout     time.Duration `envconfig:"IDLE_TIMEOUT" default:"120s"`
	ShutdownTimeout time.Duration `envconfig:"SHUTDOWN_TIMEOUT" default:"30s"`
}

type Database struct {
	Host          string `envconfig:"DB_HOST" required:"true"`
	Port          int    `envconfig:"DB_PORT" default:"5432"`
	User          string `envconfig:"DB_USER" required:"true"`
	Password      string `envconfig:"DB_PASSWORD" required:"true"`
	Database      string `envconfig:"DB_NAME" required:"true"`
	SSLMode       string `envconfig:"DB_SSL_MODE" default:"require"`
	MaxConns      int    `envconfig:"DB_MAX_CONNS" default:"20"`
	MaxIdle       int    `envconfig:"DB_MAX_IDLE" default:"10"`
	MigrationsURL string `envconfig:"DB_MIGRATIONS_URL" default:"file://migrations"`
}

func (dc Database) ConnectionString() string {
	hostPort := net.JoinHostPort(dc.Host, strconv.Itoa(dc.Port))
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
		dc.User, dc.Password, hostPort, dc.Database, dc.SSLMode)
}

type Redis struct {
	Host     string `envconfig:"REDIS_HOST" required:"true"`
	Port     int    `envconfig:"REDIS_PORT" default:"6379"`
	Password string `envconfig:"REDIS_PASSWORD"`
	Database int    `envconfig:"REDIS_DB" default:"0"`
}

func (rc Redis) Address() string {
	return fmt.Sprintf("%s:%d", rc.Host, rc.Port)
}

type Storage struct {
	UploadDir   string `envconfig:"UPLOAD_DIR" required:"true"`
	ResultDir   string `envconfig:"RESULT_DIR" required:"true"`
	MaxFileSize int64  `envconfig:"MAX_FILE_SIZE" default:"10485760"` // 10MB
}

type Logging struct {
	Level  string `envconfig:"LOG_LEVEL" default:"info"`
	Format string `envconfig:"LOG_FORMAT" default:"json"`
}

func Load() (*API, error) {
	// Try to load .env file for local development (ignore if not found)
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(".env"); err != nil {
			return nil, fmt.Errorf("load .env file: %w", err)
		}
	}

	var config API

	if err := envconfig.Process("", &config); err != nil {
		return nil, fmt.Errorf("process environment variables: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

func LoadWorker() (*Worker, error) {
	// Try to load .env file for local development (ignore if not found)
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(".env"); err != nil {
			return nil, fmt.Errorf("load .env file: %w", err)
		}
	}

	var config Worker

	if err := envconfig.Process("", &config); err != nil {
		return nil, fmt.Errorf("process environment variables: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

func LoadController() (*Controller, error) {
	// Try to load .env file for local development (ignore if not found)
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(".env"); err != nil {
			return nil, fmt.Errorf("load .env file: %w", err)
		}
	}

	var config Controller

	if err := envconfig.Process("", &config); err != nil {
		return nil, fmt.Errorf("process environment variables: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

func (c *API) Validate() error {
	// Port validation
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// Database port validation
	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", c.Database.Port)
	}

	// Redis port validation
	if c.Redis.Port <= 0 || c.Redis.Port > 65535 {
		return fmt.Errorf("invalid redis port: %d", c.Redis.Port)
	}

	// Storage validation
	if c.Storage.MaxFileSize <= 0 {
		return errors.New("max file size must be positive")
	}

	// SSL mode validation
	validSSLModes := []string{"disable", "require", "verify-ca", "verify-full"}
	if !contains(validSSLModes, c.Database.SSLMode) {
		return fmt.Errorf("invalid SSL mode: %s", c.Database.SSLMode)
	}

	// Logging validation
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, c.Logging.Level) {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	validLogFormats := []string{"json", "text"}
	if !contains(validLogFormats, c.Logging.Format) {
		return fmt.Errorf("invalid log format: %s", c.Logging.Format)
	}

	return nil
}

func (w *Worker) Validate() error {
	// Database port validation
	if w.Database.Port <= 0 || w.Database.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", w.Database.Port)
	}

	// Redis port validation
	if w.Redis.Port <= 0 || w.Redis.Port > 65535 {
		return fmt.Errorf("invalid redis port: %d", w.Redis.Port)
	}

	// Storage validation
	if w.Storage.MaxFileSize <= 0 {
		return errors.New("max file size must be positive")
	}

	// Worker validation
	if w.ConcurrentJobs <= 0 {
		return errors.New("concurrent jobs must be positive")
	}

	if w.HeartbeatInterval <= 0 {
		return errors.New("heartbeat interval must be positive")
	}

	if w.PollInterval <= 0 {
		return errors.New("poll interval must be positive")
	}

	// SSL mode validation
	validSSLModes := []string{"disable", "require", "verify-ca", "verify-full"}
	if !contains(validSSLModes, w.Database.SSLMode) {
		return fmt.Errorf("invalid SSL mode: %s", w.Database.SSLMode)
	}

	// Logging validation
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, w.Logging.Level) {
		return fmt.Errorf("invalid log level: %s", w.Logging.Level)
	}

	validLogFormats := []string{"json", "text"}
	if !contains(validLogFormats, w.Logging.Format) {
		return fmt.Errorf("invalid log format: %s", w.Logging.Format)
	}

	return nil
}

func (c *Controller) Validate() error {
	// Redis port validation
	if c.Redis.Port <= 0 || c.Redis.Port > 65535 {
		return fmt.Errorf("invalid redis port: %d", c.Redis.Port)
	}

	// Controller validation
	if c.ReconcileInterval <= 0 {
		return errors.New("reconcile interval must be positive")
	}

	if c.MetricsCollectionInterval <= 0 {
		return errors.New("metrics collection interval must be positive")
	}

	// Logging validation
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, c.Logging.Level) {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	validLogFormats := []string{"json", "text"}
	if !contains(validLogFormats, c.Logging.Format) {
		return fmt.Errorf("invalid log format: %s", c.Logging.Format)
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
