package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rsav/k8s-learning/internal/storage/database"
	"github.com/rsav/k8s-learning/internal/storage/queue"
)

type (
	jobResponse struct {
		ID               uuid.UUID      `json:"id"`
		OriginalFilename string         `json:"original_filename"`
		ProcessingType   string         `json:"processing_type"`
		Parameters       map[string]any `json:"parameters"`
		Status           string         `json:"status"`
		ErrorMessage     string         `json:"error_message,omitempty"`
		CreatedAt        time.Time      `json:"created_at"`
		StartedAt        *time.Time     `json:"started_at,omitempty"`
		CompletedAt      *time.Time     `json:"completed_at,omitempty"`
		WorkerID         string         `json:"worker_id,omitempty"`
	}
	Job struct {
		repo      Repository
		queue     Queue
		fileStore FileStorage
		log       *slog.Logger
	}
)

const memoryLimit = 32 << 20 // 32 MB limit

func NewJob(repo Repository, queue Queue, fileStore FileStorage, logger *slog.Logger) *Job {
	return &Job{
		repo:      repo,
		queue:     queue,
		fileStore: fileStore,
		log:       logger,
	}
}

func (jh *Job) CreateJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jh.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := r.ParseMultipartForm(memoryLimit); err != nil {
		jh.log.Error("failed to parse multipart form", "error", err)
		jh.writeError(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jh.log.Error("failed to get file from form", "error", err)
		jh.writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	_ = file.Close()

	processingType, ok := database.ToProcessingType(r.FormValue("processing_type"))
	if !ok {
		jh.writeError(w, http.StatusBadRequest, "invalid processing_type")
		return
	}

	var parameters map[string]any
	if parametersStr := r.FormValue("parameters"); parametersStr != "" {
		if err := json.Unmarshal([]byte(parametersStr), &parameters); err != nil {
			jh.log.Error("failed to parse parameters", "error", err)
			jh.writeError(w, http.StatusBadRequest, "invalid parameters JSON")
			return
		}
	} else {
		parameters = make(map[string]any)
	}

	if err := validateProcessingTypeAndParams(processingType, parameters); err != nil {
		jh.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	fileInfo, err := jh.fileStore.SaveUploadedFile(header)
	if err != nil {
		jh.log.Error("failed to save uploaded file", "error", err)
		jh.writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	job := &database.Job{
		ID:               uuid.New(),
		OriginalFilename: fileInfo.OriginalName,
		FilePath:         fileInfo.StoredPath,
		ProcessingType:   processingType,
		Parameters:       parameters,
		Status:           database.JobStatusPending,
		CreatedAt:        time.Now(),
	}

	if err := jh.repo.CreateJob(r.Context(), job); err != nil {
		jh.log.Error("failed to create job in database", "error", err, "job_id", job.ID)
		if err := jh.fileStore.DeleteFile(fileInfo.StoredPath); err != nil {
			jh.log.Error("failed to delete uploaded file after job creation failure", "error", err, "file_path", fileInfo.StoredPath)
		}
		jh.writeError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	queueMessage := queue.SubmitJobMessage{
		JobID:          job.ID,
		FilePath:       job.FilePath,
		ProcessingType: job.ProcessingType,
		Parameters:     job.Parameters,
		Priority:       1,
	}

	if err := jh.queue.PublishJob(r.Context(), queueMessage); err != nil {
		jh.log.Error("failed to publish job to queue", "error", err, "job_id", job.ID)
		jh.writeError(w, http.StatusInternalServerError, "failed to queue job")
		return
	}

	jh.log.Info("job created successfully",
		"job_id", job.ID,
		"processing_type", job.ProcessingType,
		"filename", job.OriginalFilename)

	jh.writeJSON(w, http.StatusCreated, jobToResponse(job))
}

func (jh *Job) GetJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jh.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	jobIDStr := getPathParam(r.URL.Path, "/api/v1/jobs/")
	if jobIDStr == "" {
		jh.writeError(w, http.StatusBadRequest, "job ID is required")
		return
	}

	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		jh.writeError(w, http.StatusBadRequest, "invalid job ID format")
		return
	}

	job, err := jh.repo.GetJobByID(r.Context(), jobID)
	if err != nil {
		jh.log.Error("failed to get job", "error", err, "job_id", jobID)
		jh.writeError(w, http.StatusNotFound, "job not found")
		return
	}

	jh.writeJSON(w, http.StatusOK, jobToResponse(job))
}

func (jh *Job) ListJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jh.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var err error
	//nolint:mnd // we need to initialize the filter with default values
	filter := database.GetJobsFilter{
		Limit:  100,
		Offset: 0,
	}

	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		var ok bool
		filter.Status, ok = database.ToJobStatus(statusStr)
		if !ok {
			jh.writeError(w, http.StatusBadRequest, "invalid job status")
			return
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if filter.Limit, err = strconv.Atoi(limitStr); err != nil {
			jh.writeError(w, http.StatusBadRequest, "invalid limit parameter")
			return
		}
		if filter.Limit < 0 {
			jh.writeError(w, http.StatusBadRequest, "limit cannot be negative")
			return
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if filter.Offset, err = strconv.Atoi(offsetStr); err != nil {
			jh.writeError(w, http.StatusBadRequest, "invalid offset parameter")
			return
		}
		if filter.Offset < 0 {
			jh.writeError(w, http.StatusBadRequest, "offset cannot be negative")
			return
		}
	}

	jobs, err := jh.repo.GetJobs(r.Context(), filter)
	if err != nil {
		jh.log.Error("failed to list jobs", "error", err)
		jh.writeError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}

	response := make([]jobResponse, len(jobs))
	for i, job := range jobs {
		response[i] = jobToResponse(job)
	}

	jh.writeJSON(w, http.StatusOK, map[string]interface{}{
		"jobs":   response,
		"limit":  filter.Limit,
		"offset": filter.Offset,
		"total":  len(response),
	})
}

func (jh *Job) GetJobResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jh.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	jobIDStr := getPathParam(r.URL.Path, "/api/v1/jobs/")
	if jobIDStr == "" {
		jh.writeError(w, http.StatusBadRequest, "job ID is required")
		return
	}

	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		jh.writeError(w, http.StatusBadRequest, "invalid job ID format")
		return
	}

	job, err := jh.repo.GetJobByID(r.Context(), jobID)
	if err != nil {
		jh.log.Error("failed to get job", "error", err, "job_id", jobID)
		jh.writeError(w, http.StatusNotFound, "job not found")
		return
	}

	if job.Status != database.JobStatusSucceeded {
		jh.writeError(w, http.StatusBadRequest,
			fmt.Sprintf("job is not completed successfully, current status: %s", job.Status))
		return
	}

	if job.ResultPath == "" {
		jh.writeError(w, http.StatusNotFound, "result file not found")
		return
	}

	if !jh.fileStore.FileExists(job.ResultPath) {
		jh.writeError(w, http.StatusNotFound, "result file not found on disk")
		return
	}

	content, err := jh.fileStore.ReadFile(job.ResultPath)
	if err != nil {
		jh.log.Error("failed to read result file", "error", err, "job_id", jobID)
		jh.writeError(w, http.StatusInternalServerError, "failed to read result file")
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"result_%s.txt\"", jobID))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(content); err != nil {
		jh.log.Error("failed to write result file to response", "error", err, "job_id", jobID)
	}
}

