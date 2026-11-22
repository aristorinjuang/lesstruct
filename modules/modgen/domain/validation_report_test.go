package modgen_test

import (
	"context"
	"testing"

	modgen "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const reportModuleGo = `package forum

import (
	"database/sql"

	"github.com/aristorinjuang/lesstruct/kernel"
	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
)

const (
	Name = "forum"

	Version = "0.1.0"
)

var _ kernel.Module = (*Module)(nil)

type Option func(*Module)

type Module struct{}

func WithLogger(logger kernel.Logger) Option {
	return func(m *Module) {
	}
}

func WithDB(_ *sql.DB, _ kerneldatabase.Dialect) Option {
	return func(m *Module) {
	}
}

func (m *Module) Name() string {
	return Name
}

func (m *Module) Version() string {
	return Version
}

func (m *Module) Requires() []string {
	return nil
}

func (m *Module) Optional() []string {
	return nil
}

func (m *Module) Init(_ kernel.Kernel) error {
	return nil
}

func (m *Module) Shutdown() error {
	return nil
}

func New(opts ...Option) *Module {
	m := &Module{}
	return m
}
`

const reportGoMod = "module github.com/aristorinjuang/lesstruct/modules/forum\n\nrequire github.com/aristorinjuang/lesstruct/kernel v1.6.0\n"

func validReportProject() modgen.Project {
	return modgen.NewProject(
		"forum",
		[]string{"routes", "migrations"},
		map[string]string{
			"go.mod":                              reportGoMod,
			"module.go":                           reportModuleGo,
			"README.md":                           "forum",
			"domain/types.go":                     "package forum\n",
			"api/handler.go":                      "package api\n\nimport (\n\t\"github.com/aristorinjuang/lesstruct/kernel/httpx\"\n)\n\nfunc x() {\n\t_ = httpx.Success\n}\n",
			"migrations/sqlite/001_init.up.sql":   "-- forum items\nCREATE TABLE IF NOT EXISTS forum_items (id INTEGER PRIMARY KEY);",
			"migrations/sqlite/001_init.down.sql": "DROP TABLE forum_items;",
		},
		"R.",
	)
}

func reportRules(report modgen.ValidationReport) []string {
	rules := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		rules = append(rules, finding.Rule)
	}
	return rules
}

func TestNewValidatorValidate(t *testing.T) {
	tests := []struct {
		name      string
		project   modgen.Project
		validator modgen.ProjectValidator
		wantValid bool
		wantRule  string
		wantErr   bool
	}{
		{
			name:      "success - valid project",
			project:   validReportProject(),
			validator: modgen.NewValidator(),
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "success - valid with kernel pin",
			project:   validReportProject(),
			validator: modgen.NewValidatorWithKernelVersion("v1.6.0"),
			wantValid: true,
			wantErr:   false,
		},
		{
			name: "error - non-migration sql file",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":                      reportGoMod,
					"module.go":                   reportModuleGo,
					"README.md":                   "r",
					"migrations/sqlite/notes.sql": "-- just notes\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-files",
			wantErr:   false,
		},
		{
			name: "error - bad name",
			project: modgen.NewProject(
				"Bad Name",
				nil,
				validReportProject().Files,
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-name",
			wantErr:   false,
		},
		{
			name: "error - bad files",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{"module.go": "package forum"},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-files",
			wantErr:   false,
		},
		{
			name: "error - empty go file",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":          reportGoMod,
					"module.go":       reportModuleGo,
					"README.md":       "r",
					"domain/empty.go": "   ",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-empty",
			wantErr:   false,
		},
		{
			name: "error - go parse failure",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":        reportGoMod,
					"module.go":     reportModuleGo,
					"README.md":     "r",
					"domain/bad.go": "package forum\nfunc (",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-parse",
			wantErr:   false,
		},
		{
			name: "error - wrong package",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":          reportGoMod,
					"module.go":       reportModuleGo,
					"README.md":       "r",
					"domain/wrong.go": "package wrong\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-package",
			wantErr:   false,
		},
		{
			name: "error - internal import",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":         reportGoMod,
					"module.go":      reportModuleGo,
					"README.md":      "r",
					"store/store.go": "package store\n\nimport (\n\t\"github.com/aristorinjuang/lesstruct/internal/app\"\n)\n\nfunc x() {}\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-internal",
			wantErr:   false,
		},
		{
			name: "error - domain net http",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":         reportGoMod,
					"module.go":      reportModuleGo,
					"README.md":      "r",
					"domain/http.go": "package forum\n\nimport (\n\t\"net/http\"\n)\n\nfunc x() {}\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-layer",
			wantErr:   false,
		},
		{
			name: "error - domain database sql",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":        reportGoMod,
					"module.go":     reportModuleGo,
					"README.md":     "r",
					"domain/sql.go": "package forum\n\nimport (\n\t\"database/sql\"\n)\n\nfunc x() {}\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-layer",
			wantErr:   false,
		},
		{
			name: "error - module contract missing",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":    reportGoMod,
					"module.go": "package forum\n",
					"README.md": "r",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-contract",
			wantErr:   false,
		},
		{
			name: "error - api without httpx",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":         reportGoMod,
					"module.go":      reportModuleGo,
					"README.md":      "r",
					"api/handler.go": "package api\n\nfunc x() {}\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-httpx",
			wantErr:   false,
		},
		{
			name: "error - go mod replace",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":    "module x\n\nreplace github.com/aristorinjuang/lesstruct/kernel => ../kernel\n",
					"module.go": reportModuleGo,
					"README.md": "r",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-replace",
			wantErr:   false,
		},
		{
			name: "error - go mod replace block",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":    "module x\n\nreplace (\ngithub.com/aristorinjuang/lesstruct/kernel => ../kernel\n)\n",
					"module.go": reportModuleGo,
					"README.md": "r",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-replace",
			wantErr:   false,
		},
		{
			name: "error - go mod missing kernel",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":    "module x\n",
					"module.go": reportModuleGo,
					"README.md": "r",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-kernel",
			wantErr:   false,
		},
		{
			name: "error - go mod require block kernel",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":    "module x\n\nrequire (\n\tgithub.com/aristorinjuang/lesstruct/kernel v1.6.0\n)\n",
					"module.go": reportModuleGo,
					"README.md": "r",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: true,
			wantErr:   false,
		},
		{
			name: "error - kernel pin mismatch",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":    "module x\n\nrequire github.com/aristorinjuang/lesstruct/kernel v1.5.0\n",
					"module.go": reportModuleGo,
					"README.md": "r",
				},
				"",
			),
			validator: modgen.NewValidatorWithKernelVersion("v1.6.0"),
			wantValid: false,
			wantRule:  "module-go-kernel-pin",
			wantErr:   false,
		},
		{
			name: "error - migration not idempotent",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":                            reportGoMod,
					"module.go":                         reportModuleGo,
					"README.md":                         "r",
					"migrations/sqlite/001_init.up.sql": "CREATE TABLE forum_items (id INTEGER PRIMARY KEY);",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-migration-idempotent",
			wantErr:   false,
		},
		{
			name: "error - migration references",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":                            reportGoMod,
					"module.go":                         reportModuleGo,
					"README.md":                         "r",
					"migrations/sqlite/001_init.up.sql": "CREATE TABLE IF NOT EXISTS forum_items (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id));",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-migration-fk",
			wantErr:   false,
		},
		{
			name: "error - unknown overlay",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":      reportGoMod,
					"module.go":   reportModuleGo,
					"README.md":   "r",
					"custom/x.go": "package custom\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-package",
			wantErr:   false,
		},
		{
			name: "success - domain benign import",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":       reportGoMod,
					"module.go":    reportModuleGo,
					"README.md":    "r",
					"domain/ok.go": "package forum\n\nimport (\n\t\"context\"\n)\n\nfunc x() {}\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: true,
			wantErr:   false,
		},
		{
			name: "success - external test package",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":                 reportGoMod,
					"module.go":              reportModuleGo,
					"README.md":              "r",
					"domain/service.go":      "package forum\n\nfunc x() {}\n",
					"domain/service_test.go": "package forum_test\n\nfunc y() {}\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: true,
			wantErr:   false,
		},
		{
			name: "error - root test file outside the allowlist",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":         reportGoMod,
					"module.go":      reportModuleGo,
					"README.md":      "r",
					"module_test.go": "package forum_test\n\nfunc y() {}\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-files",
			wantErr:   false,
		},
		{
			name: "success - api external test package",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":              reportGoMod,
					"module.go":           reportModuleGo,
					"README.md":           "r",
					"api/handler.go":      "package api\n\nimport (\n\t\"github.com/aristorinjuang/lesstruct/kernel/httpx\"\n)\n\nfunc x() {\n\t_ = httpx.Success\n}\n",
					"api/handler_test.go": "package api_test\n\nfunc y() {}\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: true,
			wantErr:   false,
		},
		{
			name: "error - domain cgo import",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":      reportGoMod,
					"module.go":   reportModuleGo,
					"README.md":   "r",
					"domain/c.go": "package forum\n\nimport (\n\t\"C\"\n)\n\nfunc x() {}\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-cgo",
			wantErr:   false,
		},
		{
			name: "error - api exec import",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":      reportGoMod,
					"module.go":   reportModuleGo,
					"README.md":   "r",
					"api/exec.go": "package api\n\nimport (\n\t\"os/exec\"\n\t\"github.com/aristorinjuang/lesstruct/kernel/httpx\"\n)\n\nfunc x() {\n\t_ = exec.Command\n\t_ = httpx.Success\n}\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-exec",
			wantErr:   false,
		},
		{
			name: "error - store unsafe import",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":          reportGoMod,
					"module.go":       reportModuleGo,
					"README.md":       "r",
					"store/unsafe.go": "package store\n\nimport (\n\t\"unsafe\"\n)\n\nvar _ = unsafe.Pointer(nil)\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-unsafe",
			wantErr:   false,
		},
		{
			name: "error - syscall import",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":       reportGoMod,
					"module.go":    reportModuleGo,
					"README.md":    "r",
					"store/sys.go": "package store\n\nimport (\n\t\"syscall\"\n)\n\nvar _ = syscall.SIGTERM\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-syscall",
			wantErr:   false,
		},
		{
			name: "error - plugin import",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":          reportGoMod,
					"module.go":       reportModuleGo,
					"README.md":       "r",
					"store/plugin.go": "package store\n\nimport (\n\t\"plugin\"\n)\n\nvar _ = plugin.Open\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-plugin",
			wantErr:   false,
		},
		{
			name: "error - net import",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":       reportGoMod,
					"module.go":    reportModuleGo,
					"README.md":    "r",
					"store/net.go": "package store\n\nimport (\n\t\"net/smtp\"\n)\n\nvar _ = smtp.PlainAuth\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-net",
			wantErr:   false,
		},
		{
			name: "error - go mod third party dep",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":    "module x\n\nrequire (\n\tgithub.com/aristorinjuang/lesstruct/kernel v1.6.0\n\tgithub.com/evil/exfil v1.0.0\n)\n",
					"module.go": reportModuleGo,
					"README.md": "r",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-deps",
			wantErr:   false,
		},
		{
			name: "success - go mod testify and chi allowed",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":    "module x\n\nrequire (\n\tgithub.com/aristorinjuang/lesstruct/kernel v1.6.0\n\tgithub.com/go-chi/chi/v5 v5.3.1\n\tgithub.com/stretchr/testify v1.11.1\n\tgithub.com/stretchr/objx v0.5.2\n\tgithub.com/davecgh/go-spew v1.1.2\n\tgithub.com/pmezard/go-difflib v1.0.1\n\tgopkg.in/yaml.v3 v3.0.1\n)\n",
					"module.go": reportModuleGo,
					"README.md": "r",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: true,
			wantErr:   false,
		},
		{
			name: "error - go mod single line third party dep",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":    "module x\n\nrequire github.com/aristorinjuang/lesstruct/kernel v1.6.0\nrequire github.com/evil/exfil v1.0.0\n",
					"module.go": reportModuleGo,
					"README.md": "r",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-go-deps",
			wantErr:   false,
		},
		{
			name: "error - migration drop table",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":                            reportGoMod,
					"module.go":                         reportModuleGo,
					"README.md":                         "r",
					"migrations/sqlite/001_init.up.sql": "CREATE TABLE IF NOT EXISTS forum_items (id INTEGER PRIMARY KEY);\nDROP TABLE users;\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-migration-statement",
			wantErr:   false,
		},
		{
			name: "success - migration alter and index allowed",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":                              reportGoMod,
					"module.go":                           reportModuleGo,
					"README.md":                           "r",
					"migrations/sqlite/001_init.up.sql":   "CREATE TABLE IF NOT EXISTS forum_items (id INTEGER PRIMARY KEY);\nCREATE INDEX IF NOT EXISTS idx_forum ON forum_items (id);\nALTER TABLE forum_items ADD COLUMN title TEXT;\n",
					"migrations/sqlite/001_init.down.sql": "DROP TABLE forum_items;\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: true,
			wantErr:   false,
		},
		{
			name: "error - migration down extra statement",
			project: modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":                              reportGoMod,
					"module.go":                           reportModuleGo,
					"README.md":                           "r",
					"migrations/sqlite/001_init.up.sql":   "CREATE TABLE IF NOT EXISTS forum_items (id INTEGER PRIMARY KEY);",
					"migrations/sqlite/001_init.down.sql": "DROP TABLE forum_items;\nDELETE FROM users;\n",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-migration-statement",
			wantErr:   false,
		},
		{
			name: "error - unknown surface",
			project: modgen.NewProject(
				"forum",
				[]string{"hooks"},
				map[string]string{
					"go.mod":    reportGoMod,
					"module.go": reportModuleGo,
					"README.md": "r",
				},
				"",
			),
			validator: modgen.NewValidator(),
			wantValid: false,
			wantRule:  "module-surfaces",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := tt.validator.Validate(context.Background(), tt.project)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.wantValid, report.Valid)
			if tt.wantRule != "" {
				assert.Contains(t, reportRules(report), tt.wantRule)
			}
		})
	}
}

func TestValidatorCommentAndBlankGoMod(t *testing.T) {
	tests := []struct {
		name      string
		goMod     string
		wantValid bool
		wantErr   bool
	}{
		{
			name:      "success - comments and blanks skipped",
			goMod:     "// a comment\n\nmodule x\n\n// another\nrequire github.com/aristorinjuang/lesstruct/kernel v1.6.0\n",
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "success - require block skipped comments",
			goMod:     "module x\n\nrequire (\n// pinned kernel\n\tgithub.com/aristorinjuang/lesstruct/kernel v1.6.0\n)\n",
			wantValid: true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := modgen.NewProject(
				"forum",
				nil,
				map[string]string{
					"go.mod":    tt.goMod,
					"module.go": reportModuleGo,
					"README.md": "r",
				},
				"",
			)
			report := modgen.NewValidator().Validate(context.Background(), project)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.wantValid, report.Valid)
		})
	}
}
