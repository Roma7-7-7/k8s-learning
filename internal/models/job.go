package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusSucceeded JobStatus = "succeeded"
	JobStatusFailed    JobStatus = "failed"
)

func (js JobStatus) String() string {
	return string(js)
}

func (js JobStatus) Valid() bool {
	switch js {
	case JobStatusPending, JobStatusRunning, JobStatusSucceeded, JobStatusFailed:
		return true
	}
	return false
}

type ProcessingType string

const (
	ProcessingWordCount ProcessingType = "wordcount"
	ProcessingLineCount ProcessingType = "linecount"
	ProcessingUppercase ProcessingType = "uppercase"
	ProcessingLowercase ProcessingType = "lowercase"
	ProcessingReplace   ProcessingType = "replace"
	ProcessingExtract   ProcessingType = "extract"
)

func (pt ProcessingType) String() string {
	return string(pt)
}

func (pt ProcessingType) Valid() bool {
	switch pt {
	case ProcessingWordCount, ProcessingLineCount, ProcessingUppercase,
		ProcessingLowercase, ProcessingReplace, ProcessingExtract:
		return true
	}
	return false
}

type ProcessingParams map[string]interface{}

func (pp ProcessingParams) Value() (driver.Value, error) {
	if pp == nil {
		return nil, nil
	}
	return json.Marshal(pp)
}

func (pp *ProcessingParams) Scan(value interface{}) error {
	if value == nil {
		*pp = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, pp)
	case string:
		return json.Unmarshal([]byte(v), pp)
	default:
		return fmt.Errorf("cannot scan %T into ProcessingParams", value)
	}
}

type Job struct {
	ID               uuid.UUID        `json:"id" db:"id"`
	OriginalFilename string           `json:"original_filename" db:"original_filename"`
	FilePath         string           `json:"file_path" db:"file_path"`
	ProcessingType   ProcessingType   `json:"processing_type" db:"processing_type"`
	Parameters       ProcessingParams `json:"parameters" db:"parameters"`
	Status           JobStatus        `json:"status" db:"status"`
	ResultPath       *string          `json:"result_path,omitempty" db:"result_path"`
	ErrorMessage     *string          `json:"error_message,omitempty" db:"error_message"`
	CreatedAt        time.Time        `json:"created_at" db:"created_at"`
	StartedAt        *time.Time       `json:"started_at,omitempty" db:"started_at"`
	CompletedAt      *time.Time       `json:"completed_at,omitempty" db:"completed_at"`
	WorkerID         *string          `json:"worker_id,omitempty" db:"worker_id"`
}

type CreateJobRequest struct {
	ProcessingType ProcessingType   `json:"processing_type"`
	Parameters     ProcessingParams `json:"parameters"`
}

func (r CreateJobRequest) Validate() error {
	if !r.ProcessingType.Valid() {
		return fmt.Errorf("invalid processing type: %s", r.ProcessingType)
	}

	switch r.ProcessingType {
	case ProcessingReplace:
		find, ok := r.Parameters["find"]
		if !ok || find == "" {
			return fmt.Errorf("replace operation requires 'find' parameter")
		}
		replaceWith, ok := r.Parameters["replace_with"]
		if !ok {
			return fmt.Errorf("replace operation requires 'replace_with' parameter")
		}
		if _, ok := replaceWith.(string); !ok {
			return fmt.Errorf("'replace_with' parameter must be a string")
		}
	case ProcessingExtract:
		pattern, ok := r.Parameters["pattern"]
		if !ok || pattern == "" {
			return fmt.Errorf("extract operation requires 'pattern' parameter")
		}
		if _, ok := pattern.(string); !ok {
			return fmt.Errorf("'pattern' parameter must be a string")
		}
	}

	return nil
}

type JobResponse struct {
	ID               uuid.UUID        `json:"id"`
	OriginalFilename string           `json:"original_filename"`
	ProcessingType   ProcessingType   `json:"processing_type"`
	Parameters       ProcessingParams `json:"parameters"`
	Status           JobStatus        `json:"status"`
	ErrorMessage     *string          `json:"error_message,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	StartedAt        *time.Time       `json:"started_at,omitempty"`
	CompletedAt      *time.Time       `json:"completed_at,omitempty"`
	WorkerID         *string          `json:"worker_id,omitempty"`
}

func (j *Job) ToResponse() JobResponse {
	return JobResponse{
		ID:               j.ID,
		OriginalFilename: j.OriginalFilename,
		ProcessingType:   j.ProcessingType,
		Parameters:       j.Parameters,
		Status:           j.Status,
		ErrorMessage:     j.ErrorMessage,
		CreatedAt:        j.CreatedAt,
		StartedAt:        j.StartedAt,
		CompletedAt:      j.CompletedAt,
		WorkerID:         j.WorkerID,
	}
}

type ListJobsRequest struct {
	Status ProcessingType `json:"status,omitempty"`
	Limit  int            `json:"limit,omitempty"`
	Offset int            `json:"offset,omitempty"`
}

func (r ListJobsRequest) Validate() error {
	if r.Limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if r.Offset < 0 {
		return fmt.Errorf("offset cannot be negative")
	}
	if r.Limit > 100 {
		return fmt.Errorf("limit cannot exceed 100")
	}
	return nil
}

func (r *ListJobsRequest) SetDefaults() {
	if r.Limit == 0 {
		r.Limit = 20
	}
}

type QueueMessage struct {
	JobID          uuid.UUID        `json:"job_id"`
	FilePath       string           `json:"file_path"`
	ProcessingType ProcessingType   `json:"processing_type"`
	Parameters     ProcessingParams `json:"parameters"`
	Priority       int              `json:"priority"`
}
