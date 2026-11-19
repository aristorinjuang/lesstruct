package plugingen

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

// declFuncName extracts a top-level function name from an AST declaration.
func declFuncName(decl ast.Decl) (string, bool) {
	fn, ok := decl.(*ast.FuncDecl)
	if !ok || fn.Recv != nil {
		return "", false
	}
	return fn.Name.Name, true
}

// validateGoMod rejects go.mod dependency directives: generated plugins
// build offline (GOPROXY=off) against the standard library, so any require
// or replace would fail the build. The failure surfaces here instead, where
// the repair loop can remove it. Tokenized on whitespace so tab- or
// space-separated directives and block openers all match.
func validateGoMod(path, content string) []Finding {
	findings := make([]Finding, 0, 2)
	var block string
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
				findings = append(findings, NewFinding("go-mod-replace", FindingSeverityError, path+" must not use replace directives"))
				continue
			}
			findings = append(findings, NewFinding("go-mod-deps", FindingSeverityError, path+" must not require external modules; generated plugins build offline against the standard library"))
			continue
		}
		fields := strings.Fields(trimmed)
		switch fields[0] {
		case "require":
			if len(fields) >= 2 && fields[1] == "(" {
				block = "require"
				continue
			}
			if len(fields) >= 2 && !strings.HasPrefix(fields[1], "//") {
				findings = append(findings, NewFinding("go-mod-deps", FindingSeverityError, path+" must not require external modules; generated plugins build offline against the standard library"))
			}
		case "replace":
			if len(fields) >= 2 && fields[1] == "(" {
				block = "replace"
				continue
			}
			if len(fields) >= 2 && !strings.HasPrefix(fields[1], "//") {
				findings = append(findings, NewFinding("go-mod-replace", FindingSeverityError, path+" must not use replace directives"))
			}
		}
	}
	return findings
}

// checkCapabilityGrants cross-checks main.go's host-function calls against
// the project's declared grants: HTTP calls need at least one URL pattern
// and database calls need the matching table permission, or the host would
// deny every call at runtime.
func checkCapabilityGrants(project Project) []Finding {
	main, ok := project.Files["main.go"]
	if !ok {
		return nil
	}
	findings := make([]Finding, 0, 2)
	callsHTTP := strings.Contains(main, "http_get") || strings.Contains(main, "http_post")
	if callsHTTP && len(project.HTTPPatterns) == 0 {
		findings = append(findings, NewFinding("project-http-pattern", FindingSeverityError, "main.go calls http_get/http_post but declares no httpPatterns; the host would deny every request"))
	}
	hasDBPerm := func(prefix string) bool {
		for _, permission := range project.DBPermissions {
			if strings.HasPrefix(strings.TrimSpace(permission), prefix) {
				return true
			}
		}
		return false
	}
	if strings.Contains(main, "db_query") && !hasDBPerm("read:") {
		findings = append(findings, NewFinding("project-db-permission", FindingSeverityError, "main.go calls db_query but declares no read: database permission; the host would deny every query"))
	}
	if strings.Contains(main, "db_exec") && !hasDBPerm("write:") {
		findings = append(findings, NewFinding("project-db-permission", FindingSeverityError, "main.go calls db_exec but declares no write: database permission; the host would deny every write"))
	}
	return findings
}

// goValidator checks generated Go source: parse shape plus hook-export and
// host-function policy.
type goValidator struct{}

func (goValidator) validateFile(path, content string, declaredHooks []string) []Finding {
	findings := make([]Finding, 0, 2)
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return append(findings, NewFinding("go-empty", FindingSeverityError, path+" is empty"))
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return append(findings, NewFinding("go-parse", FindingSeverityError, path+" does not parse as Go: "+err.Error()))
	}
	hasMain := false
	for _, decl := range file.Decls {
		if fn, ok := declFuncName(decl); ok && fn == "main" {
			hasMain = true
			break
		}
	}
	if path == "main.go" && !hasMain {
		findings = append(findings, NewFinding("go-main", FindingSeverityError, path+" must declare func main for WASI"))
	}
	if path == "main.go" {
		findings = append(findings, checkHookExports(path, file, declaredHooks)...)
		if strings.Contains(content, "os.Exit") || strings.Contains(content, "syscall.") {
			findings = append(findings, NewFinding("go-syscall", FindingSeverityError, path+" must not use process-exit or syscall imports"))
		}
		for _, token := range []string{"http_get", "http_post", "db_query", "db_exec"} {
			if strings.Contains(content, token) && !strings.Contains(content, "lesstruct") {
				findings = append(findings, NewFinding("go-hostfn", FindingSeverityWarning, path+" references "+token+" without the lesstruct host module import block"))
				break
			}
		}
	}
	return findings
}

