package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rsav/k8s-learning/internal/api/metrics"
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
		DelayMS          int            `json:"delay_ms"`
		ErrorMessage     string         `json:"error_message,omitempty"`
		CreatedAt        time.Time      `json:"created_at"`
		StartedAt        *time.Time     `json:"started_at,omitempty"`
		CompletedAt      *time.Time     `json:"completed_at,omitempty"`
		WorkerID         string         `json:"worker_id,omitempty"`
	}

	errorResponse struct {
		Error     string `json:"error"`
		ErrorCode string `json:"error_code"`
		Status    int    `json:"status"`
		Timestamp int64  `json:"timestamp"`
	}

	Job struct {
		repo      Repository
		queue     Queue
		fileStore FileStorage
		log       *slog.Logger
	}
)

const (
	memoryLimit = 32 << 20 // 32 MB limit
	maxDelayMS  = 60000    // 1 minute max delay
)

func NewJob(repo Repository, queue Queue, fileStore FileStorage, logger *slog.Logger) *Job {
	return &Job{
		repo:      repo,
		queue:     queue,
		fileStore: fileStore,
		log:       logger,
	}
}

func (jh *Job) CreateJob(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(memoryLimit); err != nil {
		jh.log.Error("failed to parse multipart form", "error", err)
		jh.writeErrorWithCode(w, http.StatusBadRequest, "failed to parse form", "FORM_PARSE_ERROR")
		return
	}

	header, err := jh.validateAndExtractFile(w, r)
	if err != nil {
		return // error already written in validateAndExtractFile
	}

	processingType, parameters, delayMS, err := jh.validateJobParameters(w, r)
	if err != nil {
		return // error already written in validateJobParameters
	}

	fileInfo, err := jh.fileStore.SaveUploadedFile(header)
	if err != nil {
		jh.log.Error("failed to save uploaded file", "error", err)
		jh.writeErrorWithCode(w, http.StatusInternalServerError, "failed to save file", "FILE_SAVE_ERROR")
		return
	}

	job := &database.Job{
		ID:               uuid.New(),
		OriginalFilename: fileInfo.OriginalName,
		FilePath:         fileInfo.StoredPath,
		ProcessingType:   processingType,
		Parameters:       database.JSONB(parameters),
		Status:           database.JobStatusPending,
		DelayMS:          delayMS,
		CreatedAt:        time.Now(),
	}

	if err := jh.repo.CreateJob(r.Context(), job); err != nil {
		jh.log.Error("failed to create job in database", "error", err, "job_id", job.ID)
		if err := jh.fileStore.DeleteFile(fileInfo.StoredPath); err != nil {
			jh.log.Error("failed to delete uploaded file after job creation failure", "error", err, "file_path", fileInfo.StoredPath)
		}
		jh.writeErrorWithCode(w, http.StatusInternalServerError, "failed to create job", "JOB_CREATE_ERROR")
		return
	}

	queueMessage := queue.SubmitJobMessage{
		JobID:          job.ID,
		FilePath:       job.FilePath,
		ProcessingType: job.ProcessingType,
		Parameters:     map[string]any(job.Parameters),
		Priority:       1,
		DelayMS:        job.DelayMS,
	}

	if err := jh.queue.PublishJob(r.Context(), queueMessage); err != nil {
		jh.log.Error("failed to publish job to queue", "error", err, "job_id", job.ID)
		jh.writeErrorWithCode(w, http.StatusInternalServerError, "failed to queue job", "QUEUE_ERROR")
		return
	}

	// Track metrics
	metrics.JobsCreatedTotal.Inc()
	priority := strconv.Itoa(queueMessage.Priority)
	metrics.JobsQueuedTotal.WithLabelValues(priority).Inc()

	jh.log.Info("job created successfully",
		"job_id", job.ID,
		"processing_type", job.ProcessingType,
		"filename", job.OriginalFilename)

	jh.writeJSON(w, http.StatusCreated, jobToResponse(job))
}

