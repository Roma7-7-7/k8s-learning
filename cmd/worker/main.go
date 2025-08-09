package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rsav/k8s-learning/internal/config"
	"github.com/rsav/k8s-learning/internal/storage/database"
	"github.com/rsav/k8s-learning/internal/storage/queue"
	"github.com/rsav/k8s-learning/internal/worker"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.LoadWorker()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err) //nolint:sloglint // we did not initialize the logger yet
		os.Exit(1)
	}

	logger := setupLogger(cfg.Logging)
	logger.InfoContext(ctx, "starting worker", "worker_id", cfg.WorkerID)

	repo, err := database.NewRepository(cfg.Database, logger)
	if err != nil {
		logger.ErrorContext(ctx, "failed to initialize database", "error", err)
		cancel()
		os.Exit(1)
	}
	defer func() {
		if err := repo.Close(); err != nil {
			logger.ErrorContext(ctx, "failed to close database connection", "error", err)
		}
	}()

	redisQueue, err := queue.NewRedisQueue(cfg.Redis, logger)
	if err != nil {
		logger.ErrorContext(ctx, "failed to initialize Redis queue", "error", err)
		cancel()
		os.Exit(1)
	}
	defer func() {
		if err := redisQueue.Close(); err != nil {
			logger.ErrorContext(ctx, "failed to close Redis connection", "error", err)
		}
	}()

	w, err := worker.New(cfg, repo, redisQueue, logger)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create worker", "error", err)
		cancel()
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.InfoContext(ctx, "received shutdown signal")
		cancel()
	}()

	logger.InfoContext(ctx, "worker starting...")
	if err := w.Start(ctx); err != nil {
		logger.ErrorContext(ctx, "worker failed", "error", err)
		cancel()
		os.Exit(1)
	}

	logger.InfoContext(ctx, "worker shutdown complete")
}

func setupLogger(config config.Logging) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: parseLogLevel(config.Level),
	}

	switch config.Format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
