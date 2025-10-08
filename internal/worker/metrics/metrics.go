package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// JobsProcessedTotal tracks the total number of jobs processed by the worker.
	JobsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_jobs_processed_total",
			Help: "Total number of jobs processed by the worker",
		},
		[]string{"worker_id", "processing_type", "status"},
	)

	// JobProcessingDuration tracks job processing duration in seconds.
	JobProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "worker_job_processing_duration_seconds",
			Help:    "Job processing duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"worker_id", "processing_type"},
	)

	// JobsActive tracks the number of jobs currently being processed.
	JobsActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "worker_jobs_active",
			Help: "Number of jobs currently being processed by the worker",
		},
		[]string{"worker_id"},
	)

	// JobDelaySeconds tracks the configured delay for jobs in seconds.
	JobDelaySeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "worker_job_delay_seconds",
			Help:    "Configured delay for jobs in seconds",
			Buckets: []float64{0, 1, 5, 10, 30, 60},
		},
		[]string{"worker_id", "processing_type"},
	)

	// DBQueriesTotal tracks the total number of database queries by operation.
	DBQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_db_queries_total",
			Help: "Total number of database queries by the worker",
		},
		[]string{"worker_id", "operation"},
	)

	// DBQueryDuration tracks database query duration in seconds.
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "worker_db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"worker_id", "operation"},
	)

	// RedisOperationsTotal tracks the total number of Redis operations.
	RedisOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_redis_operations_total",
			Help: "Total number of Redis operations by the worker",
		},
		[]string{"worker_id", "operation"},
	)

	// RedisOperationDuration tracks Redis operation duration in seconds.
	RedisOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "worker_redis_operation_duration_seconds",
			Help:    "Redis operation duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"worker_id", "operation"},
	)

	// WorkerInfo provides worker metadata as labels.
	WorkerInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "worker_info",
			Help: "Worker information (constant 1)",
		},
		[]string{"worker_id", "version"},
	)
)
