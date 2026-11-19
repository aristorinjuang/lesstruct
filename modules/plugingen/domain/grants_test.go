package plugingen_test

import (
	"context"
	"testing"

	plugingen "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValidHTTPPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected bool
		wantErr  bool
	}{
		{
			name:     "success - exact https url",
			pattern:  "https://api.example.com/hook",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "success - trailing wildcard",
			pattern:  "https://api.example.com/*",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "error - cleartext http rejected",
			pattern:  "http://localhost:8080/events",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "success - https with port",
			pattern:  "https://localhost:8443/events",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "error - near-wildcard host",
			pattern:  "https://a*",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "error - wildcard not path-scoped",
			pattern:  "https://api.example.com*",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "error - userinfo in host",
			pattern:  "https://user@example.com/events",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "error - empty host",
			pattern:  "https:///events",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "success - surrounding whitespace trimmed",
			pattern:  "  https://api.example.com/*  ",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "error - empty pattern",
			pattern:  "",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "error - missing scheme",
			pattern:  "api.example.com/*",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "error - non-http scheme",
			pattern:  "ftp://files.example.com/*",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "error - interior wildcard",
			pattern:  "https://*.example.com/hook",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "error - multiple wildcards",
			pattern:  "https://api.example.com/*/*",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "error - embedded space",
			pattern:  "https://api.example.com/a b",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "error - quote injection",
			pattern:  `https://api.example.com/"`,
			expected: false,
			wantErr:  false,
		},
		{
			name:     "error - backslash",
			pattern:  `https://api.example.com\hook`,
			expected: false,
			wantErr:  false,
		},
		{
			name:     "error - bare host",
			pattern:  "https://a",
			expected: false,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, plugingen.IsValidHTTPPattern(tt.pattern))
		})
	}
}

func TestKnownDBPermissionsList(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - permissions non-empty",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			permissions := plugingen.KnownDBPermissions()
			assert.NotEmpty(t, permissions)
			assert.Contains(t, permissions, "read:content")
			assert.Equal(t, tt.wantErr, false)
		})
	}
}

func TestParseProjectEnvelopeGrants(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		wantHTTP []string
		wantDB   []string
	}{
		{
			name:     "success - envelope with grants",
			input:    `{"name":"my-plugin","language":"go","hooks":["before_save"],"capabilities":["http_get"],"httpPatterns":["https://api.example.com/*"],"dbPermissions":["read:content"],"rationale":"r","files":{"main.go":"package main\nfunc main() {}","go.mod":"module x"}}`,
			wantErr:  false,
			wantHTTP: []string{"https://api.example.com/*"},
			wantDB:   []string{"read:content"},
		},
		{
			name:     "success - grants trimmed",
			input:    `{"name":"my-plugin","language":"go","hooks":["before_save"],"httpPatterns":["  https://api.example.com/*  "],"dbPermissions":["  read:media  "],"files":{"main.go":"package main\nfunc main() {}","go.mod":"module x"}}`,
			wantErr:  false,
			wantHTTP: []string{"https://api.example.com/*"},
			wantDB:   []string{"read:media"},
		},
		{
			name:    "error - invalid http pattern",
			input:   `{"name":"my-plugin","language":"go","hooks":["before_save"],"httpPatterns":["https://*.example.com/x"],"files":{"main.go":"x","go.mod":"y"}}`,
			wantErr: true,
		},
		{
			name:    "error - unknown db permission",
			input:   `{"name":"my-plugin","language":"go","hooks":["before_save"],"dbPermissions":["read:secrets"],"files":{"main.go":"x","go.mod":"y"}}`,
			wantErr: true,
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
			assert.Equal(t, tt.wantHTTP, project.HTTPPatterns)
			assert.Equal(t, tt.wantDB, project.DBPermissions)
		})
	}
}

func grantProject(httpPatterns, dbPermissions []string, mainGo string) plugingen.Project {
	project := plugingen.NewProject(
		"my-plugin",
		"go",
		[]string{"before_save"},
		nil,
		map[string]string{
			"main.go": mainGo,
			"go.mod":  "module example",
		},
		"",
	)
	project.HTTPPatterns = httpPatterns
	project.DBPermissions = dbPermissions
	return project
}

func reportRules(report plugingen.ValidationReport) []string {
	rules := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		rules = append(rules, finding.Rule)
	}
	return rules
}

