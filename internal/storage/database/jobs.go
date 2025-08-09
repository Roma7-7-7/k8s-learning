package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type (
	JobStatus      string
	ProcessingType string

	Job struct {
		ID               uuid.UUID      `json:"id" db:"id"`
		OriginalFilename string         `json:"original_filename" db:"original_filename"`
		FilePath         string         `json:"file_path" db:"file_path"`
		ProcessingType   ProcessingType `json:"processing_type" db:"processing_type"`
		Parameters       JSONB          `json:"parameters" db:"parameters"`
		Status           JobStatus      `json:"status" db:"status"`
		ResultPath       string         `json:"result_path,omitempty" db:"result_path"`
		ErrorMessage     string         `json:"error_message,omitempty" db:"error_message"`
		CreatedAt        time.Time      `json:"created_at" db:"created_at"`
		StartedAt        *time.Time     `json:"started_at,omitempty" db:"started_at"`
		CompletedAt      *time.Time     `json:"completed_at,omitempty" db:"completed_at"`
		WorkerID         string         `json:"worker_id,omitempty" db:"worker_id"`
	}
)

const (
	ProcessingTypeWordCount ProcessingType = "wordcount"
	ProcessingTypeLineCount ProcessingType = "linecount"
	ProcessingTypeUppercase ProcessingType = "uppercase"
	ProcessingTypeLowercase ProcessingType = "lowercase"
	ProcessingTypeReplace   ProcessingType = "replace"
	ProcessingTypeExtract   ProcessingType = "extract"
)

func (p ProcessingType) String() string {
	return string(p)
}

//nolint:gochecknoglobals // processingTypes is a map of all valid processing types.
var processingTypes = map[string]ProcessingType{
	ProcessingTypeWordCount.String(): ProcessingTypeWordCount,
	ProcessingTypeLineCount.String(): ProcessingTypeLineCount,
	ProcessingTypeUppercase.String(): ProcessingTypeUppercase,
	ProcessingTypeLowercase.String(): ProcessingTypeLowercase,
	ProcessingTypeReplace.String():   ProcessingTypeReplace,
	ProcessingTypeExtract.String():   ProcessingTypeExtract,
}

func ToProcessingType(pt string) (ProcessingType, bool) {
	res, ok := processingTypes[pt]
	return res, ok
}

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusSucceeded JobStatus = "succeeded"
	JobStatusFailed    JobStatus = "failed"
)

func (s JobStatus) String() string {
	return string(s)
}

//nolint:gochecknoglobals // jobStatuses is a map of all valid job statuses.
var jobStatuses = map[string]JobStatus{
	JobStatusPending.String():   JobStatusPending,
	JobStatusRunning.String():   JobStatusRunning,
	JobStatusSucceeded.String(): JobStatusSucceeded,
	JobStatusFailed.String():    JobStatusFailed,
}

func ToJobStatus(status string) (JobStatus, bool) {
	res, ok := jobStatuses[status]
	return res, ok
}

type GetJobsFilter struct {
	Status JobStatus
	Limit  int
	Offset int
}

func (r *Repository) GetJobs(ctx context.Context, req GetJobsFilter) ([]*Job, error) {
	if req.Limit <= 0 {
		req.Limit = 100 // Default limit
	}
	if req.Offset < 0 {
		req.Offset = 0 // Default offset
	}

	query := `
		SELECT id, original_filename, file_path, processing_type, 
			   parameters, status, COALESCE(result_path, '') as result_path, 
			   COALESCE(error_message, '') as error_message, 
			   created_at, started_at, completed_at, COALESCE(worker_id, '') as worker_id
		FROM jobs 
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2`

	args := []interface{}{req.Limit, req.Offset}

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		var job Job
		if err := rows.StructScan(&job); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, &job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return jobs, nil
}

func (r *Repository) GetJobByID(ctx context.Context, id uuid.UUID) (*Job, error) {
	var job Job
	query := `
		SELECT id, original_filename, file_path, processing_type, 
			   parameters, status, COALESCE(result_path, '') as result_path, 
			   COALESCE(error_message, '') as error_message, 
			   created_at, started_at, completed_at, COALESCE(worker_id, '') as worker_id
		FROM jobs 
		WHERE id = $1`

	err := r.db.GetContext(ctx, &job, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("job not found: %s", id)
		}
		return nil, fmt.Errorf("get job: %w", err)
	}

	return &job, nil
}

func (r *Repository) CountJobs(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM jobs`

	err := r.db.GetContext(ctx, &count, query)
	if err != nil {
		return 0, fmt.Errorf("count jobs: %w", err)
	}

	return count, nil
}

func (r *Repository) CountJobsByStatus(ctx context.Context, status JobStatus) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM jobs WHERE status = $1`

	err := r.db.GetContext(ctx, &count, query, status)
	if err != nil {
		return 0, fmt.Errorf("count jobs by status: %w", err)
	}

	return count, nil
}

func (r *Repository) CreateJob(ctx context.Context, job *Job) error {
	query := `
		INSERT INTO jobs (
			id, original_filename, file_path, processing_type, 
			parameters, status, created_at
		) VALUES (
			:id, :original_filename, :file_path, :processing_type, 
			:parameters, :status, :created_at
		)`

	_, err := r.db.NamedExecContext(ctx, query, job)
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}

	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status JobStatus, workerID *string) error {
	now := time.Now()
	var query string
	var args []interface{}

	switch status {
	case JobStatusRunning:
		query = `
			UPDATE jobs 
			SET status = $1, started_at = $2, worker_id = $3
			WHERE id = $4`
		args = []interface{}{status, now, workerID, id}
	case JobStatusSucceeded, JobStatusFailed:
		query = `
			UPDATE jobs 
			SET status = $1, completed_at = $2
			WHERE id = $3`
		args = []interface{}{status, now, id}
	case JobStatusPending:
		query = `
			UPDATE jobs 
			SET status = $1
			WHERE id = $2`
		args = []interface{}{status, id}
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

func (r *Repository) UpdateResult(ctx context.Context, id uuid.UUID, resultPath string) error {
	query := `
		UPDATE jobs 
		SET result_path = $1, status = $2, completed_at = $3
		WHERE id = $4`

	result, err := r.db.ExecContext(ctx, query, resultPath, JobStatusSucceeded, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update job result: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

func (r *Repository) UpdateError(ctx context.Context, id uuid.UUID, errorMessage string) error {
	query := `
		UPDATE jobs 
		SET error_message = $1, status = $2, completed_at = $3
		WHERE id = $4`

	result, err := r.db.ExecContext(ctx, query, errorMessage, JobStatusFailed, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update job error: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}
