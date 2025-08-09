package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type Health struct {
	repo   Repository
	queue  Queue
	logger *slog.Logger
}

func NewHealth(repo Repository, queue Queue, log *slog.Logger) *Health {
	return &Health{
		repo:   repo,
		queue:  queue,
		logger: log,
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
		hh.logger.ErrorContext(r.Context(), "database health check failed", "error", err)
	} else {
		checks["database"] = "healthy"
	}

	if err := hh.queue.HealthCheck(r.Context()); err != nil {
		checks["redis"] = "unhealthy"
		allHealthy = false
		hh.logger.ErrorContext(r.Context(), "redis health check failed", "error", err)
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
		hh.logger.Error("failed to get queue stats", "error", err)
		hh.writeError(w, http.StatusInternalServerError, "failed to get queue stats")
		return
	}

	wg := sync.WaitGroup{}

	var totalJobs int
	wg.Add(1)
	go func() {
		var gErr error
		defer wg.Done()
		totalJobs, gErr = hh.repo.CountJobs(r.Context())
		if gErr != nil {
			hh.logger.Error("failed to count total jobs", "error", gErr)
			totalJobs = -1
		}
	}()

	var pendingJobs int
	wg.Add(1)
	go func() {
		var gErr error
		defer wg.Done()
		pendingJobs, gErr = hh.repo.CountJobsByStatus(r.Context(), "pending")
		if gErr != nil {
			hh.logger.Error("failed to count pending jobs", "error", gErr)
			pendingJobs = -1
		}
	}()

	var runningJobs int
	wg.Add(1)
	go func() {
		var gErr error
		defer wg.Done()
		runningJobs, gErr = hh.repo.CountJobsByStatus(r.Context(), "running")
		if gErr != nil {
			hh.logger.Error("failed to count running jobs", "error", gErr)
			runningJobs = -1
		}
	}()

	var succeededJobs int
	wg.Add(1)
	go func() {
		var gErr error
		defer wg.Done()
		succeededJobs, gErr = hh.repo.CountJobsByStatus(r.Context(), "succeeded")
		if gErr != nil {
			hh.logger.Error("failed to count succeeded jobs", "error", gErr)
			succeededJobs = -1
		}
	}()

	var failedJobs int
	wg.Add(1)
	go func() {
		var gErr error
		defer wg.Done()
		failedJobs, gErr = hh.repo.CountJobsByStatus(r.Context(), "failed")
		if gErr != nil {
			hh.logger.Error("failed to count failed jobs", "error", gErr)
			failedJobs = -1
		}
	}()

	stats := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"service":   "text-api",
		"queue":     queueStats,
		"jobs": map[string]interface{}{
			"total":     totalJobs,
			"pending":   pendingJobs,
			"running":   runningJobs,
			"succeeded": succeededJobs,
			"failed":    failedJobs,
		},
	}

	hh.writeJSON(w, http.StatusOK, stats)
}

func (hh *Health) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		hh.logger.Error("failed to encode JSON response", "error", err)
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

	json.NewEncoder(w).Encode(errorResponse)
}
