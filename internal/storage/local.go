package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStorage implements the Storage interface for local file storage
type LocalStorage struct {
	baseDir       string
	urlPathPrefix string
}

// resolve returns the filesystem path for a stored filePath without
// double-joining baseDir onto Save's return value. Save returns
// baseDir-joined paths (e.g. "data/uploads/media/a.webp"), so a stored path
// that already carries the baseDir prefix is used as-is, like S3's keyFor
// prefix guard. Only bare filenames are joined with baseDir.
func (s *LocalStorage) resolve(filePath string) string {
	if filepath.IsAbs(filePath) {
		return filePath
	}
	cleaned := filepath.Clean(filePath)
	base := filepath.Clean(s.baseDir)
	if cleaned == base {
		return cleaned
	}
	if strings.HasPrefix(cleaned, base+string(filepath.Separator)) {
		return cleaned
	}
	return filepath.Join(s.baseDir, filePath)
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
	filePath = s.resolve(filePath)
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

// Open returns a reader for the stored file so maintenance jobs read back
// through the interface instead of sniffing the driver. Relative paths
// resolve against baseDir like Delete does.
func (s *LocalStorage) Open(filePath string) (io.ReadCloser, error) {
	file, err := os.Open(s.resolve(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return file, nil
}

// NewLocalStorage creates a new local file storage rooted at baseDir and served
// under the given URL path prefix (e.g. "/uploads/media/")
func NewLocalStorage(baseDir string, urlPathPrefix string) *LocalStorage {
	return &LocalStorage{
		baseDir:       baseDir,
		urlPathPrefix: urlPathPrefix,
	}
}
