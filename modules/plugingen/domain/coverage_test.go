package plugingen_test

import (
	"context"
	"strings"
	"testing"

	plugingen "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withGrants attaches capability grants to a project built by a factory.
// NewProject covers the core fields; the envelope parser fills these, and
// tests set them directly.
func withGrants(project plugingen.Project, httpPatterns, dbPermissions []string) plugingen.Project {
	project.HTTPPatterns = httpPatterns
	project.DBPermissions = dbPermissions
	return project
}

func TestParseEnvelopeEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "error - path with spaces",
			input:   `{"name":"my-plugin","language":"go","hooks":["before_save"],"files":{" main.go":"x","go.mod":"y"}}`,
			wantErr: true,
		},
		{
			name:    "error - empty path",
			input:   `{"name":"my-plugin","language":"go","hooks":["before_save"],"files":{"":"x","go.mod":"y"}}`,
			wantErr: true,
		},
		{
			name:    "error - unknown capability",
			input:   `{"name":"my-plugin","language":"go","hooks":["before_save"],"capabilities":["exec"],"files":{"main.go":"x","go.mod":"y"}}`,
			wantErr: true,
		},
		{
			name:    "error - files fail contract",
			input:   `{"name":"my-plugin","language":"go","hooks":["before_save"],"files":{"main.go":"x"}}`,
			wantErr: true,
		},
		{
			name:    "error - no hooks",
			input:   `{"name":"my-plugin","language":"go","hooks":[],"files":{"main.go":"x","go.mod":"y"}}`,
			wantErr: true,
		},
		{
			name:    "success - empty language defaults",
			input:   `{"name":"my-plugin","hooks":["before_save"],"files":{"main.go":"package main\nfunc main() {} // hook","go.mod":"module x"}}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := plugingen.ParseProjectEnvelope(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestIsAllowedPathEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]string
		wantErr bool
	}{
		{
			name: "error - backslash path",
			input: map[string]string{
				"main.go": "x",
				"go.mod":  "y",
				`a\b`:     "z",
			},
			wantErr: true,
		},
		{
			name: "error - parent traversal",
			input: map[string]string{
				"main.go": "x",
				"go.mod":  "y",
				"a/../b":  "z",
			},
			wantErr: true,
		},
		{
			name: "error - leading slash",
			input: map[string]string{
				"main.go":  "x",
				"go.mod":   "y",
				"/main.go": "z",
			},
			wantErr: true,
		},
		{
			name: "error - empty path",
			input: map[string]string{
				"main.go": "x",
				"go.mod":  "y",
				"":        "z",
			},
			wantErr: true,
		},
		{
			name: "error - per file too large",
			input: map[string]string{
				"main.go": strings.Repeat("a", (512<<10)+1),
				"go.mod":  "y",
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

func TestScanUnsafeWarning(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]string
		wantErr bool
	}{
		{
			name: "success - unsafe other warns",
			input: map[string]string{
				"main.go": "package main\nvar _ = unsafe.Sizeof(0)",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := plugingen.ScanGeneratedProjectFiles(tt.input)
			assert.NotNil(t, findings)
		})
	}
}

func TestValidatorMethodRecvAndTypeDecl(t *testing.T) {
	tests := []struct {
		name    string
		input   plugingen.Project
		valid   bool
		wantErr bool
	}{
		{
			name: "error - method with receiver ignored, missing main",
			input: plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{
					"main.go": "package main\ntype T struct{}\nfunc (t T) Foo() {}\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }",
					"go.mod":  "module x",
				},
				"",
			),
			valid:   false,
			wantErr: false,
		},
		{
			name: "success - hostfn warning path",
			input: withGrants(plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				[]string{"http_get"},
				map[string]string{
					"main.go": "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}\nvar _ = http_get",
					"go.mod":  "module x",
				},
				"",
			), []string{"https://api.example.com/*"}, nil),
			valid:   true,
			wantErr: false,
		},
		{
			name: "error - empty hooks",
			input: plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{},
				nil,
				map[string]string{
					"main.go": "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}",
					"go.mod":  "module x",
				},
				"",
			),
			valid:   false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := plugingen.NewValidator().Validate(context.Background(), tt.input)
			assert.Equal(t, tt.valid, report.Valid)
		})
	}
}

// stubValidator returns canned reports to exercise errorProblems branches.
type stubValidator struct {
	reports []plugingen.ValidationReport
	calls   int
}

func (s *stubValidator) Validate(_ context.Context, _ plugingen.Project) plugingen.ValidationReport {
	if s.calls < len(s.reports) {
		report := s.reports[s.calls]
		s.calls++
		return report
	}
	s.calls++
	return plugingen.NewFindingsReport(nil)
}

func TestGenerateRepairBranches(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "error - repair fails",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First report: error with empty rule + warning; repair returns bad again; second fails.
			first := plugingen.NewValidationReport(false, []plugingen.Finding{
				plugingen.NewFinding("", "error", "bare problem"),
				plugingen.NewFinding("w", "warning", "guidance"),
			})
			second := plugingen.NewValidationReport(false, []plugingen.Finding{
				plugingen.NewFinding("go-hook", "error", "still bad"),
			})
			validator := &stubValidator{reports: []plugingen.ValidationReport{first, second, second}}
			gen := &fakeGeneration{project: validProject(), repair: validProject()}
			// Force repair to fail by setting repairErr after first repair? Instead make repair project invalid so loop exhausts.
			gen.repair = plugingen.NewProject(
				"bad-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{"main.go": "package main\nfunc main() {}", "go.mod": "module x"},
				"",
			)
			store := newFakeStore()
			svc := plugingen.NewService(&genAdapter{inner: gen}, validator, store, nil)
			brief := plugingen.NewBrief("desc", "go", []string{"before_save"}, nil)
			_, _, err := svc.Generate(context.Background(), brief)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGenerateStoreAndRepairErrors(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() (*fakeGeneration, *fakeStore, plugingen.ProjectValidator)
		wantErr bool
	}{
		{
			name: "error - store install fails",
			setup: func() (*fakeGeneration, *fakeStore, plugingen.ProjectValidator) {
				store := newFakeStore()
				store.installErr = assert.AnError
				return &fakeGeneration{project: validProject()}, store, plugingen.NewValidator()
			},
			wantErr: true,
		},
		{
			name: "error - repair call fails",
			setup: func() (*fakeGeneration, *fakeStore, plugingen.ProjectValidator) {
				bad := plugingen.NewProject(
					"bad-plugin",
					"go",
					[]string{"before_save"},
					nil,
					map[string]string{"main.go": "package main\nfunc main() {}", "go.mod": "module x"},
					"",
				)
				gen := &fakeGeneration{project: bad}
				gen.repairErr = assert.AnError
				return gen, newFakeStore(), plugingen.NewValidator()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen, store, validator := tt.setup()
			svc := plugingen.NewService(&genAdapter{inner: gen}, validator, store, nil)
			brief := plugingen.NewBrief("desc", "go", []string{"before_save"}, nil)
			_, _, err := svc.Generate(context.Background(), brief)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGenerateFallbacks(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - empty language and hooks fall back",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proj := validProject()
			proj.Language = ""
			proj.Hooks = nil
			gen := &fakeGeneration{project: proj}
			store := newFakeStore()
			svc := plugingen.NewService(&genAdapter{inner: gen}, plugingen.NewValidator(), store, nil)
			brief := plugingen.NewBrief("desc", "go", []string{"before_save"}, nil)
			got, report, err := svc.Generate(context.Background(), brief)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.True(t, report.Valid)
			assert.Equal(t, "go", got.Language)
			assert.Equal(t, []string{"before_save"}, got.Hooks)
		})
	}
}
