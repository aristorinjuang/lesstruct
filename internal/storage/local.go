package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalStorage implements the Storage interface for local file storage
type LocalStorage struct {
	baseDir string
	urlPathPrefix string
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

// Delete deletes a file from the local storage.
// If filePath is relative, it is resolved against baseDir.
func (s *LocalStorage) Delete(filePath string) error {
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(s.baseDir, filePath)
	}
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// GetURL returns the relative URL path for a file. Local uploads are always
// served by this same process (the /uploads/ file server is mounted only when
// the local driver is active), so a root-relative URL keeps working behind
// reverse proxies and HTTPS without baking host/port into stored data.
func (s *LocalStorage) GetURL(filePath string) string {
	return s.urlPathPrefix + filepath.Base(filePath)
}

// NewLocalStorage creates a new local file storage rooted at baseDir and served
// under the given URL path prefix (e.g. "/uploads/media/")
func NewLocalStorage(baseDir string, urlPathPrefix string) *LocalStorage {
	return &LocalStorage{
		baseDir:       baseDir,
		urlPathPrefix: urlPathPrefix,
	}
}
