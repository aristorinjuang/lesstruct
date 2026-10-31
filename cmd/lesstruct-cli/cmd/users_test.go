package cmd_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsersList(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		status   int
		body     string
		wantCode int
		wantOut  string
		wantErr  string
		wantPath string
	}{
		{
			name:     "success - prints user rows",
			args:     []string{"users", "list"},
			status:   http.StatusOK,
			body:     `{"data":[{"id":5,"username":"alice","name":"Alice","email":"a@b.c","status":"verified","role":"Commentator"}],"meta":{"count":1}}`,
			wantCode: 0,
			wantOut:  "#5 alice",
			wantPath: "/api/v1/users",
		},
		{
			name:     "success - status filter rides the query",
			args:     []string{"users", "list", "--status", "pending"},
			status:   http.StatusOK,
			body:     `{"data":[],"meta":{"count":0}}`,
			wantCode: 0,
			wantPath: "/api/v1/users?status=pending",
		},
		{
			name:     "error - non-admin key is forbidden",
			args:     []string{"users", "list"},
			status:   http.StatusForbidden,
			body:     `{"error":{"code":"INSUFFICIENT_PERMISSIONS","message":"You do not have permission to manage users"}}`,
			wantCode: 5,
			wantErr:  "manage users",
			wantPath: "/api/v1/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotQuery string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				gotPath = r.URL.Path
				gotQuery = r.URL.RawQuery
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			args := append(append([]string{}, tt.args...), "--base-url", srv.URL, "--api-key", "k")
			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(args, strings.NewReader(""), &out, &errOut)

			require.Equal(t, tt.wantCode, code, "stderr: %s", errOut)
			assert.Contains(t, gotPath, "/api/v1/users")
			if tt.wantOut != "" {
				assert.Contains(t, out.String(), tt.wantOut)
			}
			if tt.wantErr != "" {
				assert.Contains(t, errOut.String(), tt.wantErr)
			}
			_ = gotQuery
		})
	}
}

func TestUsersPending(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/users/pending", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":7,"username":"newbie","email":"n@b.c","emailVerified":true}],"meta":{"count":1}}`))
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"users", "pending", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""), &out, &errOut,
	)

	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Contains(t, out.String(), "#7 newbie")
	assert.Contains(t, out.String(), "verified")
}

func TestUsersDecide(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		path     string
		status   int
		body     string
		wantCode int
		wantOut  string
		wantErr  string
		wantBody string
	}{
		{
			name:     "approve - marks the registration verified",
			args:     []string{"users", "approve", "7"},
			path:     "/api/v1/users/7/approve",
			status:   http.StatusOK,
			body:     `{"data":{"message":"ok"},"meta":{}}`,
			wantCode: 0,
			wantOut:  "user approved",
		},
		{
			name:     "approve - unverified email refuses with 409",
			args:     []string{"users", "approve", "7"},
			path:     "/api/v1/users/7/approve",
			status:   http.StatusConflict,
			body:     `{"error":{"code":"EMAIL_NOT_VERIFIED","message":"User email is not verified yet"}}`,
			wantCode: 5,
			wantErr:  "not verified",
		},
		{
			name:     "reject - requires --confirm",
			args:     []string{"users", "reject", "7"},
			wantCode: 2,
			wantErr:  "--confirm is required",
		},
		{
			name:     "reject - sends confirmation",
			args:     []string{"users", "reject", "7", "--confirm"},
			path:     "/api/v1/users/7/reject",
			status:   http.StatusOK,
			body:     `{"data":{"message":"User rejected successfully"},"meta":{}}`,
			wantCode: 0,
			wantOut:  "user rejected",
			wantBody: `"confirmed":true`,
		},
		{
			name:     "suspend - sends reason",
			args:     []string{"users", "suspend", "7", "--reason", "spam"},
			path:     "/api/v1/users/7/suspend",
			status:   http.StatusOK,
			body:     `{"data":{"message":"User suspended successfully"},"meta":{}}`,
			wantCode: 0,
			wantOut:  "user suspended",
			wantBody: `"reason":"spam"`,
		},
		{
			name:     "delete - requires --confirm",
			args:     []string{"users", "delete", "7"},
			wantCode: 2,
			wantErr:  "--confirm is required",
		},
		{
			name:     "delete - sends confirmation",
			args:     []string{"users", "delete", "7", "--confirm"},
			path:     "/api/v1/users/7/soft-delete",
			status:   http.StatusOK,
			body:     `{"data":{"message":"User deleted"},"meta":{}}`,
			wantCode: 0,
			wantOut:  "user deleted",
			wantBody: `"confirmed":true`,
		},
		{
			name:     "error - bad id never hits the wire",
			args:     []string{"users", "approve", "abc"},
			wantCode: 5,
			wantErr:  "invalid user id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotBody string
			var called bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				gotPath = r.URL.Path
				raw, _ := io.ReadAll(r.Body)
				gotBody = string(raw)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			args := append(append([]string{}, tt.args...), "--base-url", srv.URL, "--api-key", "k")
			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(args, strings.NewReader(""), &out, &errOut)

			require.Equal(t, tt.wantCode, code, "stderr: %s", errOut)
			if tt.wantOut != "" {
				assert.Contains(t, out.String(), tt.wantOut)
			}
			if tt.wantErr != "" {
				assert.Contains(t, errOut.String(), tt.wantErr)
			}
			if tt.path != "" {
				require.True(t, called, "expected an HTTP call")
				assert.Equal(t, tt.path, gotPath)
			} else {
				assert.False(t, called, "validation must precede the wire")
			}
			if tt.wantBody != "" {
				assert.Contains(t, gotBody, tt.wantBody)
			}
		})
	}
}
