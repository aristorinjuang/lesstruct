package cmd_test

import (
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-mcp/cmd"
	"github.com/stretchr/testify/assert"
)

func TestExecuteArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		env        map[string]string
		wantCode   int
		wantStderr string
	}{
		{
			name:       "success - version flag",
			args:       []string{"--version"},
			env:        nil,
			wantCode:   0,
			wantStderr: "",
		},
		{
			name:       "error - unknown flag",
			args:       []string{"--nope"},
			env:        nil,
			wantCode:   1,
			wantStderr: "unknown flag",
		},
		{
			name:       "error - missing API key",
			args:       []string{},
			env:        map[string]string{"LESSTRUCT_API_KEY": "", "LESSTRUCT_CONFIG": "/nonexistent-config.toml"},
			wantCode:   1,
			wantStderr: "no API key found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			var out, errOut strings.Builder
			code := cmd.ExecuteArgs(tt.args, &out, &errOut)
			assert.Equal(t, tt.wantCode, code)
			if tt.wantStderr != "" {
				assert.Contains(t, errOut.String(), tt.wantStderr)
			}
		})
	}
}
