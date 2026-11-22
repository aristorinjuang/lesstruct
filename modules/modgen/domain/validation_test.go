package modgen_test

import (
	"fmt"
	"strings"
	"testing"

	modgen "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
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
			input:   "forum",
			wantErr: false,
		},
		{
			name:    "success - trims whitespace",
			input:   "  forum  ",
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
			name:    "error - reserved kernel",
			input:   "kernel",
			wantErr: true,
		},
		{
			name:    "error - reserved first party",
			input:   "themegen",
			wantErr: true,
		},
		{
			name:    "error - uppercase",
			input:   "Forum",
			wantErr: true,
		},
		{
			name:    "error - hyphen",
			input:   "my-mod",
			wantErr: true,
		},
		{
			name:    "error - go keyword",
			input:   "func",
			wantErr: true,
		},
		{
			name:    "error - reserved edition name",
			input:   "app",
			wantErr: true,
		},
		{
			name:    "error - reserved modules name",
			input:   "modules",
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
			err := modgen.ValidateProjectName(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func validModuleFiles() map[string]string {
	return map[string]string{
		"go.mod":    "module github.com/aristorinjuang/lesstruct/modules/forum\n\nrequire github.com/aristorinjuang/lesstruct/kernel v1.6.0\n",
		"module.go": "package forum",
		"README.md": "forum",
	}
}

func TestValidateProjectFiles(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]string
		wantErr bool
	}{
		{
			name:    "success - minimal files",
			input:   validModuleFiles(),
			wantErr: false,
		},
		{
			name: "success - full overlays",
			input: map[string]string{
				"go.mod":                              "module x",
				"module.go":                           "package forum",
				"README.md":                           "r",
				"embed.go":                            "package forum",
				"domain/types.go":                     "package forum",
				"domain/repository.go":                "package forum",
				"domain/service.go":                   "package forum",
				"domain/v2_store.go":                  "package forum",
				"api/handler.go":                      "package api",
				"store/store.go":                      "package store",
				"admin/manifest.go":                   "package admin",
				"testutil/testutil.go":                "package testutil",
				"migrations/sqlite/001_init.up.sql":   "CREATE TABLE t (id INTEGER);",
				"migrations/sqlite/001_init.down.sql": "DROP TABLE t;",
			},
			wantErr: false,
		},
		{
			name:    "error - empty files",
			input:   map[string]string{},
			wantErr: true,
		},
		{
			name: "error - empty path",
			input: map[string]string{
				"":          "x",
				"go.mod":    "module x",
				"module.go": "package forum",
				"README.md": "r",
			},
			wantErr: true,
		},
		{
			name: "error - bare migrations dir",
			input: map[string]string{
				"go.mod":            "module x",
				"module.go":         "package forum",
				"README.md":         "r",
				"migrations/sqlite": "x",
			},
			wantErr: true,
		},
		{
			name: "error - missing readme",
			input: map[string]string{
				"go.mod":    "module x",
				"module.go": "package forum",
			},
			wantErr: true,
		},
		{
			name: "error - missing module",
			input: map[string]string{
				"go.mod":    "module x",
				"README.md": "r",
			},
			wantErr: true,
		},
		{
			name: "error - dotfile",
			input: map[string]string{
				"go.mod":    "module x",
				"module.go": "package forum",
				"README.md": "r",
				".hidden":   "x",
			},
			wantErr: true,
		},
		{
			name: "error - absolute path",
			input: map[string]string{
				"go.mod":    "module x",
				"module.go": "package forum",
				"README.md": "r",
				"/abs.go":   "x",
			},
			wantErr: true,
		},
		{
			name: "error - traversal",
			input: map[string]string{
				"go.mod":    "module x",
				"module.go": "package forum",
				"README.md": "r",
				"../up.go":  "x",
			},
			wantErr: true,
		},
		{
			name: "error - backslash",
			input: map[string]string{
				"go.mod":      "module x",
				"module.go":   "package forum",
				"README.md":   "r",
				`domain\x.go`: "x",
			},
			wantErr: true,
		},
		{
			name: "error - empty segment",
			input: map[string]string{
				"go.mod":       "module x",
				"module.go":    "package forum",
				"README.md":    "r",
				"domain//x.go": "x",
			},
			wantErr: true,
		},
		{
			name: "error - unknown root file",
			input: map[string]string{
				"go.mod":    "module x",
				"module.go": "package forum",
				"README.md": "r",
				"build.sh":  "x",
			},
			wantErr: true,
		},
		{
			name: "error - nested overlay",
			input: map[string]string{
				"go.mod":          "module x",
				"module.go":       "package forum",
				"README.md":       "r",
				"api/agent/x.go":  "x",
				"domain/types.go": "package forum",
			},
			wantErr: true,
		},
		{
			name: "error - bad go filename",
			input: map[string]string{
				"go.mod":        "module x",
				"module.go":     "package forum",
				"README.md":     "r",
				"domain/Bad.go": "package forum",
			},
			wantErr: true,
		},
		{
			name: "error - non go overlay",
			input: map[string]string{
				"go.mod":           "module x",
				"module.go":        "package forum",
				"README.md":        "r",
				"domain/notes.txt": "x",
			},
			wantErr: true,
		},
		{
			name: "error - unknown overlay",
			input: map[string]string{
				"go.mod":      "module x",
				"module.go":   "package forum",
				"README.md":   "r",
				"custom/x.go": "package custom",
			},
			wantErr: true,
		},
		{
			name: "error - bad migration driver",
			input: map[string]string{
				"go.mod":                            "module x",
				"module.go":                         "package forum",
				"README.md":                         "r",
				"migrations/oracle/001_init.up.sql": "x",
			},
			wantErr: true,
		},
		{
			name: "error - bad migration name",
			input: map[string]string{
				"go.mod":                         "module x",
				"module.go":                      "package forum",
				"README.md":                      "r",
				"migrations/sqlite/001_init.sql": "x",
			},
			wantErr: true,
		},
		{
			name: "error - nested migration",
			input: map[string]string{
				"go.mod":                                "module x",
				"module.go":                             "package forum",
				"README.md":                             "r",
				"migrations/sqlite/sub/001_init.up.sql": "x",
			},
			wantErr: true,
		},
		{
			name: "error - single file too large",
			input: map[string]string{
				"go.mod":    "module x",
				"module.go": "package forum",
				"README.md": strings.Repeat("r", 513<<10),
			},
			wantErr: true,
		},
		{
			name:    "error - too many files",
			input:   manyFiles(65),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := modgen.ValidateProjectFiles(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func manyFiles(count int) map[string]string {
	files := validModuleFiles()
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("migrations/sqlite/%03d_init.up.sql", i%1000)
		files[name] = "CREATE TABLE IF NOT EXISTS t (id INTEGER);"
	}
	return files
}

func TestValidateProjectFilesTotalCap(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]string
		wantErr bool
	}{
		{
			name: "error - total exceeds cap",
			input: map[string]string{
				"go.mod":               strings.Repeat("m", 500<<10),
				"module.go":            strings.Repeat("m", 500<<10),
				"README.md":            strings.Repeat("r", 500<<10),
				"domain/types.go":      strings.Repeat("t", 500<<10),
				"domain/repository.go": strings.Repeat("t", 500<<10),
				"domain/service.go":    strings.Repeat("t", 500<<10),
				"api/handler.go":       strings.Repeat("h", 500<<10),
				"store/store.go":       strings.Repeat("s", 500<<10),
				"admin/manifest.go":    strings.Repeat("a", 500<<10),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := modgen.ValidateProjectFiles(tt.input)
			if tt.wantErr {
				require.ErrorIs(t, err, modgen.ErrSizeLimitExceeded)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateBrief(t *testing.T) {
	tests := []struct {
		name    string
		input   modgen.Brief
		wantErr bool
	}{
		{
			name:    "success - with surfaces",
			input:   modgen.NewBrief("A forum module", []string{"routes", "migrations"}),
			wantErr: false,
		},
		{
			name:    "success - no surfaces infers",
			input:   modgen.NewBrief("Something", nil),
			wantErr: false,
		},
		{
			name:    "error - empty",
			input:   modgen.NewBrief("", nil),
			wantErr: true,
		},
		{
			name:    "error - blank",
			input:   modgen.NewBrief("   ", nil),
			wantErr: true,
		},
		{
			name:    "error - too long",
			input:   modgen.NewBrief(strings.Repeat("a", 5001), nil),
			wantErr: true,
		},
		{
			name:    "error - unknown surface",
			input:   modgen.NewBrief("Something", []string{"hooks"}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := modgen.ValidateBrief(tt.input)
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
		name          string
		input         map[string]string
		wantErrCount  int
		wantWarnCount int
		wantErr       bool
	}{
		{
			name: "success - clean",
			input: map[string]string{
				"module.go": "package forum",
				"notes.txt": "javascript: ignored, not Go",
			},
			wantErr: false,
		},
		{
			name: "error - javascript url",
			input: map[string]string{
				"module.go": "package forum // see javascript:void(0)",
			},
			wantErrCount: 1,
			wantErr:      false,
		},
		{
			name: "error - linkname",
			input: map[string]string{
				"module.go": "package forum\n//go:linkname foo bar",
			},
			wantErrCount: 1,
			wantErr:      false,
		},
		{
			name: "success - imports are the AST validator's job",
			input: map[string]string{
				"module.go": "package forum\nimport \"os/exec\"\n",
			},
			wantErrCount: 0,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := modgen.ScanGeneratedProjectFiles(tt.input)
			require.Equal(t, tt.wantErr, false)
			errs := 0
			warns := 0
			for _, finding := range findings {
				if finding.Severity == modgen.FindingSeverityError {
					errs++
				} else {
					warns++
				}
				assert.NotEmpty(t, finding.Rule)
				assert.NotEmpty(t, finding.Message)
			}
			assert.Equal(t, tt.wantErrCount, errs)
			assert.Equal(t, tt.wantWarnCount, warns)
		})
	}
}