func (jh *Job) GetJob(w http.ResponseWriter, r *http.Request) {
	jobIDStr := r.PathValue("id")
	if jobIDStr == "" {
		jh.writeErrorWithCode(w, http.StatusBadRequest, "job ID is required", "JOB_ID_MISSING")
		return
	}

	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		jh.writeErrorWithCode(w, http.StatusBadRequest, "invalid job ID format", "INVALID_JOB_ID")
		return
	}

	job, err := jh.repo.GetJobByID(r.Context(), jobID)
	if err != nil {
		jh.log.Error("failed to get job", "error", err, "job_id", jobID)
		jh.writeErrorWithCode(w, http.StatusNotFound, "job not found", "JOB_NOT_FOUND")
		return
	}

	jh.writeJSON(w, http.StatusOK, jobToResponse(job))
}

func (jh *Job) ListJobs(w http.ResponseWriter, r *http.Request) {
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
			jh.writeErrorWithCode(w, http.StatusBadRequest, "invalid job status", "INVALID_STATUS_FILTER")
			return
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if filter.Limit, err = strconv.Atoi(limitStr); err != nil {
			jh.writeErrorWithCode(w, http.StatusBadRequest, "invalid limit parameter", "INVALID_LIMIT")
			return
		}
		if filter.Limit < 0 {
			jh.writeErrorWithCode(w, http.StatusBadRequest, "limit cannot be negative", "INVALID_LIMIT")
			return
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if filter.Offset, err = strconv.Atoi(offsetStr); err != nil {
			jh.writeErrorWithCode(w, http.StatusBadRequest, "invalid offset parameter", "INVALID_OFFSET")
			return
		}
		if filter.Offset < 0 {
			jh.writeErrorWithCode(w, http.StatusBadRequest, "offset cannot be negative", "INVALID_OFFSET")
			return
		}
	}

	jobs, err := jh.repo.GetJobs(r.Context(), filter)
	if err != nil {
		jh.log.Error("failed to list jobs", "error", err)
		jh.writeErrorWithCode(w, http.StatusInternalServerError, "failed to list jobs", "JOB_LIST_ERROR")
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
	jobIDStr := r.PathValue("id")
	if jobIDStr == "" {
		jh.writeErrorWithCode(w, http.StatusBadRequest, "job ID is required", "JOB_ID_MISSING")
		return
	}

	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		jh.writeErrorWithCode(w, http.StatusBadRequest, "invalid job ID format", "INVALID_JOB_ID")
		return
	}

	job, err := jh.repo.GetJobByID(r.Context(), jobID)
	if err != nil {
		jh.log.Error("failed to get job", "error", err, "job_id", jobID)
		jh.writeErrorWithCode(w, http.StatusNotFound, "job not found", "JOB_NOT_FOUND")
		return
	}

	if job.Status != database.JobStatusSucceeded {
		jh.writeErrorWithCode(w, http.StatusBadRequest,
			fmt.Sprintf("job is not completed successfully, current status: %s", job.Status), "JOB_NOT_READY")
		return
	}

	if job.ResultPath == "" {
		jh.writeErrorWithCode(w, http.StatusNotFound, "result file not found", "RESULT_FILE_MISSING")
		return
	}

	if !jh.fileStore.FileExists(job.ResultPath) {
		jh.writeErrorWithCode(w, http.StatusNotFound, "result file not found on disk", "RESULT_FILE_NOT_ON_DISK")
		return
	}

	content, err := jh.fileStore.ReadFile(job.ResultPath)
	if err != nil {
		jh.log.Error("failed to read result file", "error", err, "job_id", jobID)
		jh.writeErrorWithCode(w, http.StatusInternalServerError, "failed to read result file", "RESULT_FILE_READ_ERROR")
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

func (jh *Job) writeErrorWithCode(w http.ResponseWriter, statusCode int, message, errorCode string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResp := errorResponse{
		Error:     message,
		ErrorCode: errorCode,
		Status:    statusCode,
		Timestamp: time.Now().Unix(),
	}

	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		jh.log.Error("failed to encode error response", "error", err, "status_code", statusCode, "message", message, "error_code", errorCode)
		return
	}
}

func (jh *Job) isValidTextFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	validExtensions := []string{".txt", ".md", ".csv", ".json", ".xml", ".log"}

	for _, validExt := range validExtensions {
		if ext == validExt {
			return true
		}
	}

	return false
}

