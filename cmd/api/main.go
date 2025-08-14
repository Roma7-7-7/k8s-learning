package main

import (
	"context"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rsav/k8s-learning/internal/api"
	"github.com/rsav/k8s-learning/internal/config"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err) //nolint:sloglint // we did not initialize the logger yet
		os.Exit(1)
	}

	log := setupLogger(cfg.Logging.Level, cfg.Logging.Format)
	slog.SetDefault(log)

	log.InfoContext(ctx, "Starting text processing API service")

	server, err := api.NewServer(cfg, log)
	if err != nil {
		log.ErrorContext(ctx, "Failed to create server", "error", err)
		os.Exit(1)
	}

	if err := server.Start(ctx); err != nil {
		log.ErrorContext(ctx, "Server failed", "error", err)
	}
}

func setupLogger(level, format string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
