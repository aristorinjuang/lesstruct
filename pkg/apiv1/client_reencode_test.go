package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ReencodeMedia(t *testing.T) {
	tests := []struct {
		name       string
		delay      time.Duration
		status     int
		body       string
		wantCode   string
		wantStatus int
	}{
		{
			name:   "success",
			status: http.StatusOK,
			body:   `{"data":{"processed":2,"skipped":0,"bytesBefore":100,"bytesAfter":10}}`,
		},
		{
			name:   "success - slow server still completes",
			delay:  500 * time.Millisecond,
			status: http.StatusOK,
			body:   `{"data":{"processed":2,"skipped":0,"bytesBefore":100,"bytesAfter":10}}`,
		},
		{
			name:       "forbidden",
			status:     http.StatusForbidden,
			body:       `{"error":{"code":"FORBIDDEN","message":"admin only"}}`,
			wantCode:   "FORBIDDEN",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod string
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				if tt.delay > 0 {
					select {
					case <-r.Context().Done():
						return
					case <-time.After(tt.delay):
					}
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c, err := client.New(srv.URL, "k")
			require.NoError(t, err)

			data, _, err := c.ReencodeMedia(context.Background())
			if tt.wantCode != "" {
				require.Error(t, err)
				apiErr, ok := err.(*client.APIError)
				require.True(t, ok)
				assert.Equal(t, tt.wantStatus, apiErr.StatusCode)
				assert.Equal(t, tt.wantCode, apiErr.Code)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, gotMethod)
			assert.Equal(t, "/api/v1/media/reencode", gotPath)
			var summary struct {
				Processed int `json:"processed"`
			}
			require.NoError(t, json.Unmarshal(data, &summary))
			assert.Equal(t, 2, summary.Processed)
		})
	}
}

func TestClient_ReencodeTimeoutCoversLongRuns(t *testing.T) {
	assert.GreaterOrEqual(t, client.ReencodeTimeout, 5*time.Minute, "re-encode rewrites the whole library server-side and takes minutes; the shared 30s client timeout must not apply")
}
