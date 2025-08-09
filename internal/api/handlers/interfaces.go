package handlers

import (
	"context"
	"mime/multipart"

	"github.com/google/uuid"
	"github.com/rsav/k8s-learning/internal/storage/database"
	"github.com/rsav/k8s-learning/internal/storage/filestore"
	"github.com/rsav/k8s-learning/internal/storage/queue"
)

type Repository interface {
	JobsRepository
	HealthCheck(ctx context.Context) error
}

type JobsRepository interface {
	GetJobs(ctx context.Context, req database.GetJobsFilter) ([]*database.Job, error)
	GetJobByID(ctx context.Context, id uuid.UUID) (*database.Job, error)
	CountJobs(ctx context.Context) (int, error)
	CountJobsByStatus(ctx context.Context, status database.JobStatus) (int, error)
	CreateJob(ctx context.Context, job *database.Job) error
}

type Queue interface {
	PublishJob(ctx context.Context, message queue.SubmitJobMessage) error
	GetStats(ctx context.Context) (map[string]interface{}, error)
	HealthCheck(ctx context.Context) error
}

type FileStorage interface {
	SaveUploadedFile(fileHeader *multipart.FileHeader) (*filestore.FileInfo, error)
	ReadFile(filePath string) ([]byte, error)
	FileExists(filePath string) bool
	DeleteFile(filePath string) error
	GetStoragePaths() (string, string)
	GetMaxFileSize() int64
}
