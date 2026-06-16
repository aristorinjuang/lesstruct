package cmd_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newUpdateServer mirrors newCreateServer (in content_create_test.go) but is
// named for the update path. The shared `requestInfo` + `newCreateServer`
// helper records the method/path/body which is exactly what update needs — the
// server doesn't care that the helper's name says "Create", only the test
// asserts on the captured values.
const updateSuccessEnvelope = `{"data":{"content":{"id":7,"title":"Hello v2","slug":"hello-v2","status":"published"}}}`

func TestContentUpdate_PositionalBody(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, updateSuccessEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "# Hello v2", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, http.MethodPut, info.method)
	assert.Equal(t, "/api/v1/content/7", info.path)
	assert.Equal(t, "# Hello v2", info.payload["body"])
	assert.Equal(t, "markdown", info.payload["format"])
	assert.Equal(t, "Hello v2", info.payload["title"], "title derived from the first heading")
	_, hasPublished := info.payload["isPublished"]
	assert.False(t, hasPublished, "isPublished omitted when --published absent")
	assert.Contains(t, out.String(), "Updated content #7")
	assert.Contains(t, out.String(), `hello-v2`)
	assert.Contains(t, out.String(), "published")
	assert.Empty(t, errOut.String())
}

func TestContentUpdate_FromFile(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, updateSuccessEnvelope, &info)
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "post.md")
	require.NoError(t, os.WriteFile(path, []byte("# From File\n\nBody text."), 0o600))

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "--file", path, "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, "# From File\n\nBody text.", info.payload["body"])
	assert.Equal(t, "From File", info.payload["title"])
}

func TestContentUpdate_FromStdin(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, updateSuccessEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader("Body from stdin only\n\nMore."),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, "Body from stdin only\n\nMore.", info.payload["body"])
	assert.Equal(t, "Body from stdin only", info.payload["title"], "title derived from first non-empty line when no heading")
}

func TestContentUpdate_TitleFlag(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, updateSuccessEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "# Heading", "--title", "Custom Title", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, "Custom Title", info.payload["title"])
}

func TestContentUpdate_PublishedFlag(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, updateSuccessEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "# Hi", "--published", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, true, info.payload["isPublished"])
}

func TestContentUpdate_FileOverridesPositional(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, updateSuccessEnvelope, &info)
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "post.md")
	require.NoError(t, os.WriteFile(path, []byte("# From File"), 0o600))

	var out, errOut bytes.Buffer
	// When both --file and a positional body are given, --file wins (the
	// precedence rule matches `content create`).
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "from positional", "--file", path, "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader("from stdin"),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, "# From File", info.payload["body"])
}

func TestContentUpdate_OutputJSON(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, updateSuccessEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "# Hi", "--output", "json", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)

	var printed struct {
		Data struct {
			Content struct {
				ID     int    `json:"id"`
				Title  string `json:"title"`
				Slug   string `json:"slug"`
				Status string `json:"status"`
			} `json:"content"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &printed))
	assert.Equal(t, 7, printed.Data.Content.ID)
	assert.Equal(t, "Hello v2", printed.Data.Content.Title)
	assert.Equal(t, "hello-v2", printed.Data.Content.Slug)
	assert.Equal(t, "published", printed.Data.Content.Status)
}

func TestContentUpdate_BadID(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "abc", "body", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "invalid content id")
	assert.False(t, called, "bad id must not produce an HTTP call")
}

func TestContentUpdate_ZeroID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called for id=0")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "0", "body", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
}

func TestContentUpdate_NotFound(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusNotFound, `{"error":{"code":"NOT_FOUND","message":"Content not found"}}`, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "999", "body", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 4, code)
	assert.Contains(t, errOut.String(), "not found")
}

func TestContentUpdate_Unauthorized(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusUnauthorized, `{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "body", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 3, code)
}

func TestContentUpdate_ValidationErrorFromServer(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusBadRequest, `{"error":{"code":"VALIDATION_ERROR","message":"body is required"}}`, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "body", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "body is required")
	assert.Empty(t, out.String(), "no data on stdout for an error")
}

func TestContentUpdate_EmptyBodyExitsFive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when body is empty")
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader("   \n\n  "),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "no markdown body provided")
}

func TestContentUpdate_EmptyBodyFromFileExitsFive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when --file is empty")
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	require.NoError(t, os.WriteFile(path, []byte("   \n\n  "), 0o600))

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "--file", path, "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "no markdown body provided")
}

func TestContentUpdate_InvalidOutputExitsTwo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when --output is invalid")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "body", "--output", "xml", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut.String(), "--output")
}

func TestContentUpdate_ExtraArgsExitsTwo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called with extra positional arg")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "body", "extra", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 2, code)
}

func TestContentUpdate_MissingKeyExitsOne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when API key is missing")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "body", "--base-url", srv.URL},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "no API key found")
}

func TestContentUpdate_PostTypeFlag(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, updateSuccessEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "update", "7", "# Hi",
			"--post-type", "post",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, "post", info.payload["postType"])
}

func TestContentUpdate_TagsFlag(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, updateSuccessEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "update", "7", "# Hi",
			"--tags", "alpha,beta",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	gotTags, ok := info.payload["tags"].([]any)
	require.True(t, ok, "tags must be a JSON array on the wire")
	assert.Equal(t, []any{"alpha", "beta"}, gotTags)
}

func TestContentUpdate_TagsFlagNormalizesDedupes(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, updateSuccessEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "update", "7", "# Hi",
			"--tags", "x, x, y, , z",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	gotTags, ok := info.payload["tags"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"x", "y", "z"}, gotTags)
}

func TestContentUpdate_LanguageFlag(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, updateSuccessEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "update", "7", "# Hi",
			"--language", "en",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, "en", info.payload["language"])
}

func TestContentUpdate_NoFlagsOmitsAll(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, updateSuccessEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "# Hi", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	// omitempty on the request: when the flags are not set, the keys must be
	// absent from the wire payload. The server's update handler then
	// preserves the existing item's postType/tags/language — the existing
	// v1 contract.
	_, hasPostType := info.payload["postType"]
	assert.False(t, hasPostType, "postType must be absent when --post-type is not set")
	_, hasTags := info.payload["tags"]
	assert.False(t, hasTags, "tags must be absent when --tags is not set")
	_, hasLanguage := info.payload["language"]
	assert.False(t, hasLanguage, "language must be absent when --language is not set")
}

func TestContentUpdate_ServerValidationOnLanguage(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(
		t,
		http.StatusBadRequest,
		`{"error":{"code":"VALIDATION_ERROR","message":"language is not in the configured languages list"}}`,
		&info,
	)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "update", "7", "# Hi",
			"--language", "xx",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code, "400 VALIDATION_ERROR must map to exit 5")
	assert.Contains(t, errOut.String(), "language is not in the configured languages list")
}
