package plugingen_test

import (
	"strings"
	"testing"

	plugingen "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "success - valid name",
			input:   "my-plugin",
			wantErr: false,
		},
		{
			name:    "error - empty name",
			input:   "",
			wantErr: true,
		},
		{
			name:    "error - too short",
			input:   "a",
			wantErr: true,
		},
		{
			name:    "error - reserved",
			input:   "example",
			wantErr: true,
		},
		{
			name:    "error - uppercase",
			input:   "My-Plugin",
			wantErr: true,
		},
		{
			name:    "error - too long",
			input:   strings.Repeat("a", 51),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugingen.ValidateProjectName(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateProjectFiles(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]string
		wantErr bool
	}{
		{
			name: "success - valid files",
			input: map[string]string{
				"main.go":      "package main\nfunc main() {}",
				"go.mod":       "module example",
				"README.md":    "readme",
				"build.sh":     "tinygo",
				"project.json": "{}",
			},
			wantErr: false,
		},
		{
			name:    "error - empty files",
			input:   map[string]string{},
			wantErr: true,
		},
		{
			name: "error - missing main",
			input: map[string]string{
				"go.mod": "module example",
			},
			wantErr: true,
		},
		{
			name: "error - disallowed path",
			input: map[string]string{
				"main.go":   "package main",
				"go.mod":    "module x",
				"evil/../x": "oops",
			},
			wantErr: true,
		},
		{
			name: "error - absolute path",
			input: map[string]string{
				"main.go": "package main",
				"go.mod":  "module x",
				"/abs.go": "oops",
			},
			wantErr: true,
		},
		{
			name: "success - single file exactly at per-file cap",
			input: map[string]string{
				"main.go": strings.Repeat("a", 512<<10),
				"go.mod":  "module example",
			},
			wantErr: false,
		},
		{
			name: "error - single file one byte over per-file cap",
			input: map[string]string{
				"main.go": strings.Repeat("a", (512<<10)+1),
				"go.mod":  "module example",
			},
			wantErr: true,
		},
		{
			name: "success - total exactly at total cap",
			input: map[string]string{
				"main.go":      strings.Repeat("a", 512<<10),
				"go.mod":       strings.Repeat("b", 512<<10),
				"README.md":    strings.Repeat("c", 512<<10),
				"build.sh":     strings.Repeat("d", (512<<10)-2),
				"project.json": "{}",
			},
			wantErr: false,
		},
		{
			name: "error - total one byte over total cap",
			input: map[string]string{
				"main.go":      strings.Repeat("a", 512<<10),
				"go.mod":       strings.Repeat("b", 512<<10),
				"README.md":    strings.Repeat("c", 512<<10),
				"build.sh":     strings.Repeat("d", 512<<10),
				"project.json": "{}",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugingen.ValidateProjectFiles(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateBrief(t *testing.T) {
	tests := []struct {
		name    string
		input   plugingen.Brief
		wantErr bool
	}{
		{
			name:    "success - valid brief",
			input:   plugingen.NewBrief("uppercase titles", "go", []string{"before_save"}, []string{"log_info"}),
			wantErr: false,
		},
		{
			name:    "error - empty description",
			input:   plugingen.NewBrief("", "go", []string{"before_save"}, nil),
			wantErr: true,
		},
		{
			name:    "error - no hooks",
			input:   plugingen.NewBrief("desc", "go", nil, nil),
			wantErr: true,
		},
		{
			name:    "error - unknown hook",
			input:   plugingen.NewBrief("desc", "go", []string{"nope"}, nil),
			wantErr: true,
		},
		{
			name:    "error - unknown capability",
			input:   plugingen.NewBrief("desc", "go", []string{"before_save"}, []string{"exec"}),
			wantErr: true,
		},
		{
			name:    "error - too long",
			input:   plugingen.NewBrief(strings.Repeat("a", 5001), "go", []string{"before_save"}, nil),
			wantErr: true,
		},
		{
			name:    "error - unsupported language",
			input:   plugingen.NewBrief("uppercase titles", "rust", []string{"before_save"}, nil),
			wantErr: true,
		},
		{
			name:    "success - empty language defaults downstream",
			input:   plugingen.NewBrief("uppercase titles", "", []string{"before_save"}, nil),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugingen.ValidateBrief(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestScanGeneratedProjectFiles(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]string
		wantLen int
		wantErr bool
	}{
		{
			name: "success - clean files",
			input: map[string]string{
				"main.go": "package main\nfunc main() {}",
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "success - js url flagged",
			input: map[string]string{
				"main.go": "package main // javascript:foo",
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "success - linkname flagged",
			input: map[string]string{
				"main.go": "package main\n//go:linkname foo bar",
			},
			wantLen: 1,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := plugingen.ScanGeneratedProjectFiles(tt.input)
			assert.Len(t, findings, tt.wantLen)
		})
	}
}
