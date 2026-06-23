package cmd_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommentModerate_Verbs covers the three admin moderation verbs end-to-end:
// each sends PUT /api/v1/content/{id}/comments/{commentId}/status with the right
// status payload and prints the verb + the echoed status.
func TestCommentModerate_Verbs(t *testing.T) {
	tests := []struct {
		name       string
		verb       string // CLI subcommand
		wantStatus string // wire status payload + echoed status
		wantVerb   string // text-output prefix
	}{
		{name: "approve", verb: "approve", wantStatus: "approved", wantVerb: "Approved"},
		{name: "reject", verb: "reject", wantStatus: "rejected", wantVerb: "Rejected"},
		{name: "spam", verb: "spam", wantStatus: "spam", wantVerb: "Marked spam"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var info requestInfo
			body := `{"data":{"comment":{"id":9,"comment":"ok","username":"alice","status":"` + tt.wantStatus + `","createdAt":"2026-06-23T12:00:00Z"}}}`
			srv := newCreateServer(t, http.StatusOK, body, &info)
			defer srv.Close()

			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(
				[]string{"comment", tt.verb, "5", "9", "--base-url", srv.URL, "--api-key", "k"},
				strings.NewReader(""),
				&out,
				&errOut,
			)
			require.Equal(t, 0, code, "stderr: %s", errOut.String())
			assert.Equal(t, http.MethodPut, info.method)
			assert.Equal(t, "/api/v1/content/5/comments/9/status", info.path)
			assert.Equal(t, tt.wantStatus, info.payload["status"])
			assert.Contains(t, out.String(), tt.wantVerb)
			assert.Contains(t, out.String(), "comment #9")
			assert.Contains(t, out.String(), tt.wantStatus)
		})
	}
}

func TestCommentModerate_ForbiddenExitsFive(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(
		t,
		http.StatusForbidden,
		`{"error":{"code":"FORBIDDEN","message":"comment moderation is admin-only"}}`,
		&info,
	)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "approve", "5", "9", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code, "403 maps to exit 5 (validation)")
	assert.Contains(t, errOut.String(), "admin-only")
}

func TestCommentModerate_NotFoundExitsFour(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(
		t,
		http.StatusNotFound,
		`{"error":{"code":"NOT_FOUND","message":"Comment not found"}}`,
		&info,
	)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "reject", "5", "999", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 4, code)
}

func TestCommentModerate_BadCommentIDExitsFive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called for non-numeric comment id")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "spam", "5", "abc", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "invalid comment id")
}

func TestCommentModerate_MissingKeyExitsOne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when API key is missing")
	}))
	defer srv.Close()
	withNoCredentials(t)

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "approve", "5", "9", "--base-url", srv.URL},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "no API key found")
}
