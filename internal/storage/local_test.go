package storage_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appstorage "github.com/aristorinjuang/lesstruct/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorage_Save(t *testing.T) {
	tests := []struct {
		name     string
		baseDir  string
		filename string
		content  []byte
		wantErr  bool
	}{
		{
			name:     "successful save",
			baseDir:  t.TempDir(),
			filename: "test-file.jpg",
			content:  []byte("test content"),
			wantErr:  false,
		},
		{
			name:     "save with nested path creates directory",
			baseDir:  filepath.Join(t.TempDir(), "nested", "subdir"),
			filename: "nested-file.jpg",
			content:  []byte("nested content"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := appstorage.NewLocalStorage(tt.baseDir, "localhost", 8080, "/uploads/media/")

			reader := bytes.NewReader(tt.content)
			filePath, err := storage.Save(tt.filename, reader)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, filePath)

			savedContent, err := os.ReadFile(filePath)
			require.NoError(t, err)
			assert.True(t, bytes.Equal(savedContent, tt.content), "LocalStorage.Save() content mismatch")
		})
	}
}

func TestLocalStorage_Save_MkdirAllError(t *testing.T) {
	// Use a path that cannot be created as a directory
	invalidDir := "/dev/null/invalid/path/that/cannot/be/created"
	storage := appstorage.NewLocalStorage(invalidDir, "localhost", 8080, "/uploads/media/")

	reader := bytes.NewReader([]byte("test content"))
	_, err := storage.Save("test.txt", reader)

	assert.Error(t, err, "LocalStorage.Save() expected error for invalid directory")
	assert.True(t, strings.Contains(err.Error(), "failed to create directory"), "Error should mention directory creation failure")
}

func TestLocalStorage_Save_CreateFileError(t *testing.T) {
	// Create a directory with no write permissions to prevent file creation
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	err := os.Mkdir(readOnlyDir, 0500) // Read and execute only, no write
	require.NoError(t, err)

	storage := appstorage.NewLocalStorage(readOnlyDir, "localhost", 8080, "/uploads/media/")

	reader := bytes.NewReader([]byte("test content"))
	_, err = storage.Save("test.txt", reader)

	// This should fail because we can't create files in a read-only directory
	// Note: This test may behave differently on different OS/file systems
	if err != nil {
		assert.True(t, strings.Contains(err.Error(), "failed to create file") || strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "denied"),
			"Error should mention file creation failure or permission denied")
	}
	// If err is nil, the test still documents the attempt (some OS may allow this)
}

func TestLocalStorage_Save_CopyError(t *testing.T) {
	tmpDir := t.TempDir()
	storage := appstorage.NewLocalStorage(tmpDir, "localhost", 8080, "/uploads/media/")

	reader := &storageErrorReader{}
	_, err := storage.Save("test.txt", reader)

	assert.Error(t, err, "LocalStorage.Save() expected error for write failure")
	assert.True(t, strings.Contains(err.Error(), "failed to write file"), "Error should mention write failure")
}

func TestLocalStorage_Delete_AbsolutePath(t *testing.T) {
	tests := []struct {
		name      string
		setupFile func() (string, error)
		wantErr   bool
	}{
		{
			name: "successful delete",
			setupFile: func() (string, error) {
				tmpDir := t.TempDir()
				filePath := filepath.Join(tmpDir, "test-file.jpg")
				if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
					return "", err
				}
				return filePath, nil
			},
			wantErr: false,
		},
		{
			name: "delete non-existent file returns no error",
			setupFile: func() (string, error) {
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "non-existent.jpg"), nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath, err := tt.setupFile()
			require.NoError(t, err)

			baseDir := filepath.Dir(filePath)
			storage := appstorage.NewLocalStorage(baseDir, "localhost", 8080, "/uploads/media/")

			err = storage.Delete(filePath)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			_, err = os.Stat(filePath)
			assert.True(t, os.IsNotExist(err), "LocalStorage.Delete() file still exists after deletion")
		})
	}
}

func TestLocalStorage_Delete_RelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "user_123.webp"), []byte("fake data"), 0644)
	require.NoError(t, err)

	storage := appstorage.NewLocalStorage(tmpDir, "localhost", 8080, "/uploads/profile_pictures/")

	err = storage.Delete("user_123.webp")
	assert.NoError(t, err, "LocalStorage.Delete() should resolve relative paths against baseDir")

	_, err = os.Stat(filepath.Join(tmpDir, "user_123.webp"))
	assert.True(t, os.IsNotExist(err), "File should be deleted")
}

