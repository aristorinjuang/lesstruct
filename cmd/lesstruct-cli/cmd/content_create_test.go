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

// requestInfo captures what the test server received.
type requestInfo struct {
	method  string
	path    string
	payload map[string]any
}

// newCreateServer returns a test server that records the request and responds
// with the given status + body. The recorded request is written to *info.
func newCreateServer(t *testing.T, status int, body string, info *requestInfo) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info.method = r.Method
		info.path = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &info.payload)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

const successEnvelope = `{"data":{"content":{"id":7,"title":"Hello","slug":"hello","status":"draft"}}}`

func TestContentCreate_PositionalArg(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "create", "# Hello", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, http.MethodPost, info.method)
	assert.Equal(t, "/api/v1/content", info.path)
	assert.Equal(t, "# Hello", info.payload["body"])
	assert.Equal(t, "markdown", info.payload["format"])
	// Title derived from the first "# " line.
	assert.Equal(t, "Hello", info.payload["title"])
	_, hasPublished := info.payload["isPublished"]
	assert.False(t, hasPublished, "isPublished omitted when --published absent")
}

func TestContentCreate_FromFile(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "post.md")
	require.NoError(t, os.WriteFile(path, []byte("# From File\n\nBody text."), 0o600))

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "create", "--file", path, "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, "# From File\n\nBody text.", info.payload["body"])
	assert.Equal(t, "From File", info.payload["title"])
}

func TestContentCreate_FromStdin(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "create", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader("First line is the title\n\nMore."),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, "First line is the title\n\nMore.", info.payload["body"])
	assert.Equal(t, "First line is the title", info.payload["title"])
}

func TestContentCreate_PublishedFlag(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "create", "# Hi", "--published", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, true, info.payload["isPublished"])
}

func TestContentCreate_TitleFlagOverridesDerivation(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "create", "# Heading", "--title", "Custom Title", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, "Custom Title", info.payload["title"])
}

func TestContentCreate_OutputJSON(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "create", "# Hi", "--output", "json", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)

	// --output json echoes the server envelope {data, meta}.
	var printed struct {
		Data struct {
			Content struct {
				ID int `json:"id"`
			} `json:"content"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &printed))
	assert.Equal(t, 7, printed.Data.Content.ID)
}

func TestContentCreate_OutputText(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "create", "# Hi", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Contains(t, out.String(), "#7")
	assert.Contains(t, out.String(), "hello")
	// Diagnostics go to stderr, not stdout.
	assert.Empty(t, errOut.String())
}

func TestContentCreate_ValidationExitsFive(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(
		t,
		http.StatusBadRequest,
		`{"error":{"code":"VALIDATION_ERROR","message":"title is required"}}`,
		&info,
	)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "create", "x", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "title is required")
	assert.Empty(t, out.String(), "no data on stdout for an error")
}

func TestContentCreate_AuthExitsThree(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(
		t,
		http.StatusUnauthorized,
		`{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`,
		&info,
	)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "create", "x", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 3, code)
}

func TestContentCreate_EmptyBodyExitsFive(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "create", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader("   \n\n  "),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "no markdown body provided")
}

func TestContentCreate_InvalidOutputExitsTwo(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "create", "# Hi", "--output", "xml", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut.String(), "--output")
}

func TestContentCreate_PostTypeFlag(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "create", "# Hi",
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

func TestContentCreate_TagsFlag(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "create", "# Hi",
			"--tags", "foo,bar",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	gotTags, ok := info.payload["tags"].([]any)
	require.True(t, ok, "tags must be a JSON array on the wire")
	assert.Equal(t, []any{"foo", "bar"}, gotTags)
}

func TestContentCreate_TagsFlagNormalizesDedupes(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	// "a, a, b, , c" → trim each entry, drop the empty one, dedupe, preserve
	// first-occurrence order → ["a", "b", "c"].
	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "create", "# Hi",
			"--tags", "a, a, b, , c",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	gotTags, ok := info.payload["tags"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"a", "b", "c"}, gotTags)
}

func TestContentCreate_LanguageFlag(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "create", "# Hi",
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

func TestContentCreate_NoFlagsOmitsAll(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "create", "# Hi", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	// omitempty on the request: when the flags are not set, the keys must be
	// absent from the wire payload (the server then keeps the existing value
	// for update, or the server default for create).
	_, hasPostType := info.payload["postType"]
	assert.False(t, hasPostType, "postType must be absent when --post-type is not set")
	_, hasTags := info.payload["tags"]
	assert.False(t, hasTags, "tags must be absent when --tags is not set")
	_, hasLanguage := info.payload["language"]
	assert.False(t, hasLanguage, "language must be absent when --language is not set")
}

func TestContentCreate_ServerValidationOnLanguage(t *testing.T) {
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
			"content", "create", "# Hi",
			"--language", "xx",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code, "400 VALIDATION_ERROR must map to exit 5")
	assert.Contains(t, errOut.String(), "language is not in the configured languages list")
	assert.Empty(t, out.String(), "no data on stdout for an error")
}
