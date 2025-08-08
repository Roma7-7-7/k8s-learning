package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type HealthHandler struct {
	db              DatabaseInterface
	queue           QueueInterface
	logger          *slog.Logger
	isShuttingDown  func() bool
}

func NewHealthHandler(db DatabaseInterface, queue QueueInterface, logger *slog.Logger, isShuttingDown func() bool) *HealthHandler {
	return &HealthHandler{
		db:             db,
		queue:          queue,
		logger:         logger,
		isShuttingDown: isShuttingDown,
	}
}

func (hh *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
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

func (hh *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
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

	// Check if server is shutting down first
	if hh.isShuttingDown != nil && hh.isShuttingDown() {
		checks["shutdown"] = "shutting_down"
		allHealthy = false
		hh.logger.InfoContext(r.Context(), "readiness check failed: server is shutting down")
	} else {
		checks["shutdown"] = "running"
	}

	// Only check dependencies if not shutting down
	if allHealthy {
		if err := hh.db.HealthCheck(r.Context()); err != nil {
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

func (hh *HealthHandler) Stats(w http.ResponseWriter, r *http.Request) {
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

	totalJobs, err := hh.db.Jobs().Count(r.Context())
	if err != nil {
		hh.logger.Error("failed to count total jobs", "error", err)
		totalJobs = -1
	}

	pendingJobs, err := hh.db.Jobs().CountByStatus(r.Context(), "pending")
	if err != nil {
		hh.logger.Error("failed to count pending jobs", "error", err)
		pendingJobs = -1
	}

	runningJobs, err := hh.db.Jobs().CountByStatus(r.Context(), "running")
	if err != nil {
		hh.logger.Error("failed to count running jobs", "error", err)
		runningJobs = -1
	}

	succeededJobs, err := hh.db.Jobs().CountByStatus(r.Context(), "succeeded")
	if err != nil {
		hh.logger.Error("failed to count succeeded jobs", "error", err)
		succeededJobs = -1
	}

	failedJobs, err := hh.db.Jobs().CountByStatus(r.Context(), "failed")
	if err != nil {
		hh.logger.Error("failed to count failed jobs", "error", err)
		failedJobs = -1
	}

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

func (hh *HealthHandler) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		hh.logger.Error("failed to encode JSON response", "error", err)
	}
}

func (hh *HealthHandler) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := map[string]interface{}{
		"error":     message,
		"status":    statusCode,
		"timestamp": time.Now().Unix(),
	}

	json.NewEncoder(w).Encode(errorResponse)
}
