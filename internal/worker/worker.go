package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rsav/k8s-learning/internal/config"
	"github.com/rsav/k8s-learning/internal/storage/database"
	"github.com/rsav/k8s-learning/internal/storage/queue"
	"github.com/rsav/k8s-learning/internal/worker/metrics"
)

type Worker struct {
	config        *config.Worker
	repository    Repository
	queue         JobConsumer
	log           *slog.Logger
	workerID      string
	textProcessor *TextProcessor

	// Control channels
	shutdownCh chan struct{}
	doneCh     chan struct{}
	jobSema    chan struct{}
}

type Repository interface {
	GetJobByID(ctx context.Context, id uuid.UUID) (*database.Job, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status database.JobStatus, workerID *string) error
	UpdateResult(ctx context.Context, id uuid.UUID, resultPath string) error
	UpdateError(ctx context.Context, id uuid.UUID, errorMessage string) error
	HealthCheck(ctx context.Context) error
}

func New(config *config.Worker, repository Repository, queue JobConsumer, log *slog.Logger) (*Worker, error) {
	workerID := config.WorkerID
	if workerID == "" {
		workerID = fmt.Sprintf("worker-%s", uuid.New().String()[:8])
	}

	if err := os.MkdirAll(config.Storage.ResultDir, 0750); err != nil {
		return nil, fmt.Errorf("create result directory: %w", err)
	}

	textProcessor := NewTextProcessor(config.Storage.ResultDir, log)

	return &Worker{
		config:        config,
		repository:    repository,
		queue:         queue,
		log:           log,
		workerID:      workerID,
		textProcessor: textProcessor,
		shutdownCh:    make(chan struct{}),
		doneCh:        make(chan struct{}),
		jobSema:       make(chan struct{}, config.ConcurrentJobs),
	}, nil
}

func (w *Worker) Start(ctx context.Context) error {
	w.log.InfoContext(ctx, "starting worker",
		"worker_id", w.workerID,
		"concurrent_jobs", w.config.ConcurrentJobs)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		w.jobLoop(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		close(w.shutdownCh)
	}()

	wg.Wait()
	close(w.doneCh)

	w.log.InfoContext(ctx, "worker stopped", "worker_id", w.workerID)
	return nil
}

func (w *Worker) Stop() {
	w.log.Info("stopping worker", "worker_id", w.workerID)
	close(w.shutdownCh)
	<-w.doneCh
}

func (w *Worker) jobLoop(ctx context.Context) {
	w.log.InfoContext(ctx, "starting job processing loop", "worker_id", w.workerID)

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.shutdownCh:
			return
		default:
			consumeStart := time.Now()
			message, err := w.queue.ConsumeJob(ctx, w.config.PollInterval)
			metrics.RedisOperationsTotal.WithLabelValues(w.workerID, "consume_job").Inc()
			metrics.RedisOperationDuration.WithLabelValues(w.workerID, "consume_job").Observe(time.Since(consumeStart).Seconds())

			if err != nil {
				if errors.Is(err, queue.ErrNoJobsAvailable) {
					w.log.DebugContext(ctx, "no jobs available, waiting", "worker_id", w.workerID)
					time.Sleep(w.config.PollInterval)
					continue
				}
				w.log.ErrorContext(ctx, "failed to consume job", "error", err, "worker_id", w.workerID)
				time.Sleep(w.config.PollInterval)
				continue
			}

			w.log.InfoContext(ctx, "received job",
				"job_id", message.JobID,
				"processing_type", message.ProcessingType,
				"worker_id", w.workerID)

			select {
			case w.jobSema <- struct{}{}:
				metrics.JobsActive.WithLabelValues(w.workerID).Inc()
				go func(msg *queue.SubmitJobMessage) {
					defer func() {
						<-w.jobSema
						metrics.JobsActive.WithLabelValues(w.workerID).Dec()
					}()
					w.processJob(ctx, msg)
				}(message)
			case <-ctx.Done():
				return
			case <-w.shutdownCh:
				return
			}
		}
	}
}

type contextKey string

const jobIDKey contextKey = "job_id"

