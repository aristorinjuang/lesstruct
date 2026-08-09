package storage_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	appstorage "github.com/aristorinjuang/lesstruct/internal/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// s3MockServer emulates the S3 REST API for the subset of operations used by
// S3Storage (PutObject and DeleteObject), recording the requests it receives.
type s3MockServer struct {
	server       *httptest.Server
	mu           sync.Mutex
	lastMethod   string
	lastPath     string
	lastBody     string
	lastCT       string
	putStatus    int
	deleteStatus int
}

func (m *s3MockServer) snapshot() (method, path, body, contentType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastMethod, m.lastPath, m.lastBody, m.lastCT
}

func newS3MockServer(t *testing.T) *s3MockServer {
	t.Helper()
	m := &s3MockServer{putStatus: http.StatusOK, deleteStatus: http.StatusNoContent}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		m.mu.Lock()
		m.lastMethod = r.Method
		m.lastPath = r.URL.Path
		m.lastBody = string(body)
		m.lastCT = r.Header.Get("Content-Type")
		m.mu.Unlock()

		switch r.Method {
		case http.MethodPut:
			w.Header().Set("ETag", `"mock-etag"`)
			w.WriteHeader(m.putStatus)
			if m.putStatus != http.StatusOK {
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>InternalError</Code><Message>mock failure</Message></Error>`))
			}
		case http.MethodDelete:
			w.WriteHeader(m.deleteStatus)
			if m.deleteStatus != http.StatusNoContent {
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>InternalError</Code><Message>mock failure</Message></Error>`))
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(m.server.Close)
	return m
}

func newTestS3Storage(t *testing.T, m *s3MockServer, keyPrefix string) *appstorage.S3Storage {
	t.Helper()
	storage, err := appstorage.NewS3Storage(appstorage.S3Options{
		Endpoint:        m.server.URL,
		Region:          "us-east-1",
		Bucket:          "lesstruct",
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-secret-key",
		UsePathStyle:    true,
		PublicBaseURL:   "https://cdn.example.com",
		KeyPrefix:       keyPrefix,
		HTTPClient:      m.server.Client(),
		Retryer:         aws.NopRetryer{},
	})
	require.NoError(t, err)
	return storage
}

func TestS3Storage_Save(t *testing.T) {
	tests := []struct {
		name      string
		keyPrefix string
		filename  string
		content   string
		wantKey   string
	}{
		{
			name:      "saves under key prefix",
			keyPrefix: "media/",
			filename:  "abc123.webp",
			content:   "image-bytes",
			wantKey:   "media/abc123.webp",
		},
		{
			name:      "saves without key prefix",
			keyPrefix: "",
			filename:  "user_1.webp",
			content:   "avatar-bytes",
			wantKey:   "user_1.webp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newS3MockServer(t)
			storage := newTestS3Storage(t, m, tt.keyPrefix)

			key, err := storage.Save(tt.filename, strings.NewReader(tt.content))

			require.NoError(t, err)
			assert.Equal(t, tt.wantKey, key)

			method, path, body, _ := m.snapshot()
			assert.Equal(t, http.MethodPut, method)
			assert.Equal(t, "/lesstruct/"+tt.wantKey, path)
			assert.Equal(t, tt.content, body)
		})
	}
}

func TestS3Storage_Save_ContentType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  string
		wantCT   string
	}{
		{
			name:     "webp extension maps to image/webp",
			filename: "photo.webp",
			content:  "bytes",
			wantCT:   "image/webp",
		},
		{
			name:     "jpeg extension maps to image/jpeg",
			filename: "photo.jpg",
			content:  "bytes",
			wantCT:   "image/jpeg",
		},
		{
			name:     "unknown extension falls back to content sniffing",
			filename: "photo.xyz",
			content:  string([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00}),
			wantCT:   "image/jpeg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newS3MockServer(t)
			storage := newTestS3Storage(t, m, "media/")

			_, err := storage.Save(tt.filename, strings.NewReader(tt.content))
			require.NoError(t, err)

			_, _, _, contentType := m.snapshot()
			assert.Equal(t, tt.wantCT, contentType)
		})
	}
}