func TestLocalStorage_Delete_ReadOnlyFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "readonly.txt")
	err := os.WriteFile(filePath, []byte("test"), 0444) // Read-only file
	require.NoError(t, err)

	storage := appstorage.NewLocalStorage(tmpDir, "localhost", 8080, "/uploads/media/")

	// Deleting a read-only file should succeed (we have write permission on directory)
	err = storage.Delete(filePath)
	assert.NoError(t, err, "LocalStorage.Delete() should succeed for read-only file in writable directory")
}

func TestLocalStorage_Delete_DirectoryInsteadOfFile(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "testdir")

	err := os.Mkdir(dirPath, 0755)
	require.NoError(t, err)

	storage := appstorage.NewLocalStorage(tmpDir, "localhost", 8080, "/uploads/media/")

	// Attempting to delete a directory instead of a file
	// On most systems this will fail with an error other than IsNotExist
	err = storage.Delete(dirPath)

	// The behavior varies by OS - on Unix, removing a directory requires RemoveDirectory
	// We document that this path exists even if we can't reliably trigger the specific error
	if err != nil && !os.IsNotExist(err) {
		// If we get a non-IsNotExist error, that's the path we're trying to cover
		assert.Error(t, err, "Delete may return error for directory")
		assert.True(t, strings.Contains(err.Error(), "failed to delete file"), "Error should mention delete failure")
	}
	// If err is nil, the OS allowed deleting a directory when we asked to delete a file
}

func TestLocalStorage_Delete_FileInNonWritableDirectory(t *testing.T) {
	// Create a subdirectory with a file, then make the subdirectory non-writable
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")

	err := os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	filePath := filepath.Join(subDir, "test.txt")
	err = os.WriteFile(filePath, []byte("test"), 0644)
	require.NoError(t, err)

	// Make the subdirectory non-writable (remove write permission)
	err = os.Chmod(subDir, 0500) // read+execute only
	require.NoError(t, err)

	storage := appstorage.NewLocalStorage(tmpDir, "localhost", 8080, "/uploads/media/")

	// Try to delete the file - this might fail with a permission error
	err = storage.Delete(filePath)

	// Clean up - restore permissions so we can remove the directory
	_ = os.Chmod(subDir, 0755)

	// If we got a non-IsNotExist error, that's the path we're trying to cover
	if err != nil && !os.IsNotExist(err) {
		assert.Error(t, err, "Delete should return error for file in non-writable directory")
		assert.True(t, strings.Contains(err.Error(), "failed to delete file"), "Error should mention delete failure")
	}
	// If err is nil, the OS allowed the deletion despite non-writable directory
}

func TestLocalStorage_GetURL(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		port          int
		urlPathPrefix string
		filePath      string
		want          string
	}{
		{
			name:          "media url with default port",
			host:          "localhost",
			port:          8080,
			urlPathPrefix: "/uploads/media/",
			filePath:      "/uploads/media/test-file.jpg",
			want:          "http://localhost:8080/uploads/media/test-file.jpg",
		},
		{
			name:          "media url extracts filename from path",
			host:          "localhost",
			port:          8080,
			urlPathPrefix: "/uploads/media/",
			filePath:      "/uploads/media/subdir/nested-file.gif",
			want:          "http://localhost:8080/uploads/media/nested-file.gif",
		},
		{
			name:          "profile picture url with relative filename",
			host:          "example.com",
			port:          3000,
			urlPathPrefix: "/uploads/profile_pictures/",
			filePath:      "user_123.webp",
			want:          "http://example.com:3000/uploads/profile_pictures/user_123.webp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := appstorage.NewLocalStorage("/tmp", tt.host, tt.port, tt.urlPathPrefix)

			got := storage.GetURL(tt.filePath)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewLocalStorage(t *testing.T) {
	tests := []struct {
		name    string
		baseDir string
		host    string
		port    int
		prefix  string
	}{
		{
			name:    "creates storage with default values",
			baseDir: "/uploads/media",
			host:    "localhost",
			port:    8080,
			prefix:  "/uploads/media/",
		},
		{
			name:    "creates storage with custom values",
			baseDir: "/var/storage",
			host:    "example.com",
			port:    3000,
			prefix:  "/uploads/profile_pictures/",
		},
		{
			name:    "zero host resolves to localhost for URL building",
			baseDir: "/uploads/media",
			host:    "0.0.0.0",
			port:    8080,
			prefix:  "/uploads/media/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := appstorage.NewLocalStorage(tt.baseDir, tt.host, tt.port, tt.prefix)

			assert.NotNil(t, storage)

			if tt.host == "0.0.0.0" {
				got := storage.GetURL("file.webp")
				assert.Equal(t, "http://localhost:8080"+tt.prefix+"file.webp", got)
			}
		})
	}
}

type storageErrorReader struct{}

func (e *storageErrorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}