var (
	exportCommentPattern     = regexp.MustCompile(`^//\s*export\s+(\S+)`)
	wasmexportCommentPattern = regexp.MustCompile(`^//\s*go:wasmexport\s+(\S+)`)
)

// checkHookExports verifies the declared hook list matches the actual
// //export directives: every project hook must be exported, nothing extra
// may be, and no //go:wasmexport entry points may ship. The prompt holds
// the project to exactly the hook protocol; comment mentions of hook_ alone
// do not satisfy it.
func checkHookExports(path string, file *ast.File, declaredHooks []string) []Finding {
	findings := make([]Finding, 0, 2)
	var exported []string
	for _, group := range file.Comments {
		for _, comment := range group.List {
			line := strings.TrimSpace(comment.Text)
			if match := wasmexportCommentPattern.FindStringSubmatch(line); match != nil {
				findings = append(findings, NewFinding("project-wasmexport", FindingSeverityError, path+" must not declare //go:wasmexport "+match[1]+"; only hook_<name> exports are allowed"))
				continue
			}
			if match := exportCommentPattern.FindStringSubmatch(line); match != nil {
				exported = append(exported, match[1])
			}
		}
	}
	declared := make(map[string]struct{}, len(declaredHooks))
	for _, hook := range declaredHooks {
		declared["hook_"+strings.TrimSpace(hook)] = struct{}{}
	}
	allowed := map[string]struct{}{"__hook_result_len": {}}
	for name := range declared {
		allowed[name] = struct{}{}
	}
	for _, name := range exported {
		if _, ok := allowed[name]; !ok {
			findings = append(findings, NewFinding("project-wasmexport", FindingSeverityError, path+" exports unexpected "+name+"; only hook_<name> exports are allowed"))
		}
	}
	seen := make(map[string]struct{}, len(exported))
	for _, name := range exported {
		if name == "__hook_result_len" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		if _, ok := declared[name]; !ok {
			findings = append(findings, NewFinding("project-hooks", FindingSeverityError, fmt.Sprintf("exported %s is not a declared project hook", name)))
		}
	}
	for name := range declared {
		if _, ok := seen[name]; !ok {
			findings = append(findings, NewFinding("project-hooks", FindingSeverityError, fmt.Sprintf("declared hook %q has no matching //export in main.go", strings.TrimPrefix(name, "hook_"))))
		}
	}
	return findings
}

// validator is the default ProjectValidator: file-contract checks plus Go
// shape and host-function policy.
type validator struct {
	goValidator goValidator
}

func (v validator) Validate(_ context.Context, project Project) ValidationReport {
	findings := make([]Finding, 0, 4)
	if err := ValidateProjectName(project.Name); err != nil {
		findings = append(findings, NewFinding("project-name", FindingSeverityError, fmt.Sprintf("invalid project name %q", project.Name)))
	}
	if err := ValidateProjectFiles(project.Files); err != nil {
		findings = append(findings, NewFinding("project-files", FindingSeverityError, err.Error()))
	}
	for path, content := range project.Files {
		if strings.HasSuffix(path, ".go") {
			findings = append(findings, v.goValidator.validateFile(path, content, project.Hooks)...)
		}
	}
	if content, ok := project.Files["go.mod"]; ok {
		findings = append(findings, validateGoMod("go.mod", content)...)
	}
	findings = append(findings, checkCapabilityGrants(project)...)
	if len(project.Hooks) == 0 {
		findings = append(findings, NewFinding("project-hooks", FindingSeverityError, "at least one hook is required"))
	}
	for _, hook := range project.Hooks {
		if !IsKnownHook(hook) {
			findings = append(findings, NewFinding("project-hooks", FindingSeverityError, fmt.Sprintf("unknown hook %q", hook)))
		}
	}
	for _, capability := range project.Capabilities {
		if !IsKnownCapability(capability) {
			findings = append(findings, NewFinding("project-capabilities", FindingSeverityError, fmt.Sprintf("unknown capability %q", capability)))
		}
	}
	for _, pattern := range project.HTTPPatterns {
		if !IsValidHTTPPattern(pattern) {
			findings = append(findings, NewFinding("project-http-pattern", FindingSeverityError, fmt.Sprintf("invalid http pattern %q", pattern)))
		}
	}
	for _, permission := range project.DBPermissions {
		if !IsKnownDBPermission(permission) {
			findings = append(findings, NewFinding("project-db-permission", FindingSeverityError, fmt.Sprintf("unknown db permission %q", permission)))
		}
	}
	return NewFindingsReport(findings)
}

// NewValidator builds the default project validator.
func NewValidator() ProjectValidator {
	return validator{}
}
