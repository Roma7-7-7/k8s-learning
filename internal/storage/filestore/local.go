package filestore

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rsav/k8s-learning/internal/api/handlers"
)

type FileStore struct {
	uploadDir string
	resultDir string
	maxSize   int64
}

type FileInfo struct {
	ID           string
	OriginalName string
	StoredPath   string
	Size         int64
	ContentType  string
}

func NewFileStore(uploadDir, resultDir string, maxSize int64) (*FileStore, error) {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	if err := os.MkdirAll(resultDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create result directory: %w", err)
	}

	return &FileStore{
		uploadDir: uploadDir,
		resultDir: resultDir,
		maxSize:   maxSize,
	}, nil
}

func (fs *FileStore) SaveUploadedFile(fileHeader *multipart.FileHeader) (*handlers.FileInfo, error) {
	if fileHeader.Size > fs.maxSize {
		return nil, fmt.Errorf("file size %d exceeds maximum allowed size %d",
			fileHeader.Size, fs.maxSize)
	}

	if !fs.isValidTextFile(fileHeader.Filename) {
		return nil, fmt.Errorf("invalid file type: only text files are allowed")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer file.Close()

	fileID := uuid.New().String()
	ext := filepath.Ext(fileHeader.Filename)
	storedName := fmt.Sprintf("%s%s", fileID, ext)
	storedPath := filepath.Join(fs.uploadDir, storedName)

	dst, err := os.Create(storedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	size, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(storedPath)
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	return &handlers.FileInfo{
		ID:           fileID,
		OriginalName: fileHeader.Filename,
		StoredPath:   storedPath,
		Size:         size,
		ContentType:  fileHeader.Header.Get("Content-Type"),
	}, nil
}

func (fs *FileStore) SaveResultFile(jobID uuid.UUID, filename string, content []byte) (string, error) {
	resultName := fmt.Sprintf("%s_%s", jobID.String(), filename)
	resultPath := filepath.Join(fs.resultDir, resultName)

	if err := os.WriteFile(resultPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to save result file: %w", err)
	}

	return resultPath, nil
}

func (fs *FileStore) ReadFile(filePath string) ([]byte, error) {
	if !fs.isValidPath(filePath) {
		return nil, fmt.Errorf("invalid file path")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return content, nil
}

func (fs *FileStore) FileExists(filePath string) bool {
	if !fs.isValidPath(filePath) {
		return false
	}

	_, err := os.Stat(filePath)
	return err == nil
}

func (fs *FileStore) DeleteFile(filePath string) error {
	if !fs.isValidPath(filePath) {
		return fmt.Errorf("invalid file path")
	}

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

func (fs *FileStore) GetFileSize(filePath string) (int64, error) {
	if !fs.isValidPath(filePath) {
		return 0, fmt.Errorf("invalid file path")
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}

	return info.Size(), nil
}

func (fs *FileStore) GetFileModTime(filePath string) (time.Time, error) {
	if !fs.isValidPath(filePath) {
		return time.Time{}, fmt.Errorf("invalid file path")
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get file info: %w", err)
	}

	return info.ModTime(), nil
}

func (fs *FileStore) CleanupOldFiles(maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge)

	err := filepath.Walk(fs.uploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove old file %s: %w", path, err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to cleanup upload directory: %w", err)
	}

	err = filepath.Walk(fs.resultDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove old file %s: %w", path, err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to cleanup result directory: %w", err)
	}

	return nil
}

func (fs *FileStore) isValidTextFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	validExtensions := []string{".txt", ".md", ".csv", ".json", ".xml", ".log"}

	for _, validExt := range validExtensions {
		if ext == validExt {
			return true
		}
	}

	return false
}

func (fs *FileStore) isValidPath(filePath string) bool {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}

	uploadAbs, err := filepath.Abs(fs.uploadDir)
	if err != nil {
		return false
	}

	resultAbs, err := filepath.Abs(fs.resultDir)
	if err != nil {
		return false
	}

	return strings.HasPrefix(absPath, uploadAbs) || strings.HasPrefix(absPath, resultAbs)
}

func (fs *FileStore) GetStoragePaths() (string, string) {
	return fs.uploadDir, fs.resultDir
}

func (fs *FileStore) GetMaxFileSize() int64 {
	return fs.maxSize
}