func TestServiceGenerateWarnsOnUnrequested(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - unrequested hooks and capabilities warn without failing",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := validProject()
			project.Hooks = append(project.Hooks, "after_create")
			project.Capabilities = append(project.Capabilities, "http_get")
			project.Files["main.go"] = "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\n//export hook_after_create\nfunc hookAfterCreate(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}\n"
			gen := &fakeGeneration{project: project}
			store := newFakeStore()
			svc := plugingen.NewService(&genAdapter{inner: gen}, plugingen.NewValidator(), store, nil)
			brief := plugingen.NewBrief("desc", "go", []string{"before_save"}, []string{"log_info"})
			_, report, err := svc.Generate(context.Background(), brief)
			require.NoError(t, err)
			assert.True(t, report.Valid)
			var warnings []string
			for _, finding := range report.Findings {
				if finding.Severity == plugingen.FindingSeverityWarning {
					warnings = append(warnings, finding.Rule+": "+finding.Message)
				}
			}
			require.Len(t, warnings, 2)
			assert.Contains(t, warnings[0], "after_create")
			assert.Contains(t, warnings[1], "http_get")
			require.Equal(t, false, tt.wantErr)
		})
	}
}

func TestValidatorHookExports(t *testing.T) {
	hookMain := func(extra string) string {
		return "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\n" + extra + "func main() {}\n"
	}
	tests := []struct {
		name     string
		hooks    []string
		mainGo   string
		valid    bool
		wantRule string
		wantErr  bool
	}{
		{
			name:     "success - declared hooks match exports",
			hooks:    []string{"before_save"},
			mainGo:   hookMain(""),
			valid:    true,
			wantRule: "",
			wantErr:  false,
		},
		{
			name:     "success - result-length helper export allowed",
			hooks:    []string{"before_save"},
			mainGo:   hookMain("//export __hook_result_len\nfunc hookResultLen(inputLen uint32) uint32 { return 0 }\n"),
			valid:    true,
			wantRule: "",
			wantErr:  false,
		},
		{
			name:     "error - undeclared extra export",
			hooks:    []string{"before_save"},
			mainGo:   hookMain("//export hook_after_create\nfunc hookAfterCreate(o uint32, l uint32) uint32 { return 0 }\n"),
			valid:    false,
			wantRule: "project-wasmexport",
			wantErr:  false,
		},
		{
			name:     "error - wasmexport directive",
			hooks:    []string{"before_save"},
			mainGo:   hookMain("//go:wasmexport helper\nfunc helper() {}\n"),
			valid:    false,
			wantRule: "project-wasmexport",
			wantErr:  false,
		},
		{
			name:     "error - declared hook without export",
			hooks:    []string{"before_save", "after_create"},
			mainGo:   hookMain(""),
			valid:    false,
			wantRule: "project-hooks",
			wantErr:  false,
		},
		{
			name:     "error - exported hook not declared",
			hooks:    []string{"after_create"},
			mainGo:   hookMain(""),
			valid:    false,
			wantRule: "project-hooks",
			wantErr:  false,
		},
		{
			name:     "duplicate - repeated export tolerated once",
			hooks:    []string{"before_save"},
			mainGo:   "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\n//export hook_before_save\nfunc hookBeforeSaveAgain(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}\n",
			valid:    true,
			wantRule: "",
			wantErr:  false,
		},
		{
			name:     "error - comment mention is not an export",
			hooks:    []string{"before_save"},
			mainGo:   "package main\n// hook_before_save is handled elsewhere\nfunc main() {}\n",
			valid:    false,
			wantRule: "project-hooks",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := plugingen.NewProject(
				"my-plugin",
				"go",
				tt.hooks,
				nil,
				map[string]string{
					"main.go": tt.mainGo,
					"go.mod":  "module example",
				},
				"",
			)
			report := plugingen.NewValidator().Validate(context.Background(), project)
			assert.Equal(t, tt.valid, report.Valid)
			if tt.wantRule != "" {
				assert.Contains(t, reportRules(report), tt.wantRule)
			}
		})
	}
}
func TestValidatorGrants(t *testing.T) {
	hookMain := "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}"
	httpMain := hookMain + "\nvar _ = http_get"
	dbReadMain := hookMain + "\nvar _ = db_query"
	dbWriteMain := hookMain + "\nvar _ = db_exec"

	tests := []struct {
		name     string
		input    plugingen.Project
		valid    bool
		wantRule string
		wantErr  bool
	}{
		{
			name:     "success - http calls with patterns",
			input:    grantProject([]string{"https://api.example.com/*"}, nil, httpMain),
			valid:    true,
			wantRule: "",
			wantErr:  false,
		},
		{
			name:     "error - http calls without patterns",
			input:    grantProject(nil, nil, httpMain),
			valid:    false,
			wantRule: "project-http-pattern",
			wantErr:  false,
		},
		{
			name:     "success - db read with permission",
			input:    grantProject(nil, []string{"read:content"}, dbReadMain),
			valid:    true,
			wantRule: "",
			wantErr:  false,
		},
		{
			name:     "error - db read without permission",
			input:    grantProject(nil, []string{"write:content"}, dbReadMain),
			valid:    false,
			wantRule: "project-db-permission",
			wantErr:  false,
		},
		{
			name:     "success - db write with permission",
			input:    grantProject(nil, []string{"write:media"}, dbWriteMain),
			valid:    true,
			wantRule: "",
			wantErr:  false,
		},
		{
			name:     "error - db write without permission",
			input:    grantProject(nil, nil, dbWriteMain),
			valid:    false,
			wantRule: "project-db-permission",
			wantErr:  false,
		},
		{
			name: "error - invalid http pattern in project",
			input: grantProject(
				[]string{"https://*.example.com/x"},
				nil,
				hookMain,
			),
			valid:    false,
			wantRule: "project-http-pattern",
			wantErr:  false,
		},
		{
			name: "error - unknown db permission in project",
			input: grantProject(
				nil,
				[]string{"read:secrets"},
				hookMain,
			),
			valid:    false,
			wantRule: "project-db-permission",
			wantErr:  false,
		},
		{
			name: "error - missing main skips grant checks",
			input: plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{
					"go.mod": "module example",
				},
				"",
			),
			valid:    false,
			wantRule: "project-files",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := plugingen.NewValidator().Validate(context.Background(), tt.input)
			assert.Equal(t, tt.valid, report.Valid)
			if tt.wantRule != "" {
				assert.Contains(t, reportRules(report), tt.wantRule)
			}
		})
	}
}

