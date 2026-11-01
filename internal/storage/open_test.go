package storage_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	appstorage "github.com/aristorinjuang/lesstruct/internal/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorage_Open(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, storage *appstorage.LocalStorage) string
		wantErr  bool
		expected []byte
	}{
		{
			name: "success - absolute path",
			setup: func(t *testing.T, storage *appstorage.LocalStorage) string {
				t.Helper()
				path, err := storage.Save("open-me.webp", bytes.NewReader([]byte("bytes")))
				require.NoError(t, err)
				return path
			},
			wantErr:  false,
			expected: []byte("bytes"),
		},
		{
			name: "success - relative path resolves against baseDir",
			setup: func(t *testing.T, storage *appstorage.LocalStorage) string {
				t.Helper()
				_, err := storage.Save("rel.webp", bytes.NewReader([]byte("rel-bytes")))
				require.NoError(t, err)
				return "rel.webp"
			},
			wantErr:  false,
			expected: []byte("rel-bytes"),
		},
		{
			name: "error - missing file",
			setup: func(t *testing.T, storage *appstorage.LocalStorage) string {
				t.Helper()
				return "does-not-exist.webp"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := appstorage.NewLocalStorage(t.TempDir(), "/uploads/media/")
			filePath := tt.setup(t, storage)

			rc, err := storage.Open(filePath)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			defer func() { _ = rc.Close() }()
			got, err := io.ReadAll(rc)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestS3Storage_Open(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		status     int
		wantErr    bool
		expected   string
		keyPrefix  string
		requestKey string
	}{
		{
			name:       "success - reads object body",
			body:       "object-bytes",
			status:     http.StatusOK,
			wantErr:    false,
			expected:   "object-bytes",
			keyPrefix:  "media/",
			requestKey: "media/a.webp",
		},
		{
			name:       "success - bare filename resolves against prefix",
			body:       "avatar",
			status:     http.StatusOK,
			wantErr:    false,
			expected:   "avatar",
			keyPrefix:  "media/",
			requestKey: "a.webp",
		},
		{
			name:       "error - backend failure surfaces open error",
			body:       "boom",
			status:     http.StatusInternalServerError,
			wantErr:    true,
			keyPrefix:  "media/",
			requestKey: "media/a.webp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			storage, err := appstorage.NewS3Storage(appstorage.S3Options{
				Endpoint:        srv.URL,
				Region:          "us-east-1",
				Bucket:          "lesstruct",
				AccessKeyID:     "k",
				SecretAccessKey: "s",
				UsePathStyle:    true,
				PublicBaseURL:   "https://cdn.example.com",
				KeyPrefix:       tt.keyPrefix,
				HTTPClient:      srv.Client(),
				Retryer:         aws.NopRetryer{},
			})
			require.NoError(t, err)

			rc, err := storage.Open(tt.requestKey)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			defer func() { _ = rc.Close() }()
			got, err := io.ReadAll(rc)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(got))
		})
	}
}

func TestLocalStorage_SaveOpenDeleteRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		contents []byte
		wantErr  bool
	}{
		{
			name:     "success - Save returned path round-trips through Open and Delete",
			filename: "roundtrip.webp",
			contents: []byte("roundtrip-bytes"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			cwd, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(tmp))
			defer func() { _ = os.Chdir(cwd) }()

			storage := appstorage.NewLocalStorage(
				filepath.Join("data", "uploads", "media"),
				"/uploads/media/",
			)
			savedPath, err := storage.Save(tt.filename, bytes.NewReader(tt.contents))
			require.NoError(t, err)
			assert.Equal(
				t,
				filepath.Join("data", "uploads", "media", tt.filename),
				savedPath,
			)

			rc, err := storage.Open(savedPath)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got, err := io.ReadAll(rc)
			require.NoError(t, err)
			require.NoError(t, rc.Close())
			assert.Equal(t, tt.contents, got)

			require.NoError(t, storage.Delete(savedPath))
			_, err = storage.Open(savedPath)
			require.Error(t, err)
		})
	}
}