func (jh *Job) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		jh.log.Error("failed to encode JSON response", "error", err)
	}
}

func (jh *Job) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := map[string]interface{}{
		"error":     message,
		"status":    statusCode,
		"timestamp": time.Now().Unix(),
	}

	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		jh.log.Error("failed to encode error response", "error", err, "status_code", statusCode, "message", message)
		return
	}
}

func validateProcessingTypeAndParams(processingType database.ProcessingType, params map[string]any) error {
	switch processingType {
	case database.ProcessingTypeReplace:
		find, ok := params["find"]
		if !ok || find == "" {
			return errors.New("replace operation requires 'find' parameter")
		}
		replaceWith, ok := params["replace_with"]
		if !ok {
			return errors.New("replace operation requires 'replace_with' parameter")
		}
		if _, ok := replaceWith.(string); !ok {
			return errors.New("'replace_with' parameter must be a string")
		}
	case database.ProcessingTypeExtract:
		pattern, ok := params["pattern"]
		if !ok || pattern == "" {
			return errors.New("extract operation requires 'pattern' parameter")
		}
		if _, ok := pattern.(string); !ok {
			return errors.New("'pattern' parameter must be a string")
		}
	case database.ProcessingTypeWordCount, database.ProcessingTypeLineCount, database.ProcessingTypeUppercase, database.ProcessingTypeLowercase:
		// These processing types do not require additional parameters
	}
	return nil
}

func jobToResponse(j *database.Job) jobResponse {
	return jobResponse{
		ID:               j.ID,
		OriginalFilename: j.OriginalFilename,
		ProcessingType:   string(j.ProcessingType),
		Parameters:       j.Parameters,
		Status:           string(j.Status),
		ErrorMessage:     j.ErrorMessage,
		CreatedAt:        j.CreatedAt,
		StartedAt:        j.StartedAt,
		CompletedAt:      j.CompletedAt,
		WorkerID:         j.WorkerID,
	}
}

func getPathParam(path, prefix string) string {
	if len(path) <= len(prefix) {
		return ""
	}

	param := path[len(prefix):]

	if slashIndex := findChar(param, '/'); slashIndex != -1 {
		param = param[:slashIndex]
	}

	return param
}

func findChar(s string, c rune) int {
	for i, r := range s {
		if r == c {
			return i
		}
	}
	return -1
}
