package client_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hugoImportHandler records the multipart file + skipMedia field and replies
// with the given JSON body.
type hugoImportHandler struct {
	gotPath     string
	gotFile     []byte
	gotFilename string
	gotSkip     string
	status      int
	body        string
}

func (h *hugoImportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.gotPath = r.URL.Path
	if err := r.ParseMultipartForm(32 << 20); err == nil {
		if f, fh, ferr := r.FormFile("file"); ferr == nil {
			h.gotFile, _ = io.ReadAll(f)
			h.gotFilename = fh.Filename
		}
		h.gotSkip = r.FormValue("skipMedia")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.status)
	_, _ = w.Write([]byte(h.body))
}

func TestClient_ImportHugo_Success(t *testing.T) {
	tests := []struct {
		name        string
		skipMedia   bool
		wantSkip    string
		wantPath    string
	}{
		{
			name:     "success - upload archive without skipMedia",
			skipMedia: false,
			wantSkip: "",
			wantPath: "/api/v1/hugo/import",
		},
		{
			name:     "success - upload archive with skipMedia",
			skipMedia: true,
			wantSkip: "true",
			wantPath: "/api/v1/hugo/import",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &hugoImportHandler{
				status: http.StatusAccepted,
				body:   `{"data":{"jobId":"job-9","state":"running"},"error":null}`,
			}
			srv := httptest.NewServer(h)
			defer srv.Close()

			c, err := client.New(srv.URL, "lesstruct_a1b2c3d4e5f6_<secret>")
			require.NoError(t, err)

			data, _, err := c.ImportHugo(context.Background(), client.ImportHugoRequest{
				File:      bytes.NewReader([]byte("archive-bytes")),
				Filename:  "site.tar.gz",
				SkipMedia: tt.skipMedia,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.wantPath, h.gotPath)
			assert.Equal(t, "site.tar.gz", h.gotFilename)
			assert.Equal(t, []byte("archive-bytes"), h.gotFile)
			assert.Equal(t, tt.wantSkip, h.gotSkip)

			var resp struct {
				JobID string `json:"jobId"`
				State string `json:"state"`
			}
			require.NoError(t, json.Unmarshal(data, &resp))
			assert.Equal(t, "job-9", resp.JobID)
			assert.Equal(t, "running", resp.State)
		})
	}
}

func TestClient_ImportHugo_Validation(t *testing.T) {
	c, err := client.New("http://localhost:9999", "k")
	require.NoError(t, err)

	_, _, err = c.ImportHugo(context.Background(), client.ImportHugoRequest{
		File:     nil,
		Filename: "site.tar.gz",
	})
	require.Error(t, err)
	var apiErr *client.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "VALIDATION_ERROR", apiErr.Code)
	assert.Contains(t, apiErr.Message, "File is required")

	_, _, err = c.ImportHugo(context.Background(), client.ImportHugoRequest{
		File:     bytes.NewReader([]byte("x")),
		Filename: "",
	})
	require.Error(t, err)
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "VALIDATION_ERROR", apiErr.Code)
	assert.Contains(t, apiErr.Message, "Filename is required")
}

func TestClient_ImportHugo_ServerError(t *testing.T) {
	h := &hugoImportHandler{
		status: http.StatusForbidden,
		body:   `{"data":null,"error":{"code":"INSUFFICIENT_PERMISSIONS","message":"admin required"}}`,
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c, err := client.New(srv.URL, "k")
	require.NoError(t, err)

	_, _, err = c.ImportHugo(context.Background(), client.ImportHugoRequest{
		File:     bytes.NewReader([]byte("archive-bytes")),
		Filename: "site.tar.gz",
	})
	require.Error(t, err)
	var apiErr *client.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, "INSUFFICIENT_PERMISSIONS", apiErr.Code)
	assert.Contains(t, apiErr.Message, "admin required")
}

func TestClient_GetHugoImportStatus(t *testing.T) {
	tests := []struct {
		name     string
		jobID    string
		wantPath string
	}{
		{
			name:     "success - specific job",
			jobID:    "job-9",
			wantPath: "/api/v1/hugo/import/status/job-9",
		},
		{
			name:     "success - latest job when id empty",
			jobID:    "",
			wantPath: "/api/v1/hugo/import/status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"jobId":"job-9","job":{"state":"done","imported":1,"skipped":0,"total":1}},"error":null}`))
			}))
			defer srv.Close()

			c, err := client.New(srv.URL, "k")
			require.NoError(t, err)

			data, _, err := c.GetHugoImportStatus(context.Background(), tt.jobID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantPath, gotPath)

			var env struct {
				Job client.ImportJobStatus `json:"job"`
			}
			require.NoError(t, json.Unmarshal(data, &env))
			assert.Equal(t, "done", env.Job.State)
			assert.Equal(t, 1, env.Job.Imported)
		})
	}
}
