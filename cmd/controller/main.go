package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/rsav/k8s-learning/internal/config"
	"github.com/rsav/k8s-learning/internal/controller/metrics"
	"github.com/rsav/k8s-learning/internal/controller/scaler"
	"github.com/rsav/k8s-learning/internal/storage/queue"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func main() {
	// Setup signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Parse flags and setup logger
	serverAddr, enableLeaderElection := parseFlags()

	// Load configuration
	cfg := loadConfig()

	// Setup structured logger
	log := setupLogger(cfg.Logging)
	log.InfoContext(ctx, "starting text processing controller",
		"version", "v1alpha1",
		"server_addr", serverAddr,
		"leader_election", enableLeaderElection,
		"reconcile_interval", cfg.ReconcileInterval)

	// Initialize components
	redisQueue := initRedis(ctx, cfg, log)
	k8sClient := initKubernetesClient()
	workerScaler := createWorkerScaler(k8sClient, log, redisQueue, cfg)

	// Start metrics collection
	metricsCollector := metrics.NewMetricsCollector(redisQueue, log)
	go metricsCollector.StartPeriodicCollection(ctx, cfg.MetricsCollectionInterval)

	// Start server (metrics + health endpoints)
	server := startServer(ctx, serverAddr, log, redisQueue)

	// Setup graceful shutdown
	setupGracefulShutdown(ctx, log, server)

	// Start worker scaler (blocking)
	setupLog.Info("starting worker scaler")
	workerScaler.StartPeriodicScaling(ctx)
}

func parseFlags() (string, bool) {
	var serverAddr string
	var enableLeaderElection bool

	flag.StringVar(&serverAddr, "bind-address", ":8080", "The address the server endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager.")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	return serverAddr, enableLeaderElection
}

func loadConfig() *config.Controller {
	cfg, err := config.LoadController()
	if err != nil {
		setupLog.Error(err, "unable to load controller configuration")
		os.Exit(1)
	}
	return cfg
}

func initRedis(ctx context.Context, cfg *config.Controller, log *slog.Logger) *queue.RedisQueue {
	redisQueue, err := queue.NewRedisQueue(cfg.Redis, log)
	if err != nil {
		log.ErrorContext(ctx, "failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	log.InfoContext(ctx, "Redis connection established for queue monitoring")
	return redisQueue
}

func initKubernetesClient() client.Client {
	k8sConfig := ctrl.GetConfigOrDie()
	k8sClient, err := client.New(k8sConfig, client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to create Kubernetes client")
		os.Exit(1)
	}
	return k8sClient
}

func createWorkerScaler(k8sClient client.Client, log *slog.Logger, redisQueue *queue.RedisQueue, cfg *config.Controller) *scaler.Worker {
	return &scaler.Worker{
		Client: k8sClient,
		Log:    log,
		Queue:  redisQueue,
		Config: *cfg,
	}
}

func startServer(ctx context.Context, addr string, log *slog.Logger, redisQueue *queue.RedisQueue) *http.Server {
	mux := http.NewServeMux()

	// Prometheus metrics
	mux.Handle("/metrics", promhttp.Handler())

	// Liveness check - basic check that process is running
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Alias for livez
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Readiness check - verify Redis connectivity
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := redisQueue.HealthCheck(r.Context()); err != nil {
			log.ErrorContext(r.Context(), "redis health check failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("NOT READY"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: httpReadHeaderTimeout,
	}
	go func() {
		log.InfoContext(ctx, "starting server", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			setupLog.Error(err, "server failed")
		}
	}()
	return server
}

const (
	shutdownTimeout       = 30 * time.Second
	httpReadHeaderTimeout = 5 * time.Second
)

func setupGracefulShutdown(ctx context.Context, log *slog.Logger, server *http.Server) {
	go func() {
		<-ctx.Done()
		log.InfoContext(context.Background(), "shutting down server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			setupLog.Error(err, "server shutdown failed")
		}
	}()
}

func setupLogger(config config.Logging) *slog.Logger {
	var level slog.Level
	switch config.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if config.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
