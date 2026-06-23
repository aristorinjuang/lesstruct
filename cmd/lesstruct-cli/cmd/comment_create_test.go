package cmd_test

import (
	"bytes"
	"encoding/json"
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

// commentEnvelope is the agent comment-create success envelope (a single
// comment projection under data.comment). Reused by the create + moderate tests.
const commentEnvelope = `{"data":{"comment":{"id":9,"comment":"Nice!","author":"Author 42","username":"alice","status":"pending","createdAt":"2026-06-23T12:00:00Z"}}}`

func TestCommentCreate_PositionalArg(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, commentEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "create", "5", "Nice!", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, http.MethodPost, info.method)
	assert.Equal(t, "/api/v1/content/5/comments", info.path)
	assert.Equal(t, "Nice!", info.payload["comment"])
}

func TestCommentCreate_FromFile(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, commentEnvelope, &info)
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "comment.txt")
	require.NoError(t, os.WriteFile(path, []byte("From a file."), 0o600))

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "create", "5", "--file", path, "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "From a file.", info.payload["comment"])
}

func TestCommentCreate_FromStdin(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, commentEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "create", "5", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader("Piped comment"),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "Piped comment", info.payload["comment"])
}

func TestCommentCreate_OutputJSON(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, commentEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "create", "5", "Nice!", "--output", "json", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())

	var printed struct {
		Data struct {
			Comment struct {
				ID int `json:"id"`
			} `json:"comment"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &printed))
	assert.Equal(t, 9, printed.Data.Comment.ID)
}

func TestCommentCreate_OutputText(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, commentEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "create", "5", "Nice!", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Contains(t, out.String(), "#9")
	assert.Contains(t, out.String(), "content #5")
	assert.Contains(t, out.String(), "pending")
	assert.Empty(t, errOut.String())
}

func TestCommentCreate_ValidationExitsFive(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(
		t,
		http.StatusBadRequest,
		`{"error":{"code":"VALIDATION_ERROR","message":"comment text is required"}}`,
		&info,
	)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "create", "5", "x", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "comment text is required")
	assert.Empty(t, out.String())
}

func TestCommentCreate_ForbiddenExitsFive(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(
		t,
		http.StatusForbidden,
		`{"error":{"code":"FORBIDDEN","message":"Comments are not allowed on this content"}}`,
		&info,
	)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "create", "5", "x", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code, "403 maps to exit 5 (validation)")
	assert.Contains(t, errOut.String(), "Comments are not allowed")
}

func TestCommentCreate_NotFoundExitsFour(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(
		t,
		http.StatusNotFound,
		`{"error":{"code":"NOT_FOUND","message":"Content not found"}}`,
		&info,
	)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "create", "999", "x", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 4, code, "404 maps to exit 4 (not-found)")
}

func TestCommentCreate_AuthExitsThree(t *testing.T) {
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
		[]string{"comment", "create", "5", "x", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 3, code)
}

func TestCommentCreate_EmptyBodyExitsFive(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, commentEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "create", "5", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader("   \n\n  "),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "no comment text provided")
}

func TestCommentCreate_BadContentIDExitsFive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called for non-numeric content id")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"comment", "create", "abc", "x", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "invalid content id")
}
