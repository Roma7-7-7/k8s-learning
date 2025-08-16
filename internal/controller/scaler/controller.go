package scaler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/rsav/k8s-learning/internal/config"
	"github.com/rsav/k8s-learning/internal/controller/metrics"
	"github.com/rsav/k8s-learning/internal/storage/queue"
)

const (
	WorkerDeploymentName      = "worker"
	WorkerDeploymentNamespace = "k8s-learning"

	DefaultMinReplicas = 1
	DefaultMaxReplicas = 10
	ScaleUpThreshold   = 20 // Scale up when queue depth > 20
	ScaleDownThreshold = 5  // Scale down when queue depth < 5
	JobsPerWorker      = 10 // Estimated jobs per worker capacity
)

type Worker struct {
	client.Client
	Log    *slog.Logger
	Queue  *queue.RedisQueue
	Config config.Controller
}

func (r *Worker) StartPeriodicScaling(ctx context.Context) {
	ticker := time.NewTicker(r.Config.ReconcileInterval)
	defer ticker.Stop()

	r.Log.InfoContext(ctx, "starting periodic reconciliation",
		"interval", r.Config.ReconcileInterval)

	for {
		select {
		case <-ticker.C:
			// Call scaling logic directly - no controller-runtime reconcile needed
			err := r.scaleWorkerDeployment(ctx)
			if err != nil {
				r.Log.ErrorContext(ctx, "periodic scaling failed", "error", err)
			}

		case <-ctx.Done():
			r.Log.InfoContext(ctx, "stopping periodic reconciliation")
			return
		}
	}
}

func (r *Worker) scaleWorkerDeployment(ctx context.Context) error {
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
			return nil
		}
		log.ErrorContext(ctx, "failed to get worker deployment", "error", err)
		return err
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
		err := r.updateDeploymentReplicas(ctx, &deployment, optimalReplicas)
		if err != nil {
			log.ErrorContext(ctx, "failed to update worker deployment", "error", err)
			return err
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
	return nil
}

// QueueStats holds queue and worker statistics
type QueueStats struct {
	TotalDepth    int64
	ActiveWorkers int
}

func (r *Worker) getQueueStats(ctx context.Context) (*QueueStats, error) {
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

	r.Log.DebugContext(ctx, "collected queue metrics",
		"queue_lengths", queueLengths,
		"active_workers", len(workers))

	return &QueueStats{
		TotalDepth:    totalDepth,
		ActiveWorkers: len(workers),
	}, nil
}

func (r *Worker) calculateOptimalReplicas(stats *QueueStats, currentReplicas int32) int32 {
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
		neededReplicas := int32(needed)
		if neededReplicas < 0 { // Overflow protection
			neededReplicas = DefaultMaxReplicas
		}
		targetReplicas = min(currentReplicas+2, neededReplicas) // Scale up by max 2 at a time
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

func (r *Worker) updateDeploymentReplicas(ctx context.Context, _ *appsv1.Deployment, replicas int32) error {
	var freshDeployment appsv1.Deployment
	deploymentKey := types.NamespacedName{
		Name:      WorkerDeploymentName,
		Namespace: WorkerDeploymentNamespace,
	}

	if err := r.Get(ctx, deploymentKey, &freshDeployment); err != nil {
		r.Log.ErrorContext(ctx, "failed to get fresh deployment for update", "error", err)
		return fmt.Errorf("get fresh deployment: %w", err)
	}

	r.Log.DebugContext(ctx, "attempting deployment update",
		"old_replicas", *freshDeployment.Spec.Replicas,
		"new_replicas", replicas,
		"resource_version", freshDeployment.ResourceVersion)

	// Create a copy for patching
	original := freshDeployment.DeepCopy()

	// Update the replica count
	freshDeployment.Spec.Replicas = &replicas

	// Create patch
	patch := client.MergeFrom(original)

	err := r.Patch(ctx, &freshDeployment, patch)
	if err != nil {
		if apierrors.IsConflict(err) {
			r.Log.DebugContext(ctx, "patch conflict, retrying",
				"error", err,
				"resource_version", freshDeployment.ResourceVersion)
			return nil
		}
		return fmt.Errorf("patch deployment: %w", err)
	}

	r.Log.DebugContext(ctx, "deployment patch successful",
		"new_resource_version", freshDeployment.ResourceVersion)
	return nil
}

// minInt32 returns the minimum of two int32 values.
func min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
