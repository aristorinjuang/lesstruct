package media

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalStorage implements the Storage interface for local file storage
type LocalStorage struct {
	baseDir string
	baseURL string
}

// Save saves a file to the local storage
func (s *LocalStorage) Save(filename string, reader io.Reader) (string, error) {
	filePath := filepath.Join(s.baseDir, filename)

	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = file.Close() }()

	_, err = io.Copy(file, reader)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return filePath, nil
}

// Delete deletes a file from the local storage
func (s *LocalStorage) Delete(filePath string) error {
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// GetURL returns the URL for a file
func (s *LocalStorage) GetURL(filePath string) string {
	filename := filepath.Base(filePath)
	return s.baseURL + filename
}

// NewLocalStorage creates a new local file storage
func NewLocalStorage(baseDir string, host string, port int) *LocalStorage {
	if host == "0.0.0.0" {
		host = "localhost"
	}
	baseURL := fmt.Sprintf("http://%s:%d/uploads/media/", host, port)
	return &LocalStorage{
		baseDir: baseDir,
		baseURL: baseURL,
	}
}
