package plugingen_test

import (
	"strings"
	"testing"

	plugingen "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProjectEnvelope(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "success - valid envelope",
			input:   `{"name":"my-plugin","language":"go","hooks":["before_save"],"capabilities":["log_info"],"rationale":"r","files":{"main.go":"package main\nfunc main() {} // hook_before_save","go.mod":"module x"}}`,
			wantErr: false,
		},
		{
			name:    "success - fenced envelope",
			input:   "```json\n" + `{"name":"my-plugin","language":"go","hooks":["before_save"],"rationale":"r","files":{"main.go":"package main\nfunc main() {} // hook_before_save","go.mod":"module x"}}` + "\n```",
			wantErr: false,
		},
		{
			name:    "error - empty response",
			input:   "",
			wantErr: true,
		},
		{
			name:    "error - invalid json",
			input:   `{not json`,
			wantErr: true,
		},
		{
			name:    "error - no files",
			input:   `{"name":"my-plugin","language":"go","hooks":["before_save"],"files":{}}`,
			wantErr: true,
		},
		{
			name:    "error - bad name",
			input:   `{"name":"Bad!","language":"go","hooks":["before_save"],"files":{"main.go":"x","go.mod":"y"}}`,
			wantErr: true,
		},
		{
			name:    "error - unsupported language",
			input:   `{"name":"my-plugin","language":"rust","hooks":["before_save"],"files":{"main.go":"x","go.mod":"y"}}`,
			wantErr: true,
		},
		{
			name:    "error - unknown hook",
			input:   `{"name":"my-plugin","language":"go","hooks":["nope"],"files":{"main.go":"x","go.mod":"y"}}`,
			wantErr: true,
		},
		{
			name:    "success - uppercase fence tolerated",
			input:   "```JSON\n" + `{"name":"my-plugin","language":"go","hooks":["before_save"],"files":{"main.go":"package main\nfunc main() {}","go.mod":"module x"}}` + "\n```",
			wantErr: false,
		},
		{
			name:    "success - commentary around envelope tolerated",
			input:   "Here is your plugin:\n" + `{"name":"my-plugin","language":"go","hooks":["before_save"],"files":{"main.go":"package main\nfunc main() {}","go.mod":"module x"}}` + "\nEnjoy!",
			wantErr: false,
		},
		{
			name:    "error - duplicate top-level key",
			input:   `{"name":"my-plugin","language":"go","hooks":["before_save"],"files":{"main.go":"x"},"files":{"main.go":"y"}}`,
			wantErr: true,
		},
		{
			name:    "error - JSON array response",
			input:   `[1,2]`,
			wantErr: true,
		},
		{
			name:    "error - non-JSON response",
			input:   `just some words`,
			wantErr: true,
		},
		{
			name:    "error - truncated value",
			input:   `{"name":"my-plugin","hooks":["before_save"],"files":{"main.go":`,
			wantErr: true,
		},
		{
			name:    "error - non-string key",
			input:   `{"a":1,true}`,
			wantErr: true,
		},
		{
			name:    "success - realistic-size project passes the caps",
			input:   `{"name":"my-plugin","language":"go","hooks":["before_save"],"files":{"main.go":"` + strings.Repeat("a", 64<<10) + `","go.mod":"module example"}}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project, err := plugingen.ParseProjectEnvelope(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "my-plugin", project.Name)
		})
	}
}

func TestParseProjectEnvelopeNormalizesHooks(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantHooks []string
		wantCaps  []string
		wantErr   bool
	}{
		{
			name:      "success - padded and duplicated entries trimmed and deduped",
			input:     `{"name":"my-plugin","language":"go","hooks":[" before_save ","before_save","after_create"],"capabilities":["log_info"," log_info "],"files":{"main.go":"package main\nfunc main() {}","go.mod":"module x"}}`,
			wantHooks: []string{"before_save", "after_create"},
			wantCaps:  []string{"log_info"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project, err := plugingen.ParseProjectEnvelope(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantHooks, project.Hooks)
			assert.Equal(t, tt.wantCaps, project.Capabilities)
		})
	}
}
