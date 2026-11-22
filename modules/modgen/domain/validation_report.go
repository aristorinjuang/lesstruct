package modgen

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// allowedGoModRequire reports whether a required module path may appear in
// a generated go.mod: the host kernel, the host router (route surfaces
// register chi.Router), and the test dependency family. Anything else is
// an unreviewed supply-chain dependency that would execute during
// verification and link into the operator's binary.
func allowedGoModRequire(path string) bool {
	switch {
	case path == "github.com/aristorinjuang/lesstruct/kernel":
		return true
	case path == "github.com/go-chi/chi/v5":
		return true
	case strings.HasPrefix(path, "github.com/stretchr/testify"):
		return true
	case strings.HasPrefix(path, "github.com/stretchr/objx"):
		return true
	case strings.HasPrefix(path, "github.com/davecgh/go-spew"):
		return true
	case strings.HasPrefix(path, "github.com/pmezard/go-difflib"):
		return true
	case strings.HasPrefix(path, "gopkg.in/yaml.v3"):
		return true
	default:
		return false
	}
}

// validateGoMod rejects replace directives and requires the kernel require.
// Generated modules resolve siblings through go.work, never through replace,
// and always build against the host kernel.
func validateGoMod(filepath, content, kernelVersion string) []Finding {
	findings := make([]Finding, 0, 3)
	var block string
	hasKernel := false
	for line := range strings.Lines(strings.TrimSpace(content)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if block != "" {
			if trimmed == ")" {
				block = ""
				continue
			}
			if block == "replace" {
				findings = append(findings, NewFinding("module-go-replace", FindingSeverityError, filepath+" must not use replace directives"))
				continue
			}
			if strings.Contains(trimmed, "lesstruct/kernel") {
				hasKernel = true
			}
			blockFields := strings.Fields(trimmed)
			if len(blockFields) > 0 && !allowedGoModRequire(blockFields[0]) {
				findings = append(findings, NewFinding("module-go-deps", FindingSeverityError, filepath+" must not require "+strconv.Quote(blockFields[0])+"; generated modules build on the kernel, chi, and testify only"))
			}
			continue
		}
		fields := strings.Fields(trimmed)
		switch fields[0] {
		case "require":
			if len(fields) >= 2 && fields[1] == "(" {
				block = "require"
				continue
			}
			if strings.Contains(trimmed, "lesstruct/kernel") {
				hasKernel = true
			}
			if len(fields) >= 2 && !allowedGoModRequire(fields[1]) {
				findings = append(findings, NewFinding("module-go-deps", FindingSeverityError, filepath+" must not require "+strconv.Quote(fields[1])+"; generated modules build on the kernel, chi, and testify only"))
			}
		case "replace":
			if len(fields) >= 2 && fields[1] == "(" {
				block = "replace"
				continue
			}
			findings = append(findings, NewFinding("module-go-replace", FindingSeverityError, filepath+" must not use replace directives"))
		}
	}
	if !hasKernel {
		findings = append(findings, NewFinding("module-go-kernel", FindingSeverityError, filepath+" must require lesstruct/kernel"))
	} else if strings.TrimSpace(kernelVersion) != "" && !strings.Contains(content, kernelVersion) {
		findings = append(findings, NewFinding("module-go-kernel-pin", FindingSeverityError, filepath+" must require lesstruct/kernel at "+kernelVersion))
	}
	return findings
}

// blockedHostImport reports whether an import path is forbidden everywhere
// in generated modules: cgo, process execution, unsafe, raw syscalls, the
// plugin loader, and the network (except net/http for handler constants).
// These are execution and exfiltration vectors that no generated module
// needs; the AST check replaces substring scanning so comments and string
// literals never trip it.
func blockedHostImport(importPath string) (string, bool) {
	switch importPath {
	case "C":
		return "module-go-cgo", true
	case "os/exec":
		return "module-go-exec", true
	case "unsafe":
		return "module-go-unsafe", true
	case "syscall":
		return "module-go-syscall", true
	case "plugin":
		return "module-go-plugin", true
	}
	if importPath == "net" || strings.HasPrefix(importPath, "net/") {
		if importPath != "net/http" {
			return "module-go-net", true
		}
	}
	return "", false
}

// expectedPackage derives the package clause a project path must declare:
// the module name at the root, the directory name inside overlays, and the
// conventional external test package (projectName_test) for _test.go files.
func expectedPackage(projectName, filepath string) string {
	trimmed := strings.TrimSuffix(filepath, "_test.go")
	isTest := trimmed != filepath
	top, rest, ok := strings.Cut(trimmed, "/")
	if !ok {
		if isTest {
			return projectName + "_test"
		}
		return projectName
	}
	switch top {
	case "domain":
		_ = rest
		if isTest {
			return projectName + "_test"
		}
		return projectName
	case "api", "store", "admin", "testutil":
		if isTest {
			return top + "_test"
		}
		return top
	default:
		return projectName
	}
}

