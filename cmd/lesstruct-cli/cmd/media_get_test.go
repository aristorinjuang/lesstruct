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

func TestMediaGet_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/media/5", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"media":{"id":5,"filename":"photo.jpg","mimeType":"image/jpeg","fileSize":11,"altText":"a view","url":"http://example/uploads/photo.jpg"}}}`))
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "get", "5", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	outStr := out.String()
	assert.Contains(t, outStr, "Media #5")
	assert.Contains(t, outStr, "photo.jpg")
	assert.Contains(t, outStr, "image/jpeg")
}

func TestMediaGet_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"NOT_FOUND","message":"Media not found"}}`))
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "get", "999", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 4, code)
	assert.Contains(t, errOut.String(), "not found")
}

func TestMediaGet_BadID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called for non-numeric id")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "get", "abc", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "invalid media id")
}

func TestMediaGet_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`))
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "get", "5", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 3, code)
}

func TestMediaGet_ExtraArgsExitsTwo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called with extra positional arg")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "get", "5", "8", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 2, code)
}

func TestMediaGet_MissingKeyExitsOne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when API key missing")
	}))
	defer srv.Close()
	withNoCredentials(t)

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "get", "5", "--base-url", srv.URL},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "no API key found")
}

func TestMediaGet_VariantsInTextOutput(t *testing.T) {
	// Thumbnail variants (map keyed by suffix) must render in text output, one
	// per line, sorted by key for deterministic ordering. The map arrives in an
	// arbitrary (Go-map) order, so sorting is what makes the output stable.
	body := `{"data":{"media":{"id":5,"filename":"photo.jpg","mimeType":"image/jpeg","fileSize":11,"altText":"a view","url":"http://example/uploads/photo.jpg","variants":{"_thumb":{"url":"http://example/uploads/_thumb_photo.jpg","width":150,"height":150},"_medium":{"url":"http://example/uploads/_medium_photo.jpg","width":300,"height":300},"_large":{"url":"http://example/uploads/_large_photo.jpg","width":1024,"height":1024}}}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"media", "get", "5", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())

	outStr := out.String()
	assert.Contains(t, outStr, "variant _large: url=http://example/uploads/_large_photo.jpg 1024x1024")
	assert.Contains(t, outStr, "variant _medium: url=http://example/uploads/_medium_photo.jpg 300x300")
	assert.Contains(t, outStr, "variant _thumb: url=http://example/uploads/_thumb_photo.jpg 150x150")

	// Sorted by key: _large < _medium < _thumb.
	idxLarge := strings.Index(outStr, "variant _large")
	idxMedium := strings.Index(outStr, "variant _medium")
	idxThumb := strings.Index(outStr, "variant _thumb")
	require.True(t, idxLarge > -1 && idxMedium > -1 && idxThumb > -1, "all variant lines present")
	assert.Less(t, idxLarge, idxMedium, "_large before _medium")
	assert.Less(t, idxMedium, idxThumb, "_medium before _thumb")
}