func TestValidatorGoMod(t *testing.T) {
	tests := []struct {
		name     string
		goMod    string
		valid    bool
		wantRule string
		wantErr  bool
	}{
		{
			name:     "success - bare module",
			goMod:    "module example\n\ngo 1.23\n",
			valid:    true,
			wantRule: "",
			wantErr:  false,
		},
		{
			name:     "success - comments ignored",
			goMod:    "module example\n\n// a comment about requires\n// require example.com/foo v1.0.0\n",
			valid:    true,
			wantRule: "",
			wantErr:  false,
		},
		{
			name:     "success - commented require ignored",
			goMod:    "module example\n\nrequire // pinned later\n",
			valid:    true,
			wantRule: "",
			wantErr:  false,
		},
		{
			name:     "success - commented replace ignored",
			goMod:    "module example\n\nreplace // pinned later\n",
			valid:    true,
			wantRule: "",
			wantErr:  false,
		},
		{
			name:     "error - tab-separated require",
			goMod:    "module example\n\ngo 1.23\n\nrequire\tgithub.com/evil/mod v1.0.0\n",
			valid:    false,
			wantRule: "go-mod-deps",
			wantErr:  false,
		},
		{
			name:     "error - block opener with trailing comment",
			goMod:    "module example\n\ngo 1.23\n\nrequire ( // pinned\n\texample.com/foo v1.0.0\n)\n",
			valid:    false,
			wantRule: "go-mod-deps",
			wantErr:  false,
		},
		{
			name:     "error - single-line require",
			goMod:    "module example\n\ngo 1.23\n\nrequire example.com/foo v1.0.0\n",
			valid:    false,
			wantRule: "go-mod-deps",
			wantErr:  false,
		},
		{
			name:     "error - require block",
			goMod:    "module example\n\ngo 1.23\n\nrequire (\n\texample.com/foo v1.0.0\n)\n",
			valid:    false,
			wantRule: "go-mod-deps",
			wantErr:  false,
		},
		{
			name:     "error - replace directive",
			goMod:    "module example\n\ngo 1.23\n\nreplace example.com/foo => ../foo\n",
			valid:    false,
			wantRule: "go-mod-replace",
			wantErr:  false,
		},
		{
			name:     "error - replace block",
			goMod:    "module example\n\ngo 1.23\n\nreplace (\n\texample.com/foo => ../foo\n)\n",
			valid:    false,
			wantRule: "go-mod-replace",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{
					"main.go": "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}",
					"go.mod":  tt.goMod,
				},
				"",
			)
			report := plugingen.NewValidator().Validate(context.Background(), project)
			assert.Equal(t, tt.valid, report.Valid)
			if tt.wantRule != "" {
				assert.Contains(t, reportRules(report), tt.wantRule)
			}
		})
	}
}
