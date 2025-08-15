package workerscaler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/rsav/k8s-learning/internal/config"
	"github.com/rsav/k8s-learning/internal/controller/metrics"
	"github.com/rsav/k8s-learning/internal/storage/queue"
)

const (
	// Worker deployment constants
	WorkerDeploymentName      = "worker"
	WorkerDeploymentNamespace = "k8s-learning"
	
	// Scaling parameters
	DefaultMinReplicas = 1
	DefaultMaxReplicas = 10
	ScaleUpThreshold   = 20  // Scale up when queue depth > 20
	ScaleDownThreshold = 5   // Scale down when queue depth < 5
	JobsPerWorker      = 10  // Estimated jobs per worker capacity
)

// WorkerScalerReconciler manages worker deployment scaling based on queue depth
type WorkerScalerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    *slog.Logger
	Queue  *queue.RedisQueue
	Config config.Controller
}

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile monitors queue depth and scales worker deployment
func (r *WorkerScalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With("worker-scaler", "queue-monitor")
	
	log.DebugContext(ctx, "starting worker scaling reconciliation")

	// Get current worker deployment
	var deployment appsv1.Deployment
	deploymentKey := types.NamespacedName{
		Name:      WorkerDeploymentName,
		Namespace: WorkerDeploymentNamespace,
	}
	
	if err := r.Get(ctx, deploymentKey, &deployment); err != nil {
		if apierrors.IsNotFound(err) {
			log.InfoContext(ctx, "worker deployment not found, skipping scaling")
			return ctrl.Result{RequeueAfter: time.Duration(r.Config.ReconcileIntervalSeconds) * time.Second}, nil
		}
		log.ErrorContext(ctx, "failed to get worker deployment", "error", err)
		return ctrl.Result{}, err
	}

	// Get current queue metrics
	queueStats, err := r.getQueueStats(ctx)
	if err != nil {
		log.ErrorContext(ctx, "failed to get queue stats", "error", err)
		// Continue with last known values, don't fail reconciliation
		queueStats = &QueueStats{TotalDepth: 0, ActiveWorkers: int(deployment.Status.ReadyReplicas)}
	}

	// Calculate optimal replica count
	currentReplicas := *deployment.Spec.Replicas
	optimalReplicas := r.calculateOptimalReplicas(queueStats, currentReplicas)
	
	log.InfoContext(ctx, "scaling analysis",
		"current_replicas", currentReplicas,
		"optimal_replicas", optimalReplicas,
		"queue_depth", queueStats.TotalDepth,
		"active_workers", queueStats.ActiveWorkers)

	// Update deployment if scaling is needed
	if optimalReplicas != currentReplicas {
		deployment.Spec.Replicas = &optimalReplicas
		
		if err := r.Update(ctx, &deployment); err != nil {
			log.ErrorContext(ctx, "failed to update worker deployment", "error", err)
			return ctrl.Result{}, err
		}

		// Record scaling event
		direction := "up"
		if optimalReplicas < currentReplicas {
			direction = "down"
		}
		metrics.RecordAutoscalingEvent("worker-deployment", direction)
		
		log.InfoContext(ctx, "scaled worker deployment",
			"from", currentReplicas,
			"to", optimalReplicas,
			"direction", direction,
			"reason", fmt.Sprintf("queue_depth=%d", queueStats.TotalDepth))
	}

	// Update metrics
	metrics.UpdateReplicasMetrics("worker-deployment", "mixed", currentReplicas, optimalReplicas)

	// Requeue for next check
	return ctrl.Result{RequeueAfter: time.Duration(r.Config.ReconcileIntervalSeconds) * time.Second}, nil
}

// QueueStats holds queue and worker statistics
type QueueStats struct {
	TotalDepth    int64
	ActiveWorkers int
}

func (r *WorkerScalerReconciler) getQueueStats(ctx context.Context) (*QueueStats, error) {
	if r.Queue == nil {
		return &QueueStats{TotalDepth: 0, ActiveWorkers: 0}, nil
	}

	// Get queue depths
	queueLengths, err := r.Queue.GetAllQueuesLength(ctx)
	if err != nil {
		return nil, fmt.Errorf("get queue lengths: %w", err)
	}

	// Calculate total depth (main + priority queues, exclude failed)
	totalDepth := queueLengths[queue.QueueMain] + queueLengths[queue.QueuePriority]

	// Get active workers
	workers, err := r.Queue.GetActiveWorkers(ctx)
	if err != nil {
		return nil, fmt.Errorf("get active workers: %w", err)
	}

	return &QueueStats{
		TotalDepth:    totalDepth,
		ActiveWorkers: len(workers),
	}, nil
}

func (r *WorkerScalerReconciler) calculateOptimalReplicas(stats *QueueStats, currentReplicas int32) int32 {
	// If no queue connection, maintain current replicas
	if r.Queue == nil {
		return currentReplicas
	}

	queueDepth := stats.TotalDepth
	
	// Calculate optimal replicas based on queue depth
	var targetReplicas int32
	
	if queueDepth == 0 {
		// No jobs in queue - scale down to minimum
		targetReplicas = DefaultMinReplicas
	} else if queueDepth > ScaleUpThreshold {
		// High queue depth - scale up
		// Formula: ceil(queueDepth / JobsPerWorker) but limit growth rate
		needed := (queueDepth + JobsPerWorker - 1) / JobsPerWorker // Ceiling division
		targetReplicas = min(currentReplicas+2, int32(needed)) // Scale up by max 2 at a time
	} else if queueDepth < ScaleDownThreshold && currentReplicas > DefaultMinReplicas {
		// Low queue depth - scale down gradually
		targetReplicas = currentReplicas - 1
	} else {
		// Queue depth is in acceptable range - no change
		targetReplicas = currentReplicas
	}

	// Apply constraints
	if targetReplicas < DefaultMinReplicas {
		targetReplicas = DefaultMinReplicas
	}
	if targetReplicas > DefaultMaxReplicas {
		targetReplicas = DefaultMaxReplicas
	}

	return targetReplicas
}

// SetupWithManager sets up the controller with the Manager
func (r *WorkerScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Complete(r)
}

// min returns the minimum of two int32 values
func min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}