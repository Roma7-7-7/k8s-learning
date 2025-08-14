package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/rsav/k8s-learning/internal/storage/database"
	"github.com/rsav/k8s-learning/internal/storage/queue"
)

type JobConsumer interface {
	ConsumeJob(ctx context.Context, timeout time.Duration) (*queue.SubmitJobMessage, error)
	SetWorkerHeartbeat(ctx context.Context, workerID string) error
	PublishToFailedQueue(ctx context.Context, message queue.SubmitJobMessage, errorMsg string) error
	HealthCheck(ctx context.Context) error
	Close() error
}

type ProcessingJob struct {
	JobID          string
	FilePath       string
	ProcessingType database.ProcessingType
	Parameters     map[string]any
	DelayMS        int
}

// ProcessingError represents an error that occurred during job processing.
// It contains both the error message and additional context that can be stored in the database.
type ProcessingError struct {
	Type    ErrorType `json:"type"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
	Cause   error     `json:"-"`
}

// ErrorType represents different categories of processing errors.
type ErrorType string

const (
	ErrorTypeFileRead        ErrorType = "file_read"
	ErrorTypeFileWrite       ErrorType = "file_write"
	ErrorTypeInvalidParam    ErrorType = "invalid_parameter"
	ErrorTypeRegexCompile    ErrorType = "regex_compile"
	ErrorTypeProcessingLogic ErrorType = "processing_logic"
)

// NewFileReadError creates a new file read error.
func NewFileReadError(filePath string, cause error) *ProcessingError {
	return &ProcessingError{
		Type:    ErrorTypeFileRead,
		Message: "failed to read file",
		Details: fmt.Sprintf("file: %s", filePath),
		Cause:   cause,
	}
}

// NewFileWriteError creates a new file write error.
func NewFileWriteError(filePath string, cause error) *ProcessingError {
	return &ProcessingError{
		Type:    ErrorTypeFileWrite,
		Message: "failed to write file",
		Details: fmt.Sprintf("file: %s", filePath),
		Cause:   cause,
	}
}

// NewInvalidParamError creates a new invalid parameter error.
func NewInvalidParamError(paramName string, details string) *ProcessingError {
	return &ProcessingError{
		Type:    ErrorTypeInvalidParam,
		Message: fmt.Sprintf("invalid parameter: %s", paramName),
		Details: details,
	}
}

// NewRegexCompileError creates a new regex compilation error.
func NewRegexCompileError(pattern string, cause error) *ProcessingError {
	return &ProcessingError{
		Type:    ErrorTypeRegexCompile,
		Message: "failed to compile regex pattern",
		Details: fmt.Sprintf("pattern: %s", pattern),
		Cause:   cause,
	}
}

// NewProcessingLogicError creates a new processing logic error.
func NewProcessingLogicError(operation string, details string) *ProcessingError {
	return &ProcessingError{
		Type:    ErrorTypeProcessingLogic,
		Message: fmt.Sprintf("processing failed for operation: %s", operation),
		Details: details,
	}
}

// Error implements the error interface.
func (pe *ProcessingError) Error() string {
	if pe.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", pe.Type, pe.Message, pe.Details)
	}
	return fmt.Sprintf("%s: %s", pe.Type, pe.Message)
}

// Unwrap implements the errors.Unwrap interface.
func (pe *ProcessingError) Unwrap() error {
	return pe.Cause
}