// forbiddenDomainImport reports whether an import path is forbidden inside
// domain/: transport, SQL, and the composition root never cross the port.
func forbiddenDomainImport(importPath string) bool {
	switch importPath {
	case "net/http", "database/sql":
		return true
	default:
		return strings.HasPrefix(importPath, "github.com/aristorinjuang/lesstruct/internal")
	}
}

// importPath unquotes an AST import path. Parse guarantees a quoted value,
// so the error skip only guards hand-built trees.
func importPath(spec *ast.ImportSpec) string {
	unquoted, _ := strconv.Unquote(spec.Path.Value)
	return unquoted
}

// goValidator checks generated Go source: parse shape, package clauses, and
// the layer import policy.
type goValidator struct{}

func (goValidator) validateFile(filepath, content, projectName string) []Finding {
	findings := make([]Finding, 0, 4)
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return append(findings, NewFinding("module-go-empty", FindingSeverityError, filepath+" is empty"))
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath, content, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return append(findings, NewFinding("module-go-parse", FindingSeverityError, filepath+" does not parse as Go: "+err.Error()))
	}
	if file.Name == nil || file.Name.Name != expectedPackage(projectName, filepath) {
		want := expectedPackage(projectName, filepath)
		got := ""
		if file.Name != nil {
			got = file.Name.Name
		}
		findings = append(findings, NewFinding("module-go-package", FindingSeverityError, filepath+" declares package "+strconv.Quote(got)+"; want "+strconv.Quote(want)))
	}
	top, _, _ := strings.Cut(filepath, "/")
	for _, imp := range file.Imports {
		imported := importPath(imp)
		if strings.HasPrefix(imported, "github.com/aristorinjuang/lesstruct/internal") {
			findings = append(findings, NewFinding("module-go-internal", FindingSeverityError, filepath+" must not import lesstruct/internal ("+imported+")"))
			continue
		}
		if rule, blocked := blockedHostImport(imported); blocked {
			findings = append(findings, NewFinding(rule, FindingSeverityError, filepath+" must not import "+imported+"; generated modules never shell out, link C, or touch the host"))
			continue
		}
		if top == "domain" && forbiddenDomainImport(imported) {
			findings = append(findings, NewFinding("module-go-layer", FindingSeverityError, filepath+" must not import "+imported+" inside domain/"))
		}
	}
	if filepath == "module.go" {
		findings = append(findings, checkModuleContract(filepath, content)...)
	}
	return findings
}

// checkModuleContract verifies the kernel.Module lifecycle shape with string
// checks the repair loop can act on: the kernel witness, the constructor, and
// the lifecycle methods.
func checkModuleContract(filepath, content string) []Finding {
	findings := make([]Finding, 0, 4)
	if !strings.Contains(content, "kernel.Module") {
		findings = append(findings, NewFinding("module-contract", FindingSeverityError, filepath+" must reference kernel.Module (lifecycle witness or interface)"))
	}
	if !strings.Contains(content, "func New(") {
		findings = append(findings, NewFinding("module-contract", FindingSeverityError, filepath+" must define func New (constructor builds handlers once)"))
	}
	for _, method := range []string{"func (m *Module)", "Requires()", "Init(", "Shutdown("} {
		if !strings.Contains(content, method) {
			findings = append(findings, NewFinding("module-contract", FindingSeverityError, filepath+" must implement the kernel.Module lifecycle ("+method+" missing)"))
		}
	}
	// Adoption options let a custom edition wire collaborators without
	// editing the module: the logger is universal, the database covers the
	// store overlay. The apply seam falls back to logger-only for older
	// projects, but newly generated modules always expose both.
	for _, option := range []string{"WithLogger(", "WithDB("} {
		if !strings.Contains(content, option) {
			findings = append(findings, NewFinding("module-contract", FindingSeverityError, filepath+" must expose a "+option[:len(option)-1]+" adoption option (nil-safe without collaborators)"))
		}
	}
	if !strings.Contains(content, `"0.1.0"`) {
		findings = append(findings, NewFinding("module-contract", FindingSeverityError, filepath+" must start Version at \"0.1.0\""))
	}
	return findings
}

// firstStatementWord extracts the leading keywords of a SQL statement for
// policy matching: up to two upper-cased words (e.g. "CREATE TABLE").
// Callers skip blank statements, so fields is never empty here.
func firstStatementWord(statement string) string {
	fields := strings.Fields(strings.ToUpper(strings.TrimSpace(statement)))
	head := fields[0]
	if len(fields) > 1 && (head == "CREATE" || head == "ALTER" || head == "DROP") {
		head += " " + fields[1]
	}
	return head
}