func (jh *Job) validateAndExtractFile(w http.ResponseWriter, r *http.Request) (*multipart.FileHeader, error) {
	file, header, err := r.FormFile("file")
	if err != nil {
		jh.log.Error("failed to get file from form", "error", err)
		jh.writeErrorWithCode(w, http.StatusBadRequest, "file is required", "FILE_MISSING")
		return nil, err
	}
	_ = file.Close()

	// Validate file type at handler level
	if !jh.isValidTextFile(header.Filename) {
		jh.writeErrorWithCode(w, http.StatusBadRequest,
			"invalid file type: only text files (.txt, .md, .csv, .json, .xml, .log) are allowed",
			"INVALID_FILE_TYPE")
		return nil, errors.New("invalid file type")
	}

	// Check file size
	if header.Size > jh.fileStore.GetMaxFileSize() {
		jh.writeErrorWithCode(w, http.StatusBadRequest,
			fmt.Sprintf("file size %d exceeds maximum allowed size %d",
				header.Size, jh.fileStore.GetMaxFileSize()),
			"FILE_TOO_LARGE")
		return nil, errors.New("file too large")
	}

	return header, nil
}

func (jh *Job) validateJobParameters(w http.ResponseWriter, r *http.Request) (database.ProcessingType, map[string]any, int, error) {
	processingType, ok := database.ToProcessingType(r.FormValue("processing_type"))
	if !ok {
		jh.writeErrorWithCode(w, http.StatusBadRequest, "invalid processing_type", "INVALID_PROCESSING_TYPE")
		return "", nil, 0, errors.New("invalid processing type")
	}

	var parameters map[string]any
	if parametersStr := r.FormValue("parameters"); parametersStr != "" {
		if err := json.Unmarshal([]byte(parametersStr), &parameters); err != nil {
			jh.log.Error("failed to parse parameters", "error", err)
			jh.writeErrorWithCode(w, http.StatusBadRequest, "invalid parameters JSON", "INVALID_PARAMETERS_JSON")
			return "", nil, 0, err
		}
	} else {
		parameters = make(map[string]any)
	}

	if err := validateProcessingTypeAndParams(processingType, parameters); err != nil {
		jh.writeErrorWithCode(w, http.StatusBadRequest, err.Error(), "INVALID_PARAMETERS")
		return "", nil, 0, err
	}

	var delayMS int
	if delayStr := r.FormValue("delay_ms"); delayStr != "" {
		var err error
		delayMS, err = strconv.Atoi(delayStr)
		if err != nil || delayMS < 0 {
			jh.writeErrorWithCode(w, http.StatusBadRequest, "invalid delay_ms parameter", "INVALID_DELAY_MS")
			return "", nil, 0, err
		}
		if delayMS > maxDelayMS {
			jh.writeErrorWithCode(w, http.StatusBadRequest, fmt.Sprintf("delay_ms cannot exceed %d milliseconds", maxDelayMS), "DELAY_MS_TOO_LARGE")
			return "", nil, 0, errors.New("delay too large")
		}
	}

	return processingType, parameters, delayMS, nil
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
		DelayMS:          j.DelayMS,
		ErrorMessage:     j.ErrorMessage,
		CreatedAt:        j.CreatedAt,
		StartedAt:        j.StartedAt,
		CompletedAt:      j.CompletedAt,
		WorkerID:         j.WorkerID,
	}
}