func (w *Worker) processJob(ctx context.Context, message *queue.SubmitJobMessage) {
	jobCtx := context.WithValue(ctx, jobIDKey, message.JobID)
	start := time.Now()

	w.log.InfoContext(jobCtx, "processing job",
		"job_id", message.JobID,
		"processing_type", message.ProcessingType,
		"worker_id", w.workerID)

	// Track job delay metric
	if message.DelayMS > 0 {
		const millisecondsToSeconds = 1000.0
		metrics.JobDelaySeconds.WithLabelValues(w.workerID, string(message.ProcessingType)).Observe(float64(message.DelayMS) / millisecondsToSeconds)
	}

	// Record database operation
	updateStart := time.Now()
	if err := w.repository.UpdateStatus(jobCtx, message.JobID, database.JobStatusRunning, &w.workerID); err != nil {
		w.log.ErrorContext(jobCtx, "failed to update job status to running", "error", err, "job_id", message.JobID)
		metrics.DBQueriesTotal.WithLabelValues(w.workerID, "update_status").Inc()
		metrics.DBQueryDuration.WithLabelValues(w.workerID, "update_status").Observe(time.Since(updateStart).Seconds())
		metrics.JobsProcessedTotal.WithLabelValues(w.workerID, string(message.ProcessingType), "failed").Inc()

		redisStart := time.Now()
		if publishErr := w.queue.PublishToFailedQueue(jobCtx, *message, err.Error()); publishErr != nil {
			w.log.ErrorContext(jobCtx, "failed to publish job to failed queue", "error", publishErr, "job_id", message.JobID)
		}
		metrics.RedisOperationsTotal.WithLabelValues(w.workerID, "publish_failed").Inc()
		metrics.RedisOperationDuration.WithLabelValues(w.workerID, "publish_failed").Observe(time.Since(redisStart).Seconds())
		return
	}
	metrics.DBQueriesTotal.WithLabelValues(w.workerID, "update_status").Inc()
	metrics.DBQueryDuration.WithLabelValues(w.workerID, "update_status").Observe(time.Since(updateStart).Seconds())

	processingJob := &ProcessingJob{
		JobID:          message.JobID.String(),
		FilePath:       message.FilePath,
		ProcessingType: message.ProcessingType,
		Parameters:     message.Parameters,
		DelayMS:        message.DelayMS,
	}

	outputPath, err := w.textProcessor.Process(jobCtx, processingJob)
	if err != nil {
		w.log.ErrorContext(jobCtx, "processor failed", "error", err, "job_id", message.JobID)
		updateStart := time.Now()
		if updateErr := w.repository.UpdateError(jobCtx, message.JobID, err.Error()); updateErr != nil {
			w.log.ErrorContext(jobCtx, "failed to update job error", "error", updateErr, "job_id", message.JobID)
		}
		metrics.DBQueriesTotal.WithLabelValues(w.workerID, "update_error").Inc()
		metrics.DBQueryDuration.WithLabelValues(w.workerID, "update_error").Observe(time.Since(updateStart).Seconds())
		metrics.JobsProcessedTotal.WithLabelValues(w.workerID, string(message.ProcessingType), "failed").Inc()
		metrics.JobProcessingDuration.WithLabelValues(w.workerID, string(message.ProcessingType)).Observe(time.Since(start).Seconds())
		return
	}

	updateStart = time.Now()
	if err := w.repository.UpdateResult(jobCtx, message.JobID, outputPath); err != nil {
		w.log.ErrorContext(jobCtx, "failed to update job result", "error", err, "job_id", message.JobID)
		metrics.DBQueriesTotal.WithLabelValues(w.workerID, "update_result").Inc()
		metrics.DBQueryDuration.WithLabelValues(w.workerID, "update_result").Observe(time.Since(updateStart).Seconds())
		if updateErr := w.repository.UpdateError(jobCtx, message.JobID, err.Error()); updateErr != nil {
			w.log.ErrorContext(jobCtx, "failed to update job error after result update failure", "error", updateErr, "job_id", message.JobID)
		}
		metrics.JobsProcessedTotal.WithLabelValues(w.workerID, string(message.ProcessingType), "failed").Inc()
		metrics.JobProcessingDuration.WithLabelValues(w.workerID, string(message.ProcessingType)).Observe(time.Since(start).Seconds())
		return
	}
	metrics.DBQueriesTotal.WithLabelValues(w.workerID, "update_result").Inc()
	metrics.DBQueryDuration.WithLabelValues(w.workerID, "update_result").Observe(time.Since(updateStart).Seconds())

	// Record successful job completion
	metrics.JobsProcessedTotal.WithLabelValues(w.workerID, string(message.ProcessingType), "success").Inc()
	metrics.JobProcessingDuration.WithLabelValues(w.workerID, string(message.ProcessingType)).Observe(time.Since(start).Seconds())

	w.log.InfoContext(jobCtx, "job completed successfully",
		"job_id", message.JobID,
		"output_path", outputPath,
		"worker_id", w.workerID)
}

func (w *Worker) HealthCheck(ctx context.Context) error {
	if err := w.repository.HealthCheck(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	if err := w.queue.HealthCheck(ctx); err != nil {
		return fmt.Errorf("queue health check failed: %w", err)
	}

	return nil
}