// stripSQLComments removes -- line comments from SQL so policy checks see
// statements, not commentary.
func stripSQLComments(content string) string {
	var b strings.Builder
	for line := range strings.Lines(content) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// checkMigrationHygiene enforces idempotent, FK-free, allowlisted migrations.
// Up migrations may only create tables/indexes idempotently or extend
// tables; down migrations may only drop what the up migration created.
// Anything else — data writes, drops of foreign tables, pragmas — fails,
// because migrations execute as the app's DB user on first boot.
func checkMigrationHygiene(filepath, content string) []Finding {
	findings := make([]Finding, 0, 3)
	upper := strings.ToUpper(content)
	if strings.Contains(upper, "REFERENCES") {
		findings = append(findings, NewFinding("module-migration-fk", FindingSeverityError, filepath+" must not REFERENCES another table; index the reference column instead of foreign-keying across modules"))
	}
	if strings.HasSuffix(filepath, ".down.sql") {
		for statement := range strings.SplitSeq(stripSQLComments(content), ";") {
			trimmed := strings.TrimSpace(statement)
			if trimmed == "" {
				continue
			}
			head := firstStatementWord(trimmed)
			if head != "DROP TABLE" && head != "DROP INDEX" {
				findings = append(findings, NewFinding("module-migration-statement", FindingSeverityError, filepath+" down migrations may only DROP TABLE or DROP INDEX, found "+strconv.Quote(head)))
				break
			}
		}
		return findings
	}
	if !strings.HasSuffix(filepath, ".up.sql") {
		return findings
	}
	for statement := range strings.SplitSeq(stripSQLComments(content), ";") {
		trimmed := strings.TrimSpace(statement)
		if trimmed == "" {
			continue
		}
		head := firstStatementWord(trimmed)
		switch head {
		case "CREATE TABLE", "CREATE INDEX":
			if !strings.Contains(strings.ToUpper(trimmed), "IF NOT EXISTS") {
				findings = append(findings, NewFinding("module-migration-idempotent", FindingSeverityError, filepath+" must use CREATE TABLE/INDEX IF NOT EXISTS so pre-module installs upgrade cleanly"))
			}
		case "ALTER TABLE":
		default:
			findings = append(findings, NewFinding("module-migration-statement", FindingSeverityError, filepath+" up migrations may only CREATE TABLE/INDEX IF NOT EXISTS or ALTER TABLE, found "+strconv.Quote(head)))
		}
	}
	return findings
}

// validator is the default ProjectValidator: file-contract checks plus Go
// shape, module-contract, layer policy, and migration hygiene.
type validator struct {
	goValidator   goValidator
	kernelVersion string
}

func (v validator) Validate(_ context.Context, project Project) ValidationReport {
	findings := make([]Finding, 0, 8)
	if err := ValidateProjectName(project.Name); err != nil {
		findings = append(findings, NewFinding("module-name", FindingSeverityError, fmt.Sprintf("invalid module name %q", project.Name)))
	}
	if err := ValidateProjectFiles(project.Files); err != nil {
		findings = append(findings, NewFinding("module-files", FindingSeverityError, err.Error()))
	}
	hasAPI := false
	apiUsesHttpx := false
	for filepath, content := range project.Files {
		if strings.HasSuffix(filepath, ".go") {
			findings = append(findings, v.goValidator.validateFile(filepath, content, project.Name)...)
		}
		if strings.HasPrefix(filepath, "migrations/") && strings.HasSuffix(filepath, ".sql") {
			findings = append(findings, checkMigrationHygiene(filepath, content)...)
		}
		if strings.HasPrefix(filepath, "api/") && strings.HasSuffix(filepath, ".go") {
			hasAPI = true
			if strings.Contains(content, "httpx") {
				apiUsesHttpx = true
			}
		}
	}
	if hasAPI && !apiUsesHttpx {
		findings = append(findings, NewFinding("module-httpx", FindingSeverityError, "api/ handlers must write through kernel/httpx"))
	}
	if content, ok := project.Files["go.mod"]; ok {
		findings = append(findings, validateGoMod("go.mod", content, v.kernelVersion)...)
	}
	for _, surface := range project.Surfaces {
		if !IsKnownSurface(surface) {
			findings = append(findings, NewFinding("module-surfaces", FindingSeverityError, fmt.Sprintf("unknown surface %q", surface)))
		}
	}
	return NewFindingsReport(findings)
}

// NewValidator builds the default project validator without a kernel pin:
// the kernel require must exist, but its version is unchecked.
func NewValidator() ProjectValidator {
	return validator{}
}

// NewValidatorWithKernelVersion builds the default project validator pinned
// to the host kernel version the generated go.mod must require.
func NewValidatorWithKernelVersion(kernelVersion string) ProjectValidator {
	return validator{kernelVersion: strings.TrimSpace(kernelVersion)}
}
