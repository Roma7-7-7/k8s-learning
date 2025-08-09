package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rsav/k8s-learning/internal/config"
	"github.com/rsav/k8s-learning/internal/storage/database"
)

const (
	QueueMain      = "text_tasks"
	QueuePriority  = "text_tasks:priority"
	QueueFailed    = "text_tasks:failed"
	QueueHeartbeat = "workers:heartbeat"
)

type SubmitJobMessage struct {
	JobID          uuid.UUID               `json:"job_id"`
	FilePath       string                  `json:"file_path"`
	ProcessingType database.ProcessingType `json:"processing_type"`
	Parameters     map[string]any          `json:"parameters"`
	Priority       int                     `json:"priority"`
}

type RedisQueue struct {
	client *redis.Client
	logger *slog.Logger
}

func NewRedisQueue(config config.Redis, logger *slog.Logger) (*RedisQueue, error) {
	ctx := context.Background()

	logger.InfoContext(ctx, "connecting to Redis", "host", config.Host, "port", config.Port, "db", config.Database)

	client := redis.NewClient(&redis.Options{
		Addr:     config.Address(),
		Password: config.Password,
		DB:       config.Database,
	})

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger.DebugContext(pingCtx, "pinging Redis connection")
	if err := client.Ping(pingCtx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("connect to Redis: %w", err)
	}

	logger.InfoContext(ctx, "Redis connection established successfully")
	return &RedisQueue{client: client, logger: logger}, nil
}

func (rq *RedisQueue) PublishJob(ctx context.Context, message SubmitJobMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal queue message: %w", err)
	}

	queueName := QueueMain
	if message.Priority > 5 {
		queueName = QueuePriority
	}

	rq.logger.DebugContext(ctx, "publishing job to queue", "job_id", message.JobID, "queue", queueName, "processing_type", message.ProcessingType)

	if err := rq.client.LPush(ctx, queueName, data).Err(); err != nil {
		rq.logger.ErrorContext(ctx, "failed to publish job to queue", "job_id", message.JobID, "queue", queueName, "error", err)
		return fmt.Errorf("publish job to queue: %w", err)
	}

	rq.logger.InfoContext(ctx, "job published successfully", "job_id", message.JobID, "queue", queueName)
	return nil
}

func (rq *RedisQueue) GetQueueLength(ctx context.Context, queueName string) (int64, error) {
	length, err := rq.client.LLen(ctx, queueName).Result()
	if err != nil {
		return 0, fmt.Errorf("get queue length: %w", err)
	}
	return length, nil
}

func (rq *RedisQueue) GetAllQueuesLength(ctx context.Context) (map[string]int64, error) {
	queues := []string{QueueMain, QueuePriority, QueueFailed}
	lengths := make(map[string]int64)

	for _, queue := range queues {
		length, err := rq.GetQueueLength(ctx, queue)
		if err != nil {
			return nil, err
		}
		lengths[queue] = length
	}

	return lengths, nil
}

func (rq *RedisQueue) PublishToFailedQueue(ctx context.Context, message SubmitJobMessage, errorMsg string) error {
	failedMessage := struct {
		SubmitJobMessage
		FailedAt     time.Time `json:"failed_at"`
		ErrorMessage string    `json:"error_message"`
		RetryCount   int       `json:"retry_count"`
	}{
		SubmitJobMessage: message,
		FailedAt:         time.Now(),
		ErrorMessage:     errorMsg,
		RetryCount:       1,
	}

	data, err := json.Marshal(failedMessage)
	if err != nil {
		return fmt.Errorf("marshal failed message: %w", err)
	}

	if err := rq.client.LPush(ctx, QueueFailed, data).Err(); err != nil {
		return fmt.Errorf("publish to failed queue: %w", err)
	}

	return nil
}

func (rq *RedisQueue) SetWorkerHeartbeat(ctx context.Context, workerID string) error {
	key := fmt.Sprintf("%s:%s", QueueHeartbeat, workerID)
	heartbeat := map[string]interface{}{
		"worker_id": workerID,
		"last_seen": time.Now().Unix(),
		"status":    "active",
	}

	data, err := json.Marshal(heartbeat)
	if err != nil {
		return fmt.Errorf("marshal heartbeat: %w", err)
	}

	if err := rq.client.Set(ctx, key, data, 5*time.Minute).Err(); err != nil {
		return fmt.Errorf("set worker heartbeat: %w", err)
	}

	return nil
}

func (rq *RedisQueue) GetActiveWorkers(ctx context.Context) ([]string, error) {
	pattern := fmt.Sprintf("%s:*", QueueHeartbeat)
	keys, err := rq.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("get worker keys: %w", err)
	}

	var activeWorkers []string
	for _, key := range keys {
		val, err := rq.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var heartbeat map[string]interface{}
		if err := json.Unmarshal([]byte(val), &heartbeat); err != nil {
			continue
		}

		if workerID, ok := heartbeat["worker_id"].(string); ok {
			activeWorkers = append(activeWorkers, workerID)
		}
	}

	return activeWorkers, nil
}

func (rq *RedisQueue) CleanupStaleWorkers(ctx context.Context, maxAge time.Duration) error {
	pattern := fmt.Sprintf("%s:*", QueueHeartbeat)
	keys, err := rq.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("get worker keys: %w", err)
	}

	cutoff := time.Now().Add(-maxAge).Unix()

	for _, key := range keys {
		val, err := rq.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var heartbeat map[string]interface{}
		if err := json.Unmarshal([]byte(val), &heartbeat); err != nil {
			continue
		}

		if lastSeen, ok := heartbeat["last_seen"].(float64); ok {
			if int64(lastSeen) < cutoff {
				if err := rq.client.Del(ctx, key).Err(); err != nil {
					return fmt.Errorf("cleanup stale worker: %w", err)
				}
			}
		}
	}

	return nil
}

func (rq *RedisQueue) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return rq.client.Ping(ctx).Err()
}

func (rq *RedisQueue) Close() error {
	return rq.client.Close()
}

func (rq *RedisQueue) GetStats(ctx context.Context) (map[string]interface{}, error) {
	queueLengths, err := rq.GetAllQueuesLength(ctx)
	if err != nil {
		return nil, err
	}

	activeWorkers, err := rq.GetActiveWorkers(ctx)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"queues":         queueLengths,
		"active_workers": len(activeWorkers),
		"worker_ids":     activeWorkers,
	}

	return stats, nil
}
