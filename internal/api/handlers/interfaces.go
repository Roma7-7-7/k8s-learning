package handlers

import (
	"context"
	"mime/multipart"

	"github.com/google/uuid"
	"github.com/rsav/k8s-learning/internal/models"
)

// DatabaseInterface defines what the handlers need from a database
type DatabaseInterface interface {
	Jobs() JobRepositoryInterface
	HealthCheck(ctx context.Context) error
}

// JobRepositoryInterface defines what handlers need for job operations
type JobRepositoryInterface interface {
	Create(ctx context.Context, job *models.Job) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Job, error)
	List(ctx context.Context, req models.ListJobsRequest) ([]*models.Job, error)
	Count(ctx context.Context) (int64, error)
	CountByStatus(ctx context.Context, status string) (int64, error)
}

// QueueInterface defines what handlers need from a queue
type QueueInterface interface {
	PublishJob(ctx context.Context, message models.QueueMessage) error
	GetStats(ctx context.Context) (map[string]interface{}, error)
	HealthCheck(ctx context.Context) error
}

// FileStorageInterface defines what handlers need from file storage
type FileStorageInterface interface {
	SaveUploadedFile(fileHeader *multipart.FileHeader) (*FileInfo, error)
	ReadFile(filePath string) ([]byte, error)
	FileExists(filePath string) bool
	DeleteFile(filePath string) error
	GetStoragePaths() (string, string)
	GetMaxFileSize() int64
}

// FileInfo represents information about a stored file
type FileInfo struct {
	ID           string
	OriginalName string
	StoredPath   string
	Size         int64
	ContentType  string
}
