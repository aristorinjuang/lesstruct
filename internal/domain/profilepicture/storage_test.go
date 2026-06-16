package profilepicture_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/profilepicture"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorage_Save(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)

	tests := []struct {
		name     string
		filename string
		content  []byte
		wantErr  bool
	}{
		{
			name:     "save valid file",
			filename: "test_user_20260101120000.webp",
			content:  []byte("fake webp data"),
			wantErr:  false,
		},
		{
			name:     "save empty file",
			filename: "test_user_20260101120001.webp",
			content:  []byte{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.content)
			filePath, err := storage.Save(tt.filename, reader)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify file exists on disk
			assert.FileExists(t, filePath)
			expectedDir := filepath.Join(baseDir, tt.filename)
			assert.Equal(t, expectedDir, filePath)
		})
	}
}

func TestLocalStorage_Delete(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)

	// Save a file first
	filename := "test_user_20260101120000.webp"
	filePath, err := storage.Save(filename, bytes.NewReader([]byte("fake webp data")))
	require.NoError(t, err)

	// Delete using the full path returned by Save
	err = storage.Delete(filePath)
	require.NoError(t, err)

	// Verify file is gone
	_, err = os.Stat(filepath.Join(baseDir, filename))
	assert.True(t, os.IsNotExist(err))
}

func TestLocalStorage_Delete_NonExistent(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)

	// Delete non-existent file should not error
	err := storage.Delete("nonexistent.webp")
	require.NoError(t, err)
}

func TestLocalStorage_GetURL(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)

	url := storage.GetURL("test_user_20260101120000.webp")
	assert.Equal(t, "http://localhost:8080/uploads/profile_pictures/test_user_20260101120000.webp", url)
}

func TestNewLocalStorage_ZeroHost(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "0.0.0.0", 3000)

	url := storage.GetURL("test.webp")
	assert.Contains(t, url, "localhost:3000")
}