func TestS3Storage_Save_Error(t *testing.T) {
	m := newS3MockServer(t)
	m.putStatus = http.StatusInternalServerError
	storage := newTestS3Storage(t, m, "media/")

	_, err := storage.Save("abc.webp", strings.NewReader("bytes"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to upload object")
}

func TestS3Storage_Delete(t *testing.T) {
	tests := []struct {
		name      string
		keyPrefix string
		filePath  string
		wantPath  string
	}{
		{
			name:      "deletes full object key",
			keyPrefix: "media/",
			filePath:  "media/abc.webp",
			wantPath:  "/lesstruct/media/abc.webp",
		},
		{
			name:      "resolves bare filename against key prefix",
			keyPrefix: "profile_pictures/",
			filePath:  "user_1.webp",
			wantPath:  "/lesstruct/profile_pictures/user_1.webp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newS3MockServer(t)
			storage := newTestS3Storage(t, m, tt.keyPrefix)

			err := storage.Delete(tt.filePath)

			require.NoError(t, err)
			method, path, _, _ := m.snapshot()
			assert.Equal(t, http.MethodDelete, method)
			assert.Equal(t, tt.wantPath, path)
		})
	}
}

func TestS3Storage_Delete_Error(t *testing.T) {
	m := newS3MockServer(t)
	m.deleteStatus = http.StatusInternalServerError
	storage := newTestS3Storage(t, m, "media/")

	err := storage.Delete("media/abc.webp")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete object")
}

func TestS3Storage_GetURL(t *testing.T) {
	tests := []struct {
		name      string
		keyPrefix string
		baseURL   string
		filePath  string
		want      string
	}{
		{
			name:      "full object key used as-is",
			keyPrefix: "media/",
			baseURL:   "https://cdn.example.com",
			filePath:  "media/abc.webp",
			want:      "https://cdn.example.com/media/abc.webp",
		},
		{
			name:      "bare filename resolved against key prefix",
			keyPrefix: "profile_pictures/",
			baseURL:   "https://cdn.example.com",
			filePath:  "user_1.webp",
			want:      "https://cdn.example.com/profile_pictures/user_1.webp",
		},
		{
			name:      "trailing slash on public base URL is trimmed",
			keyPrefix: "media/",
			baseURL:   "https://cdn.example.com/",
			filePath:  "media/abc.webp",
			want:      "https://cdn.example.com/media/abc.webp",
		},
		{
			name:      "no key prefix",
			keyPrefix: "",
			baseURL:   "https://cdn.example.com",
			filePath:  "abc.webp",
			want:      "https://cdn.example.com/abc.webp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newS3MockServer(t)
			storage, err := appstorage.NewS3Storage(appstorage.S3Options{
				Endpoint:      m.server.URL,
				Region:        "us-east-1",
				Bucket:        "lesstruct",
				PublicBaseURL: tt.baseURL,
				KeyPrefix:     tt.keyPrefix,
				HTTPClient:    m.server.Client(),
				Retryer:       aws.NopRetryer{},
			})
			require.NoError(t, err)

			got := storage.GetURL(tt.filePath)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewS3Storage(t *testing.T) {
	tests := []struct {
		name      string
		opts      appstorage.S3Options
		wantErr   bool
		errorText string
	}{
		{
			name: "valid options",
			opts: appstorage.S3Options{
				Region:        "us-east-1",
				Bucket:        "lesstruct",
				PublicBaseURL: "https://cdn.example.com",
			},
			wantErr: false,
		},
		{
			name: "missing bucket",
			opts: appstorage.S3Options{
				Region:        "us-east-1",
				PublicBaseURL: "https://cdn.example.com",
			},
			wantErr:   true,
			errorText: "bucket is required",
		},
		{
			name: "missing region",
			opts: appstorage.S3Options{
				Bucket:        "lesstruct",
				PublicBaseURL: "https://cdn.example.com",
			},
			wantErr:   true,
			errorText: "region is required",
		},
		{
			name: "missing public base URL",
			opts: appstorage.S3Options{
				Region: "us-east-1",
				Bucket: "lesstruct",
			},
			wantErr:   true,
			errorText: "public base URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := appstorage.NewS3Storage(tt.opts)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorText)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, storage)
		})
	}
}
