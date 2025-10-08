package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	dto "github.com/prometheus/client_model/go"

	"github.com/rsav/k8s-learning/internal/storage/queue"
)

var (
	// Queue metrics.
	queueDepthGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "textprocessing_queue_depth",
			Help: "Current depth of text processing queues",
		},
		[]string{"queue_name"},
	)

	activeWorkersGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "textprocessing_active_workers",
			Help: "Number of active text processing workers",
		},
	)

	jobsProcessedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "textprocessing_jobs_processed_total",
			Help: "Total number of text processing jobs processed",
		},
		[]string{"processing_type", "status"},
	)

	controllerReconciliationsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "textprocessing_controller_reconciliations_total",
			Help: "Total number of controller reconciliations",
		},
		[]string{"controller", "result"},
	)

	controllerReconciliationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "textprocessing_controller_reconciliation_duration_seconds",
			Help:    "Duration of controller reconciliations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"controller"},
	)

	// Scaling metrics.
	autoscalingEventsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "textprocessing_autoscaling_events_total",
			Help: "Total number of autoscaling events",
		},
		[]string{"job_name", "direction"},
	)

	currentReplicasGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "textprocessing_current_replicas",
			Help: "Current number of replicas for each TextProcessingJob",
		},
		[]string{"job_name", "processing_type"},
	)

	desiredReplicasGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "textprocessing_desired_replicas",
			Help: "Desired number of replicas for each TextProcessingJob",
		},
		[]string{"job_name", "processing_type"},
	)
)

// Collector collects and updates Prometheus metrics.
type Collector struct {
	queue *queue.RedisQueue
	log   *slog.Logger
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector(queue *queue.RedisQueue, log *slog.Logger) *Collector {
	return &Collector{
		queue: queue,
		log:   log,
	}
}

// StartPeriodicCollection starts periodic metrics collection.
func (m *Collector) StartPeriodicCollection(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	m.log.InfoContext(ctx, "starting periodic metrics collection", "interval", interval)

	for {
		select {
		case <-ctx.Done():
			m.log.InfoContext(ctx, "stopping metrics collection")
			return
		case <-ticker.C:
			if err := m.CollectQueueMetrics(ctx); err != nil {
				m.log.ErrorContext(ctx, "failed to collect queue metrics", "error", err)
			}
		}
	}
}

// CollectQueueMetrics collects queue-related metrics.
func (m *Collector) CollectQueueMetrics(ctx context.Context) error {
	if m.queue == nil {
		return nil
	}

	// Get queue depths
	queueLengths, err := m.queue.GetAllQueuesLength(ctx)
	if err != nil {
		return err
	}

	for queueName, length := range queueLengths {
		queueDepthGauge.WithLabelValues(queueName).Set(float64(length))
	}

	// Get active workers
	workers, err := m.queue.GetActiveWorkers(ctx)
	if err != nil {
		return err
	}

	activeWorkersGauge.Set(float64(len(workers)))

	m.log.DebugContext(ctx, "collected queue metrics",
		"queue_lengths", queueLengths,
		"active_workers", len(workers))

	return nil
}

// RecordJobProcessed records a processed job.
func RecordJobProcessed(processingType, status string) {
	jobsProcessedCounter.WithLabelValues(processingType, status).Inc()
}

// RecordReconciliation records a controller reconciliation.
func RecordReconciliation(controller, result string, duration time.Duration) {
	controllerReconciliationsCounter.WithLabelValues(controller, result).Inc()
	controllerReconciliationDuration.WithLabelValues(controller).Observe(duration.Seconds())
}

// RecordAutoscalingEvent records an autoscaling event.
func RecordAutoscalingEvent(jobName, direction string) {
	autoscalingEventsCounter.WithLabelValues(jobName, direction).Inc()
}

// UpdateReplicasMetrics updates replica count metrics.
func UpdateReplicasMetrics(jobName, processingType string, current, desired int32) {
	currentReplicasGauge.WithLabelValues(jobName, processingType).Set(float64(current))
	desiredReplicasGauge.WithLabelValues(jobName, processingType).Set(float64(desired))
}

// GetQueueDepth returns the current queue depth for a specific queue.
func GetQueueDepth(queueName string) float64 {
	if gauge, err := queueDepthGauge.GetMetricWithLabelValues(queueName); err == nil {
		metric := &dto.Metric{}
		_ = gauge.Write(metric)
		return metric.GetGauge().GetValue()
	}
	return 0
}

// GetActiveWorkerCount returns the current number of active workers.
func GetActiveWorkerCount() float64 {
	metric := &dto.Metric{}
	_ = activeWorkersGauge.Write(metric)
	return metric.GetGauge().GetValue()
}
