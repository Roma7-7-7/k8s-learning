package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rsav/k8s-learning/internal/models"
)

type JobHandler struct {
	db        DatabaseInterface
	queue     QueueInterface
	fileStore FileStorageInterface
	logger    *slog.Logger
}

func NewJobHandler(db DatabaseInterface, queue QueueInterface, fileStore FileStorageInterface, logger *slog.Logger) *JobHandler {
	return &JobHandler{
		db:        db,
		queue:     queue,
		fileStore: fileStore,
		logger:    logger,
	}
}

func (jh *JobHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jh.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		jh.logger.Error("failed to parse multipart form", "error", err)
		jh.writeError(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jh.logger.Error("failed to get file from form", "error", err)
		jh.writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	file.Close()

	processingTypeStr := r.FormValue("processing_type")
	if processingTypeStr == "" {
		jh.writeError(w, http.StatusBadRequest, "processing_type is required")
		return
	}

	processingType := models.ProcessingType(processingTypeStr)
	if !processingType.Valid() {
		jh.writeError(w, http.StatusBadRequest, "invalid processing_type")
		return
	}

	var parameters models.ProcessingParams
	if parametersStr := r.FormValue("parameters"); parametersStr != "" {
		if err := json.Unmarshal([]byte(parametersStr), &parameters); err != nil {
			jh.logger.Error("failed to parse parameters", "error", err)
			jh.writeError(w, http.StatusBadRequest, "invalid parameters JSON")
			return
		}
	} else {
		parameters = make(models.ProcessingParams)
	}

	createReq := models.CreateJobRequest{
		ProcessingType: processingType,
		Parameters:     parameters,
	}

	if err := createReq.Validate(); err != nil {
		jh.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	fileInfo, err := jh.fileStore.SaveUploadedFile(header)
	if err != nil {
		jh.logger.Error("failed to save uploaded file", "error", err)
		jh.writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	job := &models.Job{
		ID:               uuid.New(),
		OriginalFilename: fileInfo.OriginalName,
		FilePath:         fileInfo.StoredPath,
		ProcessingType:   createReq.ProcessingType,
		Parameters:       createReq.Parameters,
		Status:           models.JobStatusPending,
		CreatedAt:        time.Now(),
	}

	if err := jh.db.Jobs().Create(r.Context(), job); err != nil {
		jh.logger.Error("failed to create job in database", "error", err, "job_id", job.ID)
		jh.fileStore.DeleteFile(fileInfo.StoredPath)
		jh.writeError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	queueMessage := models.QueueMessage{
		JobID:          job.ID,
		FilePath:       job.FilePath,
		ProcessingType: job.ProcessingType,
		Parameters:     job.Parameters,
		Priority:       1,
	}

	if err := jh.queue.PublishJob(r.Context(), queueMessage); err != nil {
		jh.logger.Error("failed to publish job to queue", "error", err, "job_id", job.ID)
		jh.writeError(w, http.StatusInternalServerError, "failed to queue job")
		return
	}

	jh.logger.Info("job created successfully",
		"job_id", job.ID,
		"processing_type", job.ProcessingType,
		"filename", job.OriginalFilename)

	jh.writeJSON(w, http.StatusCreated, job.ToResponse())
}

func (jh *JobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
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

	job, err := jh.db.Jobs().GetByID(r.Context(), jobID)
	if err != nil {
		jh.logger.Error("failed to get job", "error", err, "job_id", jobID)
		jh.writeError(w, http.StatusNotFound, "job not found")
		return
	}

	jh.writeJSON(w, http.StatusOK, job.ToResponse())
}

func (jh *JobHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jh.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req models.ListJobsRequest

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			req.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			req.Offset = offset
		}
	}

	if err := req.Validate(); err != nil {
		jh.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	jobs, err := jh.db.Jobs().List(r.Context(), req)
	if err != nil {
		jh.logger.Error("failed to list jobs", "error", err)
		jh.writeError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}

	response := make([]models.JobResponse, len(jobs))
	for i, job := range jobs {
		response[i] = job.ToResponse()
	}

	jh.writeJSON(w, http.StatusOK, map[string]interface{}{
		"jobs":   response,
		"limit":  req.Limit,
		"offset": req.Offset,
		"total":  len(response),
	})
}

func (jh *JobHandler) GetJobResult(w http.ResponseWriter, r *http.Request) {
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

	job, err := jh.db.Jobs().GetByID(r.Context(), jobID)
	if err != nil {
		jh.logger.Error("failed to get job", "error", err, "job_id", jobID)
		jh.writeError(w, http.StatusNotFound, "job not found")
		return
	}

	if job.Status != models.JobStatusSucceeded {
		jh.writeError(w, http.StatusBadRequest,
			fmt.Sprintf("job is not completed successfully, current status: %s", job.Status))
		return
	}

	if job.ResultPath == nil {
		jh.writeError(w, http.StatusNotFound, "result file not found")
		return
	}

	if !jh.fileStore.FileExists(*job.ResultPath) {
		jh.writeError(w, http.StatusNotFound, "result file not found on disk")
		return
	}

	content, err := jh.fileStore.ReadFile(*job.ResultPath)
	if err != nil {
		jh.logger.Error("failed to read result file", "error", err, "job_id", jobID)
		jh.writeError(w, http.StatusInternalServerError, "failed to read result file")
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"result_%s.txt\"", jobID))
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func (jh *JobHandler) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		jh.logger.Error("failed to encode JSON response", "error", err)
	}
}

func (jh *JobHandler) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := map[string]interface{}{
		"error":     message,
		"status":    statusCode,
		"timestamp": time.Now().Unix(),
	}

	json.NewEncoder(w).Encode(errorResponse)
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
