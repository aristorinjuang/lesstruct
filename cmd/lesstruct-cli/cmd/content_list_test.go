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

func TestContentList_Success(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[
		{"id":7,"title":"A","slug":"a","status":"draft"},
		{"id":6,"title":"B","slug":"b","status":"published"}
	],"meta":{"pagination":{"nextCursor":"Ng","hasMore":true}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "list", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "GET", rec.method)
	assert.Equal(t, "/api/v1/content", rec.path)
	assert.Equal(t, "", rec.rawQuery, "no --limit / --cursor → empty query string")
	outStr := out.String()
	assert.Contains(t, outStr, "Found 2 item(s)")
	assert.Contains(t, outStr, "hasMore=true")
	assert.Contains(t, outStr, `nextCursor="Ng"`)
	assert.Contains(t, outStr, "#7")
	assert.Contains(t, outStr, "#6")
	assert.Contains(t, outStr, "draft")
	assert.Contains(t, outStr, "published")
}

func TestContentList_EmptyList(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[],"meta":{"pagination":{"hasMore":false}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "list", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	outStr := out.String()
	assert.Contains(t, outStr, "Found 0 item(s)")
	assert.Contains(t, outStr, "hasMore=false")
}

func TestContentList_LimitFlag(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[],"meta":{"pagination":{"hasMore":false}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "list", "--limit", "10", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "limit=10", rec.rawQuery)
}

func TestContentList_CursorFlag(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[],"meta":{"pagination":{"hasMore":false}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "list", "--cursor", "Ng", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "cursor=Ng", rec.rawQuery)
}

func TestContentList_LimitAndCursor(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[],"meta":{"pagination":{"hasMore":false}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "list", "--limit", "5", "--cursor", "Ng", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	// url.Values.Encode sorts keys alphabetically: cursor < limit.
	assert.Equal(t, "cursor=Ng&limit=5", rec.rawQuery)
}

func TestContentList_InvalidCursor(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusBadRequest, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid cursor"}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "list", "--cursor", "garbage", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 5, code)
	assert.Contains(t, errOut.String(), "Invalid cursor")
}

func TestContentList_JSON(t *testing.T) {
	const env = `{"data":[{"id":7,"title":"A","slug":"a","status":"draft"}],"meta":{"pagination":{"hasMore":false}}}`
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, env))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "list", "--output", "json", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Contains(t, out.String(), `"data":[{"id":7,"title":"A"`)
	assert.Contains(t, out.String(), `"meta":{"pagination":{"hasMore":false}}`)
}

func TestContentList_ExtraArgsExitsTwo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called with extra positional arg")
	}))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "list", "1", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.Equal(t, 2, code)
}

func TestContentList_TagsFilter(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[],"meta":{"pagination":{"hasMore":false}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "list",
			"--tags", "go",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "tag=go", rec.rawQuery)
}

func TestContentList_TagsFilterCommaSeparated(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[],"meta":{"pagination":{"hasMore":false}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "list",
			"--tags", "go,tutorial",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "tag=go&tag=tutorial", rec.rawQuery)
}

func TestContentList_TagsFilterRepeated(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[],"meta":{"pagination":{"hasMore":false}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "list",
			"--tags", "go",
			"--tags", "tutorial",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "tag=go&tag=tutorial", rec.rawQuery)
}

func TestContentList_TagsFilterNormalized(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[],"meta":{"pagination":{"hasMore":false}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	// Trailing comma → empties dropped; duplicates collapsed to first occurrence.
	code := cmd.ExecuteArgs(
		[]string{
			"content", "list",
			"--tags", "go,,tutorial,go",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "tag=go&tag=tutorial", rec.rawQuery)
}

func TestContentList_LanguageFlag(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[],"meta":{"pagination":{"hasMore":false}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "list",
			"--language", "en",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "language=en", rec.rawQuery)
}

func TestContentList_StatusFlag(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[],"meta":{"pagination":{"hasMore":false}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "list",
			"--status", "draft",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "status=draft", rec.rawQuery)
}

func TestContentList_PostTypeFlag(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[],"meta":{"pagination":{"hasMore":false}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "list",
			"--post-type", "post",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "post_type=post", rec.rawQuery)
}

func TestContentList_AuthorFlag(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[],"meta":{"pagination":{"hasMore":false}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "list",
			"--author", "alice",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "author=alice", rec.rawQuery)
}

func TestContentList_SearchFlag(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[],"meta":{"pagination":{"hasMore":false}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "list",
			"--search", "golang",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	assert.Equal(t, "search=golang", rec.rawQuery)
}

func TestContentList_AuthorFlagNonAdminReturns403(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusForbidden, `{"error":{"code":"FORBIDDEN","message":"author filter is only available for admins"}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "list",
			"--author", "alice",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	// The CLI's exit-code scheme (cmd/lesstruct-cli/internal/client/exitcode.go)
	// maps 4xx → ExitValidation (5) — there is no separate Forbidden exit code
	// today. The surface signal is the FORBIDDEN code + message printed to stderr.
	assert.Equal(t, 5, code, "4xx maps to ExitValidation in the documented scheme")
	assert.Contains(t, errOut.String(), "author filter is only available for admins")
}

func TestContentList_AllFiltersCombined(t *testing.T) {
	rec := &recordingServer{}
	srv := httptest.NewServer(rec.handler(t, http.StatusOK, `{"data":[],"meta":{"pagination":{"hasMore":false}}}`))
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "list",
			"--tags", "go,tutorial",
			"--language", "en",
			"--status", "draft",
			"--post-type", "post",
			"--author", "alice",
			"--search", "golang",
			"--limit", "10",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut.String())
	// url.Values.Encode sorts keys alphabetically: author < language < limit < post_type < search < status < tag.
	assert.Equal(t,
		"author=alice&language=en&limit=10&post_type=post&search=golang&status=draft&tag=go&tag=tutorial",
		rec.rawQuery,
	)
}
