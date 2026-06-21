package cmd_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContentSystemFields_SendsPutWithSystemFieldsEnvelope verifies the CLI sends
// PUT /api/v1/content/{id}/system-fields with the fields wrapped under a
// "systemFields" key (mirroring the server's expected body shape).
func TestContentSystemFields_SendsPutWithSystemFieldsEnvelope(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "system-fields", "7", "--field", "editorial_status=published", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.Equal(t, http.MethodPut, info.method)
	assert.Equal(t, "/api/v1/content/7/system-fields", info.path)
	systemFields, ok := info.payload["systemFields"].(map[string]any)
	require.True(t, ok, "payload must wrap fields under systemFields, got %v", info.payload)
	assert.Equal(t, "published", systemFields["editorial_status"])
}

// TestContentSystemFields_AutoTypesValues verifies the repeatable --field values
// are coerced to bool/number/string exactly like create/update custom fields.
func TestContentSystemFields_AutoTypesValues(t *testing.T) {
	var info requestInfo
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &info)
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{
			"content", "system-fields", "7",
			"--field", "featured=true",
			"--field", "priority=5",
			"--field", "note=hello",
			"--base-url", srv.URL, "--api-key", "k",
		},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	systemFields, ok := info.payload["systemFields"].(map[string]any)
	require.True(t, ok, "payload must wrap fields under systemFields, got %v", info.payload)
	assert.Equal(t, true, systemFields["featured"])
	assert.Equal(t, float64(5), systemFields["priority"])
	assert.Equal(t, "hello", systemFields["note"])
}

// TestContentSystemFields_NoFieldsIsValidationExit verifies a no-op invocation
// (no --field) is rejected client-side with a validation exit code.
func TestContentSystemFields_NoFieldsIsValidationExit(t *testing.T) {
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &requestInfo{})
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "system-fields", "7", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.NotEqual(t, 0, code, "should exit non-zero with no --field")
	assert.Contains(t, errOut.String(), "no system fields provided")
}

// TestContentSystemFields_InvalidIDIsValidationExit verifies a non-numeric id is
// rejected client-side before any request is sent.
func TestContentSystemFields_InvalidIDIsValidationExit(t *testing.T) {
	srv := newCreateServer(t, http.StatusOK, successEnvelope, &requestInfo{})
	defer srv.Close()

	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"content", "system-fields", "abc", "--field", "x=1", "--base-url", srv.URL, "--api-key", "k"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	assert.NotEqual(t, 0, code)
	assert.Contains(t, errOut.String(), "invalid content id")
}
