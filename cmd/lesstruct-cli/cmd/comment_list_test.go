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

const commentListEnvelope = `{"data":[` +
	`{"id":1,"comment":"ok","username":"alice","status":"approved","createdAt":"2026-06-23T12:00:00Z"},` +
	`{"id":2,"comment":"waiting","username":"bob","status":"pending","createdAt":"2026-06-23T12:01:00Z"}` +
	`]}`

func TestCommentList_Success(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, commentListEnvelope))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "list", "5", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "GET", rec.method)
	assert.Equal(t, "/api/v1/content/5/comments", rec.path)
	assert.Contains(t, out.String(), "Found 2 comment(s) on content #5")
	assert.Contains(t, out.String(), "#1")
	assert.Contains(t, out.String(), "alice")
}

func TestCommentList_Empty(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[]}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "list", "5", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Contains(t, out.String(), "Found 0 comment(s) on content #5")
}

func TestCommentList_OutputJSON(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, commentListEnvelope))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "list", "5", "--output", "json", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Contains(t, out.String(), `"data":`)
}

func TestCommentList_NotFoundExitsFour(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusNotFound, `{"error":{"code":"NOT_FOUND","message":"Content not found"}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "list", "999", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 4, code)
	assert.Contains(t, errOut.String(), "not found")
}

func TestCommentList_BadContentIDExitsFive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called for non-numeric content id")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "list", "abc", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "invalid content id")
}
