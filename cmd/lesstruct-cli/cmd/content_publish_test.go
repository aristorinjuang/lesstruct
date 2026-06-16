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

const publishSuccessEnvelope = `{"data":{"content":{"id":7,"title":"Hello","slug":"hello","status":"published"}}}`
const unpublishSuccessEnvelope = `{"data":{"content":{"id":7,"title":"Hello","slug":"hello","status":"draft"}}}`

func TestContentPublish_Success(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, publishSuccessEnvelope))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "publish", "7", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api/v1/content/7/publish", rec.path)
	assert.Equal(t, "Bearer k", rec.auth)
	assert.Empty(t, rec.rawQuery, "publish sends no query string")
	outStr := out.String()
	assert.Contains(t, outStr, "Published content #7")
	assert.Contains(t, outStr, "Hello")
	assert.Contains(t, outStr, "published")
}

func TestContentPublish_JSON(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, publishSuccessEnvelope))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "publish", "7", "--output", "json", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Contains(t, out.String(), `"data":{"content":{"id":7`)
	assert.Contains(t, out.String(), `"status":"published"`)
}

func TestContentPublish_IdempotentNoOpBody(t *testing.T) {
	// Server returns 200 with no body (defensive case the client treats as
	// success-without-data). The CLI surfaces a clear error rather than
	// printing a bogus "Published content #0" line — defense in depth, since
	// the publish endpoint always returns a ContentResponse on 200.
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, ""))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "publish", "7", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 1, code, "empty body should map to ExitGeneric (server contract violation)")
	assert.Contains(t, errOut.String(), "server returned no content")
	_ = rec // recordingServer captures the call so we can assert method/path above if needed
}

func TestContentPublish_NotFound(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusNotFound, `{"error":{"code":"NOT_FOUND","message":"Content not found"}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "publish", "999", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 4, code)
	assert.Contains(t, errOut.String(), "not found")
}

func TestContentPublish_Unauthorized(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusUnauthorized, `{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "publish", "7", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 3, code)
}

func TestContentPublish_BadID(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "publish", "abc", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "invalid content id")
	assert.False(t, called, "bad id must not produce an HTTP call")
}

func TestContentPublish_ZeroID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called for id=0")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "publish", "0", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
}

func TestContentPublish_MissingIDExitsTwo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when no id is given")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "publish", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 2, code)
}

func TestContentPublish_ExtraArgsExitsTwo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when extra positional arg is given")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "publish", "7", "extra", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 2, code)
}

func TestContentPublish_NoBodyFlags(t *testing.T) {
	// Defensive: the publish verb must not accept body flags like --file or
	// --published. Asserting on the absence of those flags keeps the surface
	// minimal — a user who needs body flags belongs on `content update`.
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, publishSuccessEnvelope))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		// Intentionally add unknown flags to surface a usage error.
		[]string{"content", "publish", "7", "--file", "/tmp/x.md", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	// Cobra returns ExitUsage (2) for unknown flags.
	assert.Equal(t, 2, code)
}

func TestContentUnpublish_Success(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, unpublishSuccessEnvelope))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "unpublish", "7", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api/v1/content/7/unpublish", rec.path)
	assert.Equal(t, "Bearer k", rec.auth)
	assert.Empty(t, rec.rawQuery, "unpublish sends no query string")
	outStr := out.String()
	assert.Contains(t, outStr, "Unpublished content #7")
	assert.Contains(t, outStr, "Hello")
	assert.Contains(t, outStr, "draft")
}

func TestContentUnpublish_JSON(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, unpublishSuccessEnvelope))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "unpublish", "7", "--output", "json", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Contains(t, out.String(), `"data":{"content":{"id":7`)
	assert.Contains(t, out.String(), `"status":"draft"`)
}

func TestContentUnpublish_NotFound(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusNotFound, `{"error":{"code":"NOT_FOUND","message":"Content not found"}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "unpublish", "999", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 4, code)
	assert.Contains(t, errOut.String(), "not found")
}

func TestContentUnpublish_Unauthorized(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusUnauthorized, `{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "unpublish", "7", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 3, code)
}

func TestContentUnpublish_BadID(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "unpublish", "abc", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "invalid content id")
	assert.False(t, called, "bad id must not produce an HTTP call")
}

func TestContentUnpublish_ZeroID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called for id=0")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "unpublish", "0", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
}

func TestContentUnpublish_MissingIDExitsTwo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when no id is given")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "unpublish", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 2, code)
}

func TestContentUnpublish_ExtraArgsExitsTwo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when extra positional arg is given")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "unpublish", "7", "extra", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 2, code)
}
