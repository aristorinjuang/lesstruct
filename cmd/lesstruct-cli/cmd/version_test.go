package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersion_FlagPrintsVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{"--version"},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)

	// Cobra's default version template renders "<name> version <version>". In
	// test builds no -ldflags are injected, so the value is the "dev" default.
	rendered := out.String()
	assert.Contains(t, rendered, "lesstruct-cli")
	assert.Contains(t, rendered, "version")
	assert.Contains(t, rendered, "dev")
}

func TestVersion_NoFlagRunsNormally(t *testing.T) {
	// `lesstruct-cli` with no args and no --version still behaves as before
	// (prints help to stdout, exits 0) — the auto-added flag is non-disruptive.
	var out, errOut bytes.Buffer
	code := cmd.ExecuteArgs(
		[]string{},
		strings.NewReader(""),
		&out,
		&errOut,
	)
	require.Equal(t, 0, code, "stderr: %s", errOut)
	assert.NotContains(t, out.String(), "version dev")
}
