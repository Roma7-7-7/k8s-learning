package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/rsav/k8s-learning/internal/config"
	"github.com/rsav/k8s-learning/internal/storage/database"
	"github.com/rsav/k8s-learning/internal/storage/queue"
	"github.com/rsav/k8s-learning/internal/worker"
	"github.com/rsav/k8s-learning/internal/worker/metrics"
)

func main() {
	cfg, err := config.LoadWorker()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err) //nolint:sloglint // we did not initialize the logger yet
		os.Exit(1)
	}

	os.Exit(runWithShutdown(cfg))
}

func runWithShutdown(cfg *config.Worker) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := setupLogger(cfg.Logging)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.InfoContext(ctx, "received shutdown signal")
		cancel()
	}()

	return run(ctx, cfg, log)
}

func run(ctx context.Context, cfg *config.Worker, log *slog.Logger) int {
	log.InfoContext(ctx, "starting worker", "worker_id", cfg.WorkerID)

	// Set worker info metric
	metrics.WorkerInfo.WithLabelValues(cfg.WorkerID, "1.0.0").Set(1)

	repo, err := database.NewRepository(cfg.Database, log)
	if err != nil {
		log.ErrorContext(ctx, "failed to initialize database", "error", err)
		return 1
	}
	defer func() {
		if err := repo.Close(); err != nil {
			log.ErrorContext(ctx, "failed to close database connection", "error", err)
		}
	}()

	redisQueue, err := queue.NewRedisQueue(cfg.Redis, log)
	if err != nil {
		log.ErrorContext(ctx, "failed to initialize Redis queue", "error", err)
		return 1
	}
	defer func() {
		if err := redisQueue.Close(); err != nil {
			log.ErrorContext(ctx, "failed to close Redis connection", "error", err)
		}
	}()

	w, err := worker.New(cfg, repo, redisQueue, log)
	if err != nil {
		log.ErrorContext(ctx, "failed to create worker", "error", err)
		return 1
	}

	// Start metrics and health server
	var wg sync.WaitGroup
	metricsServer := startMetricsServer(ctx, cfg.MetricsPort, log, &wg, repo, redisQueue)

	log.InfoContext(ctx, "worker starting...")
	if err := w.Start(ctx); err != nil {
		log.ErrorContext(ctx, "worker failed", "error", err)
		shutdownMetricsServer(metricsServer, log)
		wg.Wait()
		return 1
	}

	// Shutdown metrics server
	shutdownMetricsServer(metricsServer, log)
	wg.Wait()

	log.InfoContext(ctx, "worker shutdown complete")
	return 0
}

func startMetricsServer(ctx context.Context, port int, log *slog.Logger, wg *sync.WaitGroup, repo *database.Repository, queue *queue.RedisQueue) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	// Health endpoints
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		allHealthy := true

		// Check database connectivity
		if err := repo.HealthCheck(r.Context()); err != nil {
			log.ErrorContext(r.Context(), "database health check failed", "error", err)
			allHealthy = false
		}

		// Check Redis connectivity
		if err := queue.HealthCheck(r.Context()); err != nil {
			log.ErrorContext(r.Context(), "redis health check failed", "error", err)
			allHealthy = false
		}

		if allHealthy {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("NOT READY"))
		}
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second, //nolint:mnd // reasonable timeout for metrics endpoint
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.InfoContext(ctx, "starting metrics and health server", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.ErrorContext(ctx, "metrics server error", "error", err)
		}
	}()

	return server
}

func shutdownMetricsServer(server *http.Server, log *slog.Logger) {
	const shutdownTimeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.ErrorContext(ctx, "metrics server shutdown error", "error", err)
	}
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
