package profilepicture

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalStorage handles local file storage for profile pictures
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

// GetURL returns the URL for a file
func (s *LocalStorage) GetURL(filename string) string {
	return s.baseURL + filename
}

// NewLocalStorage creates a new local file storage for profile pictures
func NewLocalStorage(baseDir string, host string, port int) *LocalStorage {
	if host == "0.0.0.0" {
		host = "localhost"
	}
	baseURL := fmt.Sprintf("http://%s:%d/uploads/profile_pictures/", host, port)
	return &LocalStorage{
		baseDir: baseDir,
		baseURL: baseURL,
	}
}
