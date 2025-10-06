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
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string

	// Setup signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Load configuration
	cfg, err := config.LoadController()
	if err != nil {
		setupLog.Error(err, "unable to load controller configuration")
		os.Exit(1)
	}

	// Note: We accept command line flags for compatibility with deployment
	// but don't use them since we've simplified the controller

	// Setup structured logger
	log := setupLogger(cfg.Logging)
	log.InfoContext(ctx, "starting text processing controller",
		"version", "v1alpha1",
		"metrics_addr", metricsAddr,
		"health_addr", probeAddr,
		"leader_election", enableLeaderElection,
		"reconcile_interval", cfg.ReconcileInterval)

	// Connect to Redis
	redisQueue, err := queue.NewRedisQueue(cfg.Redis, log)
	if err != nil {
		log.ErrorContext(ctx, "failed to connect to Redis", "error", err)
		os.Exit(1)
	} else {
		log.InfoContext(ctx, "Redis connection established for queue monitoring")
	}

	// Setup Kubernetes client directly (no controller-runtime manager needed)
	k8sConfig := ctrl.GetConfigOrDie()
	k8sClient, err := client.New(k8sConfig, client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to create Kubernetes client")
		os.Exit(1)
	}

	// Setup worker scaler with direct client
	workerScaler := &scaler.Worker{
		Client: k8sClient,
		Log:    log,
		Queue:  redisQueue,
		Config: *cfg,
	}

	metricsCollector := metrics.NewMetricsCollector(redisQueue, log)
	go metricsCollector.StartPeriodicCollection(ctx, cfg.MetricsCollectionInterval)

	// Start metrics server
	metricsServer := &http.Server{
		Addr:    metricsAddr,
		Handler: promhttp.Handler(),
	}
	go func() {
		log.InfoContext(ctx, "starting metrics server", "addr", metricsAddr)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			setupLog.Error(err, "metrics server failed")
		}
	}()

	// Start health check server
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	healthMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	healthServer := &http.Server{
		Addr:    probeAddr,
		Handler: healthMux,
	}
	go func() {
		log.InfoContext(ctx, "starting health server", "addr", probeAddr)
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			setupLog.Error(err, "health server failed")
		}
	}()

	// Gracefully shutdown servers on context cancellation
	go func() {
		<-ctx.Done()
		log.InfoContext(context.Background(), "shutting down servers")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := metricsServer.Shutdown(shutdownCtx); err != nil {
			setupLog.Error(err, "metrics server shutdown failed")
		}
		if err := healthServer.Shutdown(shutdownCtx); err != nil {
			setupLog.Error(err, "health server shutdown failed")
		}
	}()

	// Start timer-based reconciliation (blocking call)
	setupLog.Info("starting worker scaler")
	workerScaler.StartPeriodicScaling(ctx)
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
