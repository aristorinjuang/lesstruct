package cmd_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/stretchr/testify/assert"
)

func TestContentDelete_Success(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusNoContent, ""))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "delete", "7", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "DELETE", rec.method)
	assert.Equal(t, "/api/v1/content/7", rec.path)
	assert.Contains(t, out.String(), "Deleted content #7")
}

func TestContentDelete_JSON_EmptyStdout(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusNoContent, ""))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "delete", "7", "--output", "json", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "", out.String(), "204 with --output json must produce empty stdout")
}

func TestContentDelete_NotFound(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusNotFound, `{"error":{"code":"NOT_FOUND","message":"Content not found"}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "delete", "999", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 4, code)
	assert.Contains(t, errOut.String(), "not found")
}

func TestContentDelete_BadID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called for non-numeric id")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "delete", "abc", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "invalid content id")
}

func TestContentDelete_Unauthorized(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusUnauthorized, `{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "delete", "7", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 3, code)
}

func TestContentDelete_MissingKeyExitsOne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when API key is missing")
	}))
	defer srv.Close()
	withNoCredentials(t)

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "delete", "7", "--base-url", srv.URL},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "no API key found")
}

func TestContentDelete_ExtraArgsExitsTwo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called with extra positional arg")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "delete", "7", "8", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 2, code)
}
