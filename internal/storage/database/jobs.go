package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
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
		DelayMS          int            `json:"delay_ms" db:"delay_ms"`
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

// psql is a Squirrel query builder configured for PostgreSQL.
//
//nolint:gochecknoglobals // psql is a stateless query builder, safe to use as global
var psql = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

// jobSelectColumns defines the standard columns returned when querying jobs.
// Uses COALESCE to handle NULL values gracefully by converting them to empty strings.
//
//nolint:gochecknoglobals // jobSelectColumns is a read-only slice, safe to use as global
var jobSelectColumns = []string{
	"id",
	"original_filename",
	"file_path",
	"processing_type",
	"parameters",
	"status",
	"delay_ms",
	"COALESCE(result_path, '') as result_path",
	"COALESCE(error_message, '') as error_message",
	"created_at",
	"started_at",
	"completed_at",
	"COALESCE(worker_id, '') as worker_id",
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

	query := psql.Select(jobSelectColumns...).
		From("jobs").
		OrderBy("created_at DESC").
		Limit(uint64(req.Limit)).
		Offset(uint64(req.Offset))

	// Add status filter if specified
	if req.Status != "" {
		query = query.Where(squirrel.Eq{"status": req.Status})
	}

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.QueryxContext(ctx, sqlQuery, args...)
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

	query, args, err := psql.Select(jobSelectColumns...).
		From("jobs").
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	err = r.db.GetContext(ctx, &job, query, args...)
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

	sqlQuery, args, err := psql.Select("COUNT(*)").From("jobs").ToSql()
	if err != nil {
		return 0, fmt.Errorf("build query: %w", err)
	}

	err = r.db.GetContext(ctx, &count, sqlQuery, args...)
	if err != nil {
		return 0, fmt.Errorf("count jobs: %w", err)
	}

	return count, nil
}

func (r *Repository) CountJobsByStatus(ctx context.Context, status JobStatus) (int, error) {
	var count int

	sqlQuery, args, err := psql.Select("COUNT(*)").
		From("jobs").
		Where(squirrel.Eq{"status": status}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build query: %w", err)
	}

	err = r.db.GetContext(ctx, &count, sqlQuery, args...)
	if err != nil {
		return 0, fmt.Errorf("count jobs by status: %w", err)
	}

	return count, nil
}

func (r *Repository) CreateJob(ctx context.Context, job *Job) error {
	sqlQuery, args, err := psql.Insert("jobs").
		Columns("id", "original_filename", "file_path", "processing_type",
			"parameters", "status", "delay_ms", "created_at").
		Values(job.ID, job.OriginalFilename, job.FilePath, job.ProcessingType,
			job.Parameters, job.Status, job.DelayMS, job.CreatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	_, err = r.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}

	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status JobStatus, workerID *string) error {
	now := time.Now()

	query := psql.Update("jobs").Where(squirrel.Eq{"id": id})

	switch status {
	case JobStatusRunning:
		query = query.Set("status", status).
			Set("started_at", now).
			Set("worker_id", workerID)
	case JobStatusSucceeded, JobStatusFailed:
		query = query.Set("status", status).
			Set("completed_at", now)
	case JobStatusPending:
		query = query.Set("status", status)
	}

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	result, err := r.db.ExecContext(ctx, sqlQuery, args...)
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
	sqlQuery, args, err := psql.Update("jobs").
		Set("result_path", resultPath).
		Set("status", JobStatusSucceeded).
		Set("completed_at", time.Now()).
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	result, err := r.db.ExecContext(ctx, sqlQuery, args...)
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
	sqlQuery, args, err := psql.Update("jobs").
		Set("error_message", errorMessage).
		Set("status", JobStatusFailed).
		Set("completed_at", time.Now()).
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	result, err := r.db.ExecContext(ctx, sqlQuery, args...)
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
