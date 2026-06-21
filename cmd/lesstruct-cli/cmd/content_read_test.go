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

// recordingServer returns a test server that captures the method+path it
// received and responds with the given status+body. Used by content read
// tests; reused for get/list/delete (the small response-shape differences
// are per-test).
type recordingServer struct {
	method    string
	path      string
	auth      string
	rawQuery  string
}

func (r *recordingServer) handler(t *testing.T, status int, body string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.method = req.Method
		r.path = req.URL.Path
		r.rawQuery = req.URL.RawQuery
		r.auth = req.Header.Get("Authorization")
		w.WriteHeader(status)
		if body != "" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}
	})
}

func TestContentGet_Success(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":{"content":{"id":7,"title":"Hello","slug":"hello","status":"published"}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "get", "7", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "GET", rec.method)
	assert.Equal(t, "/api/v1/content/7", rec.path)
	assert.Equal(t, "Bearer k", rec.auth)
	assert.Contains(t, out.String(), "#7")
	assert.Contains(t, out.String(), "Hello")
	assert.Contains(t, out.String(), "published")
}

func TestContentGet_JSON(t *testing.T) {
	const env = `{"data":{"content":{"id":7,"title":"Hello","slug":"hello","status":"published"}}}`
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, env))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "get", "7", "--output", "json", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Contains(t, out.String(), `"data":{"content":{"id":7`)
}

func TestContentGet_NotFound(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusNotFound, `{"error":{"code":"NOT_FOUND","message":"Content not found"}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "get", "999", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 4, code)
	assert.Contains(t, errOut.String(), "not found")
}

func TestContentGet_Unauthorized(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusUnauthorized, `{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "get", "7", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 3, code)
}

func TestContentGet_BadID(t *testing.T) {
	// Bad id must NOT make an HTTP call. We point --base-url at a server that
	// fails the test if it receives any request.
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "get", "abc", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "invalid content id")
	assert.False(t, called, "bad id must not produce an HTTP call")
}

func TestContentGet_ZeroID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called for id=0")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "get", "0", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
}

func TestContentGet_ExtraArgsExitsTwo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when extra positional arg is given")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "get", "7", "8", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 2, code)
}

func TestContentGet_InvalidOutputExitsTwo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when --output is invalid")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "get", "7", "--output", "xml", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 2, code)
}

func TestContentGet_MissingKeyExitsOne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when API key is missing")
	}))
	defer srv.Close()
	withNoCredentials(t)

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "get", "7", "--base-url", srv.URL},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "no API key found")
}
