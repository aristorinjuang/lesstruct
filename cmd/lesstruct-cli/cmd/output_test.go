package cmd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputJSON_ErrorsAreJSON(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		status   int
		wantCode int
		wantMsg  string
	}{
		{
			name:     "server error arrives as a JSON envelope",
			args:     []string{"media", "get", "999", "--output", "json"},
			status:   http.StatusNotFound,
			wantCode: 4,
			wantMsg:  "Media not found",
		},
		{
			name:     "equals form selects JSON too",
			args:     []string{"media", "get", "999", "--output=json"},
			status:   http.StatusNotFound,
			wantCode: 4,
			wantMsg:  "Media not found",
		},
		{
			name:     "client-side validation error arrives as JSON",
			args:     []string{"media", "get", "abc", "--output", "json"},
			wantCode: 5,
			wantMsg:  "invalid media id",
		},
		{
			name:     "cobra usage error arrives as JSON",
			args:     []string{"media", "get", "--output", "json"},
			wantCode: 2,
			wantMsg:  "accepts 1 arg(s), received 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"error":{"code":"NOT_FOUND","message":"Media not found"}}`))
			}))
			defer srv.Close()

			args := append(append([]string{}, tt.args...), "--base-url", srv.URL, "--api-key", "k")
			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(args, strings.NewReader(""), &out, &errOut)

			require.Equal(t, tt.wantCode, code)
			assert.Empty(t, out.String(), "stdout stays clean on failure")
			var envelope struct {
				Error struct {
					Message  string `json:"message"`
					ExitCode int    `json:"exitCode"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(errOut.Bytes(), &envelope), "stderr must be JSON, got: %q", errOut.String())
			assert.Contains(t, envelope.Error.Message, tt.wantMsg)
			assert.Equal(t, tt.wantCode, envelope.Error.ExitCode)
		})
	}
}

func TestOutputText_ErrorsStayPlain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"NOT_FOUND","message":"Media not found"}}`))
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "get", "999", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""), &out, &errOut,
	)

	require.Equal(t, 4, code)
	assert.Contains(t, errOut.String(), "Media not found")
	assert.NotContains(t, errOut.String(), `"error"`, "text mode must not emit JSON")
}
