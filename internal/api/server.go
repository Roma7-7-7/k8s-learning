package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/rsav/k8s-learning/internal/api/handlers"
	"github.com/rsav/k8s-learning/internal/api/middleware"
	"github.com/rsav/k8s-learning/internal/config"
	"github.com/rsav/k8s-learning/internal/storage/database"
	"github.com/rsav/k8s-learning/internal/storage/filestore"
	"github.com/rsav/k8s-learning/internal/storage/queue"
)

type Server struct {
	config     *config.API
	repo       *database.Repository
	queue      *queue.RedisQueue
	fileStore  *filestore.FileStore
	log        *slog.Logger
	httpServer *http.Server
	// Atomic flag to indicate if server is shutting down
	// 0 = running, 1 = shutting down
	shuttingDown int32
}

func NewServer(cfg *config.API, log *slog.Logger) (*Server, error) {
	ctx := context.Background()

	log.DebugContext(ctx, "Initializing database connection")
	repo, err := database.NewRepository(cfg.Database, log)
	if err != nil {
		return nil, fmt.Errorf("initialize database: %w", err)
	}

	log.DebugContext(ctx, "Initializing Redis queue connection")
	q, err := queue.NewRedisQueue(cfg.Redis, log)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("initialize Redis queue: %w", err)
	}

	log.DebugContext(ctx, "Initializing file store",
		"upload_dir", cfg.Storage.UploadDir, "result_dir", cfg.Storage.ResultDir, "max_file_size", cfg.Storage.MaxFileSize)
	fileStore, err := filestore.NewFileStore(
		cfg.Storage.UploadDir,
		cfg.Storage.ResultDir,
		cfg.Storage.MaxFileSize,
	)
	if err != nil {
		_ = repo.Close()
		_ = q.Close()
		return nil, fmt.Errorf("initialize file store: %w", err)
	}

	server := &Server{
		config:    cfg,
		repo:      repo,
		queue:     q,
		fileStore: fileStore,
		log:       log,
	}

	server.setupRoutes()

	return server, nil
}

func (s *Server) setupRoutes() {
	mux := http.NewServeMux()

	jobHandler := handlers.NewJob(s.repo, s.queue, s.fileStore, s.log)
	healthHandler := handlers.NewHealth(s.repo, s.queue, s.log)

	mux.HandleFunc("/api/v1/jobs", jobHandler.CreateJob)
	mux.HandleFunc("/api/v1/jobs/", s.routeJobRequests(jobHandler))
	mux.HandleFunc("/health", healthHandler.Health)
	mux.HandleFunc("/ready", healthHandler.Ready)
	mux.HandleFunc("/stats", healthHandler.Stats)

	middlewareChain := middleware.Chain(
		middleware.RecoveryMiddleware(s.log),
		middleware.RequestIDMiddleware(),
		middleware.LoggingMiddleware(s.log),
		middleware.CORSMiddleware(),
		middleware.SecurityHeadersMiddleware(),
		middleware.MaxRequestSizeMiddleware(s.config.Storage.MaxFileSize),
	)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port),
		Handler:      middlewareChain(mux),
		ReadTimeout:  s.config.Server.ReadTimeout,
		WriteTimeout: s.config.Server.WriteTimeout,
		IdleTimeout:  s.config.Server.IdleTimeout,
	}
}

func (s *Server) routeJobRequests(jobHandler *handlers.Job) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/api/v1/jobs/" {
			jobHandler.ListJobs(w, r)
			return
		}

		if strings.HasSuffix(path, "/result") {
			jobHandler.GetJobResult(w, r)
			return
		}

		jobHandler.GetJob(w, r)
	}
}

func (s *Server) Start(ctx context.Context) error {
	s.log.InfoContext(ctx, "starting server",
		"address", s.httpServer.Addr,
		"upload_dir", s.config.Storage.UploadDir,
		"result_dir", s.config.Storage.ResultDir,
		"max_file_size", s.config.Storage.MaxFileSize,
	)

	errCh := make(chan error, 1)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("server listen failed: %w", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	// Listen for termination signals from Kubernetes and system
	// SIGTERM: Standard termination signal from Kubernetes during pod shutdown
	// SIGINT: Interrupt signal (Ctrl+C) for local development
	// SIGQUIT: Quit signal for emergency shutdown
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case err := <-errCh:
		return err
	case sig := <-sigCh:
		s.log.InfoContext(ctx, "received shutdown signal", "signal", sig.String())
		return s.shutdown(ctx)
	case <-ctx.Done():
		s.log.InfoContext(ctx, "context cancelled, shutting down")
		return s.shutdown(ctx)
	}
}

func (s *Server) shutdown(ctx context.Context) error {
	// Signal that we're shutting down
	atomic.StoreInt32(&s.shuttingDown, 1)

	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.Server.ShutdownTimeout)
	defer cancel()

	s.log.InfoContext(shutdownCtx, "initiating graceful shutdown",
		"timeout", s.config.Server.ShutdownTimeout.String())

	// Step 1: Stop accepting new HTTP requests and close existing connections
	s.log.InfoContext(shutdownCtx, "stopping HTTP server...")
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.log.ErrorContext(shutdownCtx, "HTTP server shutdown failed", "error", err)
		// Don't return immediately, try to cleanup other resources
	} else {
		s.log.InfoContext(shutdownCtx, "HTTP server stopped successfully")
	}

	// Step 2: Close Redis queue connection
	if s.queue != nil {
		s.log.InfoContext(shutdownCtx, "closing Redis connection...")
		if err := s.queue.Close(); err != nil {
			s.log.ErrorContext(shutdownCtx, "failed to close Redis connection", "error", err)
		} else {
			s.log.InfoContext(shutdownCtx, "Redis connection closed successfully")
		}
	}

	// Step 3: Close database connections
	if s.repo != nil {
		s.log.InfoContext(shutdownCtx, "closing database connections...")
		if err := s.repo.Close(); err != nil {
			s.log.ErrorContext(shutdownCtx, "failed to close database connection", "error", err)
		} else {
			s.log.InfoContext(shutdownCtx, "database connections closed successfully")
		}
	}

	s.log.InfoContext(shutdownCtx, "graceful shutdown completed")
	return nil
}

func (s *Server) HealthCheck(ctx context.Context) error {
	if err := s.repo.HealthCheck(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	if err := s.queue.HealthCheck(ctx); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	return nil
}
