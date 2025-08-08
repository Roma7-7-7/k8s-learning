package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rsav/k8s-learning/internal/models"
)

type JobStore struct {
	db *sqlx.DB
}

func NewJobStore(db *sqlx.DB) *JobStore {
	return &JobStore{db: db}
}

func (js *JobStore) Create(ctx context.Context, job *models.Job) error {
	query := `
		INSERT INTO jobs (
			id, original_filename, file_path, processing_type, 
			parameters, status, created_at
		) VALUES (
			:id, :original_filename, :file_path, :processing_type, 
			:parameters, :status, :created_at
		)`

	_, err := js.db.NamedExecContext(ctx, query, job)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	return nil
}

func (js *JobStore) GetByID(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	var job models.Job
	query := `
		SELECT id, original_filename, file_path, processing_type, 
			   parameters, status, result_path, error_message, 
			   created_at, started_at, completed_at, worker_id
		FROM jobs 
		WHERE id = $1`

	err := js.db.GetContext(ctx, &job, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return &job, nil
}

func (js *JobStore) List(ctx context.Context, req models.ListJobsRequest) ([]*models.Job, error) {
	req.SetDefaults()

	query := `
		SELECT id, original_filename, file_path, processing_type, 
			   parameters, status, result_path, error_message, 
			   created_at, started_at, completed_at, worker_id
		FROM jobs 
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2`

	args := []interface{}{req.Limit, req.Offset}

	rows, err := js.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		var job models.Job
		if err := rows.StructScan(&job); err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, &job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return jobs, nil
}

func (js *JobStore) UpdateStatus(ctx context.Context, id uuid.UUID, status models.JobStatus, workerID *string) error {
	now := time.Now()
	var query string
	var args []interface{}

	switch status {
	case models.JobStatusRunning:
		query = `
			UPDATE jobs 
			SET status = $1, started_at = $2, worker_id = $3
			WHERE id = $4`
		args = []interface{}{status, now, workerID, id}
	case models.JobStatusSucceeded, models.JobStatusFailed:
		query = `
			UPDATE jobs 
			SET status = $1, completed_at = $2
			WHERE id = $3`
		args = []interface{}{status, now, id}
	default:
		query = `
			UPDATE jobs 
			SET status = $1
			WHERE id = $2`
		args = []interface{}{status, id}
	}

	result, err := js.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

func (js *JobStore) UpdateResult(ctx context.Context, id uuid.UUID, resultPath string) error {
	query := `
		UPDATE jobs 
		SET result_path = $1, status = $2, completed_at = $3
		WHERE id = $4`

	result, err := js.db.ExecContext(ctx, query, resultPath, models.JobStatusSucceeded, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update job result: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

func (js *JobStore) UpdateError(ctx context.Context, id uuid.UUID, errorMessage string) error {
	query := `
		UPDATE jobs 
		SET error_message = $1, status = $2, completed_at = $3
		WHERE id = $4`

	result, err := js.db.ExecContext(ctx, query, errorMessage, models.JobStatusFailed, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update job error: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

func (js *JobStore) Count(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM jobs`

	err := js.db.GetContext(ctx, &count, query)
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	return count, nil
}

func (js *JobStore) CountByStatus(ctx context.Context, status models.JobStatus) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM jobs WHERE status = $1`

	err := js.db.GetContext(ctx, &count, query, status)
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs by status: %w", err)
	}

	return count, nil
}
