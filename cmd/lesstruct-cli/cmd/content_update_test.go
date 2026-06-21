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

// newPatchServer replies to GET /api/v1/content/{id} with the `existing`
// content projection (so runContentUpdate can carry omitted flags forward) and
// to PUT with updateSuccessEnvelope, recording the PUT payload into *put. The
// GET response is the raw projection object; it is wrapped in the data/content
// envelope here.
func newPatchServer(t *testing.T, existing string, put *requestInfo) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"content":` + existing + `}}`))
		case http.MethodPut:
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &put.payload)
			put.method = r.Method
			put.path = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(updateSuccessEnvelope))
		}
	}))
}

// richExisting is a content projection with every carry-forward field populated,
// used by patch-semantics tests: omitting --post-type/--tags/--language must
// preserve these, and omitting --published must preserve the published status.
const richExisting = `{"id":7,"title":"Original","status":"published","postType":"post","tags":["keep-a","keep-b"],"language":"en"}`

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
	// Patch semantics: --published is omitted, so the existing status ("published"
	// in the GET response) is carried forward → isPublished=true is sent.
	assert.Equal(t, true, info.payload["isPublished"], "omitted --published carries the existing status forward")
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
	withNoCredentials(t)

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

func TestContentUpdate_OmittedFlagsCarriedForward(t *testing.T) {
	// Patch semantics: omitting --post-type/--tags/--language/--published carries
	// the existing values forward (fetched via a GET), so a body-only edit no
	// longer wipes them and does not unpublish. (Replaces the old omit→absent
	// contract, which relied on the server to preserve them.)
	var put requestInfo
	srv := newPatchServer(t, richExisting, &put)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "# Hi", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, http.MethodPut, put.method)
	assert.Equal(t, "post", put.payload["postType"], "postType carried forward from existing")
	assert.Equal(t, "en", put.payload["language"], "language carried forward from existing")
	assert.Equal(t, true, put.payload["isPublished"], "published status carried forward from existing")
	gotTags, ok := put.payload["tags"].([]any)
	require.True(t, ok, "tags carried forward as a JSON array")
	assert.Equal(t, []any{"keep-a", "keep-b"}, gotTags, "tags carried forward from existing")
}

func TestContentUpdate_SetFlagsOverrideExisting(t *testing.T) {
	// Explicitly-set flags override the existing values; omitted flags still
	// carry forward. Here --post-type/--tags/--language are overridden while
	// --published is omitted, so the published status is carried forward. (Note
	// --published=false serializes as false+omitempty → omitted on the wire,
	// which the server reads as draft, so it can't be asserted distinctly from
	// omit here; the draft-preserve test covers that wire shape.)
	var put requestInfo
	srv := newPatchServer(t, richExisting, &put)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "update", "7", "# Hi",
			"--post-type", "page",
			"--tags", "new",
			"--language", "fr",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, "page", put.payload["postType"], "explicit --post-type overrides existing")
	assert.Equal(t, "fr", put.payload["language"], "explicit --language overrides existing")
	assert.Equal(t, true, put.payload["isPublished"], "omitted --published still carries forward the published status")
	gotTags, ok := put.payload["tags"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"new"}, gotTags, "explicit --tags overrides existing")
}

func TestContentUpdate_OmittedPublishedPreservesDraft(t *testing.T) {
	// Omitting --published preserves whatever the existing status is — here a
	// draft, so isPublished is false and omitted on the wire (not forced to true).
	const draftExisting = `{"id":7,"title":"Original","status":"draft","postType":"post","tags":["x"],"language":"en"}`
	var put requestInfo
	srv := newPatchServer(t, draftExisting, &put)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "update", "7", "# Hi", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	_, has := put.payload["isPublished"]
	assert.False(t, has, "draft status carried forward → isPublished omitted (false + omitempty)")
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

func TestContentUpdate_CustomFieldsFlag(t *testing.T) {
	t.Run("set sends customFields (replace)", func(t *testing.T) {
		var info requestInfo
		srv := newCreateServer(t, http.StatusOK, updateSuccessEnvelope, &info)
		defer srv.Close()

		var out, errOut bytes.Buffer
		code := cmd.ExecuteArgs(
			[]string{
				"content", "update", "7", "# Hi",
				"--field", "minutes=45",
				"--field", "has_video=false",
				"--base-url", srv.URL, "--api-key", "k",
			},
			strings.NewReader(""),
			&out,
			&errOut,
		)
		require.Equal(t, 0, code, "stderr: %s", errOut)
		fields, ok := info.payload["customFields"].(map[string]any)
		require.True(t, ok, "customFields must be a JSON object on the wire")
		assert.Equal(t, float64(45), fields["minutes"])
		assert.Equal(t, false, fields["has_video"])
	})

	t.Run("omitted sends nothing on the wire (preserves existing)", func(t *testing.T) {
		// The server preserves customFields when the request omits the map, so the
		// CLI must NOT send a nil/empty customFields key when --field is absent.
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
		_, has := info.payload["customFields"]
		assert.False(t, has, "customFields must be absent when --field is not set")
	})
}
