package plugingen

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

var (
	// ErrInvalidBrief reports an empty or over-long generation brief.
	ErrInvalidBrief = errors.New("invalid brief")
	// ErrInvalidProjectName reports a project name outside the slug grammar.
	ErrInvalidProjectName = errors.New("invalid project name")
	// ErrPathNotAllowed reports a project file path outside the allowlist.
	ErrPathNotAllowed = errors.New("path not allowed")
	// ErrSizeLimitExceeded reports a project file or total exceeding the caps.
	ErrSizeLimitExceeded = errors.New("size limit exceeded")
	// ErrGenerationFailed reports an AI generation or repair failure.
	ErrGenerationFailed = errors.New("generation failed")
	// ErrProjectExists reports an install colliding with an existing project dir.
	ErrProjectExists = errors.New("project exists")
	// ErrProjectNotFound reports a lookup for a missing project.
	ErrProjectNotFound = errors.New("project not found")
	// ErrGenerationUnavailable reports generation without an AI key.
	ErrGenerationUnavailable = errors.New("generation unavailable")
	// ErrInvalidHooks reports an empty or unknown hook list.
	ErrInvalidHooks = errors.New("invalid hooks")
	// ErrInvalidCapabilities reports an unknown capability declaration.
	ErrInvalidCapabilities = errors.New("invalid capabilities")
	// ErrInvalidHTTPPatterns reports a malformed HTTP URL grant.
	ErrInvalidHTTPPatterns = errors.New("invalid http patterns")
	// ErrInvalidDBPermissions reports an unknown database permission.
	ErrInvalidDBPermissions = errors.New("invalid db permissions")
	// ErrBuildUnavailable reports a build without a configured toolchain.
	ErrBuildUnavailable = errors.New("build unavailable")
	// ErrBuildFailed reports a TinyGo compile, load-check, or install failure.
	ErrBuildFailed = errors.New("build failed")
	// ErrPluginInstalled reports a build colliding with an installed plugin.
	ErrPluginInstalled = errors.New("plugin installed")
	// ErrInvalidManifest reports a project that cannot render a manifest.
	ErrInvalidManifest = errors.New("invalid manifest")
)

const (
	// maxProjectNameRunes bounds the slug length (2–50 runes).
	maxProjectNameRunes = 50
	// minProjectNameRunes is the minimum slug length.
	minProjectNameRunes = 2
	// maxFileBytes caps a single project file at 512 KiB.
	maxFileBytes = 512 << 10
	// maxTotalBytes caps a project at 2 MiB total.
	maxTotalBytes = 2 << 20
)

var (
	projectNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,49}$`)
)

func isReservedProjectName(name string) bool {
	switch name {
	case "default", "example", "test", "hello-world":
		return true
	default:
		return false
	}
}

// isAllowedProjectPath reports whether a relative project path is
// installable: the fixed scaffold files only. Dotfiles, absolute paths, and
// parent traversals are rejected.
func isAllowedProjectPath(path string) bool {
	if path == "" {
		return false
	}
	if strings.HasPrefix(path, "/") {
		return false
	}
	if strings.Contains(path, "..") {
		return false
	}
	if strings.Contains(path, "\\") {
		return false
	}
	for segment := range strings.SplitSeq(path, "/") {
		if strings.HasPrefix(segment, ".") {
			return false
		}
		if segment == "" {
			return false
		}
	}
	switch path {
	case "main.go", "go.mod", "README.md", "build.sh", "project.json":
		return true
	default:
		return false
	}
}

// isScannableGo reports whether a project path is Go source subject to the
// import and host-function policy scan.
func isScannableGo(path string) bool {
	return path == "main.go"
}

// ValidateProjectName reports whether name is an installable project slug:
// lowercase alphanumerics and dashes, 2–50 runes, not reserved.
func ValidateProjectName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ErrInvalidProjectName
	}
	runes := len([]rune(trimmed))
	if runes < minProjectNameRunes || runes > maxProjectNameRunes {
		return ErrInvalidProjectName
	}
	if !projectNamePattern.MatchString(trimmed) {
		return ErrInvalidProjectName
	}
	if isReservedProjectName(trimmed) {
		return ErrInvalidProjectName
	}
	return nil
}

// ValidateProjectFiles enforces the v1 project contract: main.go plus go.mod
// required, allowlisted paths only, per-file 512 KiB and 2 MiB total caps.
func ValidateProjectFiles(files map[string]string) error {
	if len(files) == 0 {
		return ErrPathNotAllowed
	}
	var total int
	var hasMain bool
	var hasMod bool
	for path, content := range files {
		if !isAllowedProjectPath(path) {
			return ErrPathNotAllowed
		}
		size := len(content)
		if size > maxFileBytes {
			return ErrSizeLimitExceeded
		}
		total += size
		if total > maxTotalBytes {
			return ErrSizeLimitExceeded
		}
		if path == "main.go" {
			hasMain = true
		}
		if path == "go.mod" {
			hasMod = true
		}
	}
	if !hasMain || !hasMod {
		return ErrPathNotAllowed
	}
	return nil
}

// ValidateBrief enforces the generation brief contract: non-empty and capped
// at 5,000 runes; hooks must be non-empty and known; capabilities must be
// known when present; language is go-only in v1.
func ValidateBrief(brief Brief) error {
	trimmed := strings.TrimSpace(brief.Description)
	if trimmed == "" {
		return ErrInvalidBrief
	}
	if len([]rune(trimmed)) > maxBriefRunes {
		return ErrInvalidBrief
	}
	if language := strings.ToLower(strings.TrimSpace(brief.Language)); language != "" && language != "go" {
		return fmt.Errorf("%w: unsupported language %q", ErrInvalidBrief, brief.Language)
	}
	if len(brief.Hooks) == 0 {
		return ErrInvalidHooks
	}
	for _, hook := range brief.Hooks {
		if !IsKnownHook(strings.TrimSpace(hook)) {
			return ErrInvalidHooks
		}
	}
	for _, capability := range brief.Capabilities {
		if !IsKnownCapability(strings.TrimSpace(capability)) {
			return ErrInvalidCapabilities
		}
	}
	return nil
}

// ScanGeneratedProjectFiles flags content a generated project must not ship:
// linker directives, wasmexport tricks, and javascript: URLs in Go source.
// It runs only on the generation path so repair findings stay deterministic.
func ScanGeneratedProjectFiles(files map[string]string) []Finding {
	findings := make([]Finding, 0, 4)
	for _, path := range slices.Sorted(maps.Keys(files)) {
		if !isScannableGo(path) {
			continue
		}
		content := files[path]
		lowered := strings.ToLower(content)
		if strings.Contains(lowered, "javascript:") {
			findings = append(findings, NewFinding("project-js-url", FindingSeverityError, path+" contains a javascript: URL; generated plugins must not use them"))
		}
		if strings.Contains(content, "go:linkname") {
			findings = append(findings, NewFinding("project-linkname", FindingSeverityError, path+" must not use go:linkname"))
		}
		if strings.Contains(content, "unsafe.") && !strings.Contains(content, "unsafe.Slice") && !strings.Contains(content, "unsafe.Pointer") {
			findings = append(findings, NewFinding("project-unsafe", FindingSeverityWarning, path+" uses unsafe beyond the memory-protocol Slice/Pointer pair"))
		}
	}
	return findings
}
