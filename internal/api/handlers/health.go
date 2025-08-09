package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type Health struct {
	repo  Repository
	queue Queue
	log   *slog.Logger
}

func NewHealth(repo Repository, queue Queue, log *slog.Logger) *Health {
	return &Health{
		repo:  repo,
		queue: queue,
		log:   log,
	}
}

func (hh *Health) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		hh.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"service":   "text-api",
	}

	hh.writeJSON(w, http.StatusOK, health)
}

func (hh *Health) Ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		hh.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	checks := map[string]interface{}{
		"database": "unknown",
		"redis":    "unknown",
		"shutdown": "unknown",
	}

	allHealthy := true

	if err := hh.repo.HealthCheck(r.Context()); err != nil {
		checks["database"] = "unhealthy"
		allHealthy = false
		hh.log.ErrorContext(r.Context(), "database health check failed", "error", err)
	} else {
		checks["database"] = "healthy"
	}

	if err := hh.queue.HealthCheck(r.Context()); err != nil {
		checks["redis"] = "unhealthy"
		allHealthy = false
		hh.log.ErrorContext(r.Context(), "redis health check failed", "error", err)
	} else {
		checks["redis"] = "healthy"
	}

	status := "ready"
	statusCode := http.StatusOK

	if !allHealthy {
		status = "not ready"
		statusCode = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().Unix(),
		"checks":    checks,
		"service":   "text-api",
	}

	hh.writeJSON(w, statusCode, response)
}

func (hh *Health) Stats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		hh.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	queueStats, err := hh.queue.GetStats(r.Context())
	if err != nil {
		hh.log.Error("failed to get queue stats", "error", err)
		hh.writeError(w, http.StatusInternalServerError, "failed to get queue stats")
		return
	}

	wg := &sync.WaitGroup{}
	jobStats := &sync.Map{}

	wg.Add(1)
	go statFetcher(wg, func() (int, error) { return hh.repo.CountJobs(r.Context()) }, "total", jobStats, hh.log)

	wg.Add(1)
	go statFetcher(wg, func() (int, error) { return hh.repo.CountJobsByStatus(r.Context(), "pending") }, "pending", jobStats, hh.log)

	wg.Add(1)
	go statFetcher(wg, func() (int, error) { return hh.repo.CountJobsByStatus(r.Context(), "running") }, "running", jobStats, hh.log)

	wg.Add(1)
	go statFetcher(wg, func() (int, error) { return hh.repo.CountJobsByStatus(r.Context(), "succeeded") }, "succeeded", jobStats, hh.log)

	wg.Add(1)
	go statFetcher(wg, func() (int, error) { return hh.repo.CountJobsByStatus(r.Context(), "failed") }, "failed", jobStats, hh.log)

	wg.Wait()
	jobsMap := make(map[string]int)
	jobStats.Range(func(key, value interface{}) bool {
		if strKey, ok := key.(string); ok {
			if intValue, ok := value.(int); ok {
				jobsMap[strKey] = intValue
			} else {
				hh.log.Error("job stats value is not an int", "key", strKey, "value", value)
			}
		} else {
			hh.log.Error("job stats key is not a string", "key", key)
		}
		return true
	})

	stats := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"service":   "text-api",
		"queue":     queueStats,
		"jobs":      jobsMap,
	}

	hh.writeJSON(w, http.StatusOK, stats)
}

func (hh *Health) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		hh.log.Error("failed to encode JSON response", "error", err)
	}
}

func (hh *Health) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := map[string]interface{}{
		"error":     message,
		"status":    statusCode,
		"timestamp": time.Now().Unix(),
	}

	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		hh.log.Error("failed to encode error response", "error", err)
	}
}

func statFetcher(wg *sync.WaitGroup, f func() (int, error), key string, target *sync.Map, log *slog.Logger) {
	defer wg.Done()

	val, err := f()
	if err != nil {
		log.Error("failed to fetch stat", "key", key, "error", err)
		val = -1 // Default value on error
	}
	target.Store(key, val)
}
