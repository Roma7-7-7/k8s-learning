package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/rsav/k8s-learning/internal/config"
	"github.com/rsav/k8s-learning/internal/controller/metrics"
	"github.com/rsav/k8s-learning/internal/controller/workerscaler"
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

	// Override config with command line flags if provided
	if metricsAddr != ":8080" {
		cfg.MetricsAddr = metricsAddr
	}
	cfg.EnableLeaderElection = enableLeaderElection

	// Setup structured logger
	log := setupLogger(cfg.Logging)
	log.InfoContext(context.Background(), "starting text processing controller",
		"version", "v1alpha1",
		"metrics_addr", cfg.MetricsAddr,
		"leader_election", cfg.EnableLeaderElection,
		"auto_scaling", cfg.EnableAutoScaling)

	// Setup manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         cfg.EnableLeaderElection,
		LeaderElectionID:       "textprocessing.k8s-learning.dev",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Setup Redis queue connection
	var redisQueue *queue.RedisQueue
	if cfg.Redis.Host != "" {
		redisQueue, err = queue.NewRedisQueue(cfg.Redis, log)
		if err != nil {
			log.ErrorContext(context.Background(), "failed to connect to Redis", "error", err)
			// Continue without Redis - controller will still work for basic functionality
			log.WarnContext(context.Background(), "continuing without Redis connection - queue metrics will be unavailable")
		} else {
			log.InfoContext(context.Background(), "Redis connection established for queue monitoring")
		}
	}

	// Setup controllers
	if err = (&workerscaler.WorkerScalerReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    log,
		Queue:  redisQueue,
		Config: *cfg,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "WorkerScaler")
		os.Exit(1)
	}

	// Setup health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Setup metrics collection
	if redisQueue != nil {
		metricsCollector := metrics.NewMetricsCollector(redisQueue, log)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go metricsCollector.StartPeriodicCollection(ctx, cfg.MetricsCollectionInterval)
	}

	// Setup signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
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