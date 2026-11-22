package modgen

import (
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
)

var (
	projectNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]{1,49}$`)
)

func isReservedProjectName(name string) bool {
	switch name {
	case "kernel", "main", "test", "example", "default", "modgen",
		"adminpanel", "auth", "cart", "catalog", "comments", "importers",
		"media", "orders", "payments", "plugingen", "privacy", "seo",
		"sitessr", "speech", "textgen", "themedefault", "themestorefront",
		"themegen", "users", "posttypesbasic":
		return true
	default:
		return false
	}
}

// isReservedEditionName reports whether name collides with the identifiers
// the generated custom edition imports: the host packages it wires against
// plus the conventional edition aliases. A module with one of these names
// would emit an uncompilable custom.go on apply.
func isReservedEditionName(name string) bool {
	switch name {
	case "app", "modules", "custom", "blogedition", "customedition":
		return true
	default:
		return false
	}
}

// isGoKeyword reports whether name is a reserved Go keyword, which can never
// serve as the package clause of a generated module.
func isGoKeyword(name string) bool {
	switch name {
	case "break", "case", "chan", "const", "continue", "default", "defer",
		"else", "fallthrough", "for", "func", "go", "goto", "if", "import",
		"interface", "map", "package", "range", "return", "select", "struct",
		"switch", "type", "var":
		return true
	default:
		return false
	}
}

// isAllowedProjectPath reports whether a relative project path is
// installable: the fixed file grammar only. Dotfiles, absolute paths, and
// parent traversals are rejected.
func isAllowedProjectPath(filepath string) bool {
	if filepath == "" {
		return false
	}
	if strings.HasPrefix(filepath, "/") {
		return false
	}
	if strings.Contains(filepath, "..") {
		return false
	}
	if strings.Contains(filepath, "\\") {
		return false
	}
	for segment := range strings.SplitSeq(filepath, "/") {
		if strings.HasPrefix(segment, ".") {
			return false
		}
		if segment == "" {
			return false
		}
	}
	switch filepath {
	case "go.mod", "module.go", "README.md", "embed.go":
		return true
	default:
		return isAllowedOverlayPath(filepath)
	}
}

// isAllowedOverlayPath reports whether path fits one of the tiered overlays:
// domain/, api/, store/, admin/, testutil/, or versioned migrations.
func isAllowedOverlayPath(filepath string) bool {
	top, rest, ok := strings.Cut(filepath, "/")
	if !ok {
		return false
	}
	switch top {
	case "domain", "api", "store", "admin", "testutil":
		if strings.Contains(rest, "/") {
			return false
		}
		return strings.HasSuffix(rest, ".go") && validGoFilename(rest)
	case "migrations":
		return isAllowedMigrationPath(rest)
	default:
		return false
	}
}

// validGoFilename reports whether name is a plain Go filename without
// subdirectories. Callers only pass .go-suffixed names (the overlay check
// short-circuits on the suffix first), so the suffix is trimmed directly.
func validGoFilename(name string) bool {
	base := strings.TrimSuffix(name, ".go")
	for _, r := range base {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '_' {
			continue
		}
		return false
	}
	return true
}

// isAllowedMigrationPath reports whether rest is a versioned migration file
// under one of the bare driver dirs.
func isAllowedMigrationPath(rest string) bool {
	driver, file, ok := strings.Cut(rest, "/")
	if !ok {
		return false
	}
	switch driver {
	case "sqlite", "postgresql", "mysql":
	default:
		return false
	}
	if strings.Contains(file, "/") {
		return false
	}
	matched, _ := path.Match("[0-9][0-9][0-9]_*.up.sql", file)
	if matched {
		return true
	}
	matched, _ = path.Match("[0-9][0-9][0-9]_*.down.sql", file)
	return matched
}

// isScannableGo reports whether a project path is Go source subject to the
// generated-content policy scan.
func isScannableGo(filepath string) bool {
	return strings.HasSuffix(filepath, ".go")
}

// ValidateProjectName reports whether name is an installable module name:
// lowercase alphanumerics, hyphen-free (so the package clause needs no
// rename), 2–50 runes, not reserved.
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
	if isGoKeyword(trimmed) {
		return ErrInvalidProjectName
	}
	if isReservedEditionName(trimmed) {
		return ErrInvalidProjectName
	}
	if isReservedProjectName(trimmed) {
		return ErrInvalidProjectName
	}
	return nil
}

// ValidateProjectFiles enforces the v1 module contract: go.mod plus module.go
// plus README.md required, allowlisted paths only, per-file 512 KiB, 4 MiB
// total, and at most maxFileCount files.
func ValidateProjectFiles(files map[string]string) error {
	if len(files) == 0 {
		return ErrPathNotAllowed
	}
	if len(files) > maxFileCount {
		return ErrSizeLimitExceeded
	}
	var total int
	var hasMod bool
	var hasModule bool
	var hasReadme bool
	for filepath, content := range files {
		if !isAllowedProjectPath(filepath) {
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
		switch filepath {
		case "go.mod":
			hasMod = true
		case "module.go":
			hasModule = true
		case "README.md":
			hasReadme = true
		}
	}
	if !hasMod || !hasModule || !hasReadme {
		return ErrPathNotAllowed
	}
	return nil
}

// ValidateBrief enforces the generation brief contract: non-empty and capped
// at 5,000 runes; surfaces must be known when present (empty lets the model
// infer them from the description).
func ValidateBrief(brief Brief) error {
	trimmed := strings.TrimSpace(brief.Description)
	if trimmed == "" {
		return ErrInvalidBrief
	}
	if len([]rune(trimmed)) > maxBriefRunes {
		return ErrInvalidBrief
	}
	for _, surface := range brief.Surfaces {
		if !IsKnownSurface(strings.TrimSpace(surface)) {
			return ErrInvalidSurfaces
		}
	}
	return nil
}

// ScanGeneratedProjectFiles flags content a generated module must not ship
// that only string scanning can see: javascript: URLs in Go source. Import
// policy (cgo, os/exec, unsafe, syscall, plugin, net) lives in the AST
// validator, which never false-positives on comments or string literals.
// It runs only on the generation path so repair findings stay
// deterministic.
func ScanGeneratedProjectFiles(files map[string]string) []Finding {
	findings := make([]Finding, 0, 2)
	for _, filepath := range slices.Sorted(maps.Keys(files)) {
		if !isScannableGo(filepath) {
			continue
		}
		content := files[filepath]
		lowered := strings.ToLower(content)
		if strings.Contains(lowered, "javascript:") {
			findings = append(findings, NewFinding("module-js-url", FindingSeverityError, filepath+" contains a javascript: URL; generated modules must not use them"))
		}
		if strings.Contains(content, "go:linkname") {
			findings = append(findings, NewFinding("module-linkname", FindingSeverityError, filepath+" must not use go:linkname"))
		}
	}
	return findings
}
