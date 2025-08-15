package textprocessingjob

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/rsav/k8s-learning/internal/config"
	"github.com/rsav/k8s-learning/internal/models/v1alpha1"
	"github.com/rsav/k8s-learning/internal/storage/queue"
)

const (
	// TextProcessingJobFinalizerName is the finalizer name for cleanup
	TextProcessingJobFinalizerName = "textprocessingjob.k8s-learning.dev/finalizer"
	
	// Labels for managed resources
	LabelJobName = "textprocessingjob.k8s-learning.dev/job-name"
	LabelJobType = "textprocessingjob.k8s-learning.dev/job-type"
	LabelComponent = "textprocessingjob.k8s-learning.dev/component"
	
	// Conditions
	ConditionReady = "Ready"
	ConditionScaling = "Scaling"
	ConditionQueueMonitoring = "QueueMonitoring"
)

// TextProcessingJobReconciler reconciles a TextProcessingJob object
type TextProcessingJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    *slog.Logger
	Queue  *queue.RedisQueue
	Config config.Controller
}

// +kubebuilder:rbac:groups=textprocessing.k8s-learning.dev,resources=textprocessingjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=textprocessing.k8s-learning.dev,resources=textprocessingjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=textprocessing.k8s-learning.dev,resources=textprocessingjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *TextProcessingJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With("textprocessingjob", req.NamespacedName)
	
	log.InfoContext(ctx, "starting reconciliation")

	// Fetch the TextProcessingJob instance
	var textJob v1alpha1.TextProcessingJob
	if err := r.Get(ctx, req.NamespacedName, &textJob); err != nil {
		if apierrors.IsNotFound(err) {
			log.InfoContext(ctx, "textprocessingjob resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.ErrorContext(ctx, "failed to get textprocessingjob", "error", err)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if textJob.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &textJob, log)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&textJob, TextProcessingJobFinalizerName) {
		controllerutil.AddFinalizer(&textJob, TextProcessingJobFinalizerName)
		if err := r.Update(ctx, &textJob); err != nil {
			log.ErrorContext(ctx, "failed to add finalizer", "error", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile the desired state
	result, err := r.reconcileNormal(ctx, &textJob, log)
	if err != nil {
		log.ErrorContext(ctx, "reconciliation failed", "error", err)
		r.updateStatus(ctx, &textJob, "Failed", err.Error())
		return result, err
	}

	log.InfoContext(ctx, "reconciliation completed successfully")
	return result, nil
}

func (r *TextProcessingJobReconciler) reconcileNormal(ctx context.Context, textJob *v1alpha1.TextProcessingJob, log *slog.Logger) (ctrl.Result, error) {
	// Update queue metrics
	if err := r.updateQueueMetrics(ctx, textJob, log); err != nil {
		log.ErrorContext(ctx, "failed to update queue metrics", "error", err)
		// Don't fail reconciliation for metrics errors
	}

	// Reconcile worker deployment
	if err := r.reconcileDeployment(ctx, textJob, log); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile deployment: %w", err)
	}

	// Update status
	r.updateStatus(ctx, textJob, "Running", "Processing jobs")

	// Determine scaling strategy
	queueDepth := textJob.Status.QueueDepth
	currentReplicas := textJob.Status.ActiveReplicas
	desiredReplicas := textJob.Spec.Replicas

	// Auto-scaling logic based on queue depth
	if r.Config.EnableAutoScaling {
		scaledReplicas := r.calculateOptimalReplicas(queueDepth, currentReplicas, desiredReplicas)
		if scaledReplicas != desiredReplicas {
			log.InfoContext(ctx, "auto-scaling adjustment", 
				"current_replicas", currentReplicas,
				"desired_replicas", desiredReplicas,
				"scaled_replicas", scaledReplicas,
				"queue_depth", queueDepth)
			
			textJob.Spec.Replicas = scaledReplicas
			if err := r.Update(ctx, textJob); err != nil {
				return ctrl.Result{}, fmt.Errorf("update replicas for auto-scaling: %w", err)
			}
		}
	}

	// Requeue for periodic reconciliation
	return ctrl.Result{RequeueAfter: time.Duration(r.Config.ReconcileIntervalSeconds) * time.Second}, nil
}

func (r *TextProcessingJobReconciler) calculateOptimalReplicas(queueDepth, currentReplicas, desiredReplicas int32) int32 {
	// Simple scaling algorithm:
	// - Scale up if queue depth > 10 per replica
	// - Scale down if queue depth < 2 per replica
	// - Respect min/max bounds
	
	const (
		minReplicas = 1
		maxReplicas = 10
		scaleUpThreshold = 10
		scaleDownThreshold = 2
	)

	if currentReplicas == 0 {
		return desiredReplicas
	}

	avgQueuePerReplica := queueDepth / currentReplicas
	
	var targetReplicas int32
	if avgQueuePerReplica > scaleUpThreshold && currentReplicas < maxReplicas {
		targetReplicas = currentReplicas + 1
	} else if avgQueuePerReplica < scaleDownThreshold && currentReplicas > minReplicas {
		targetReplicas = currentReplicas - 1
	} else {
		targetReplicas = currentReplicas
	}

	// Ensure we're within bounds
	if targetReplicas < minReplicas {
		targetReplicas = minReplicas
	}
	if targetReplicas > maxReplicas {
		targetReplicas = maxReplicas
	}

	return targetReplicas
}

func (r *TextProcessingJobReconciler) updateQueueMetrics(ctx context.Context, textJob *v1alpha1.TextProcessingJob, log *slog.Logger) error {
	if r.Queue == nil {
		return nil
	}

	queueDepth, err := r.Queue.GetQueueLength(ctx, queue.QueueMain)
	if err != nil {
		return fmt.Errorf("get queue length: %w", err)
	}

	priorityQueueDepth, err := r.Queue.GetQueueLength(ctx, queue.QueuePriority)
	if err != nil {
		return fmt.Errorf("get priority queue length: %w", err)
	}

	totalQueueDepth := queueDepth + priorityQueueDepth
	textJob.Status.QueueDepth = int32(totalQueueDepth)

	log.DebugContext(ctx, "updated queue metrics", 
		"main_queue", queueDepth,
		"priority_queue", priorityQueueDepth,
		"total_queue_depth", totalQueueDepth)

	return nil
}

func (r *TextProcessingJobReconciler) reconcileDeployment(ctx context.Context, textJob *v1alpha1.TextProcessingJob, log *slog.Logger) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-workers", textJob.Name),
			Namespace: textJob.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(textJob, deployment, r.Scheme); err != nil {
			return err
		}

		// Update labels
		if deployment.Labels == nil {
			deployment.Labels = make(map[string]string)
		}
		deployment.Labels[LabelJobName] = textJob.Name
		deployment.Labels[LabelJobType] = textJob.Spec.ProcessingType
		deployment.Labels[LabelComponent] = "worker"

		// Update deployment spec
		deployment.Spec = appsv1.DeploymentSpec{
			Replicas: &textJob.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					LabelJobName: textJob.Name,
					LabelComponent: "worker",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelJobName:   textJob.Name,
						LabelJobType:   textJob.Spec.ProcessingType,
						LabelComponent: "worker",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "worker",
							Image: r.Config.WorkerImage,
							Env: []corev1.EnvVar{
								{
									Name:  "PROCESSING_TYPE_FILTER",
									Value: textJob.Spec.ProcessingType,
								},
								{
									Name:  "JOB_PRIORITY_MIN",
									Value: fmt.Sprintf("%d", textJob.Spec.Priority),
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    r.Config.WorkerResourceRequests.CPU,
									corev1.ResourceMemory: r.Config.WorkerResourceRequests.Memory,
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    r.Config.WorkerResourceLimits.CPU,
									corev1.ResourceMemory: r.Config.WorkerResourceLimits.Memory,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("create or update deployment: %w", err)
	}

	log.InfoContext(ctx, "deployment reconciled", "operation", op, "deployment", deployment.Name)

	// Update status with current replica count
	if deployment.Status.ReadyReplicas > 0 {
		textJob.Status.ActiveReplicas = deployment.Status.ReadyReplicas
	}

	return nil
}

func (r *TextProcessingJobReconciler) updateStatus(ctx context.Context, textJob *v1alpha1.TextProcessingJob, phase, message string) {
	textJob.Status.Phase = phase
	textJob.Status.Message = message

	now := metav1.Now()
	if phase == "Running" && textJob.Status.StartTime == nil {
		textJob.Status.StartTime = &now
	}
	if phase == "Completed" || phase == "Failed" {
		textJob.Status.CompletionTime = &now
	}

	// Update conditions
	condition := metav1.Condition{
		Type:    ConditionReady,
		Status:  metav1.ConditionTrue,
		Reason:  phase,
		Message: message,
		LastTransitionTime: now,
	}

	if phase == "Failed" {
		condition.Status = metav1.ConditionFalse
	}

	// Find and update or append condition
	found := false
	for i, cond := range textJob.Status.Conditions {
		if cond.Type == ConditionReady {
			textJob.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		textJob.Status.Conditions = append(textJob.Status.Conditions, condition)
	}

	if err := r.Status().Update(ctx, textJob); err != nil {
		r.Log.ErrorContext(ctx, "failed to update status", "error", err)
	}
}

func (r *TextProcessingJobReconciler) handleDeletion(ctx context.Context, textJob *v1alpha1.TextProcessingJob, log *slog.Logger) (ctrl.Result, error) {
	log.InfoContext(ctx, "handling textprocessingjob deletion")

	// Perform cleanup tasks here
	// For example, drain any pending jobs, cleanup resources, etc.

	// Remove finalizer
	controllerutil.RemoveFinalizer(textJob, TextProcessingJobFinalizerName)
	if err := r.Update(ctx, textJob); err != nil {
		log.ErrorContext(ctx, "failed to remove finalizer", "error", err)
		return ctrl.Result{}, err
	}

	log.InfoContext(ctx, "textprocessingjob deletion completed")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TextProcessingJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.TextProcessingJob{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}