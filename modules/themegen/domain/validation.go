package themegen

import (
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"
)

var (
	// ErrInvalidBrief reports an empty or over-long generation brief.
	ErrInvalidBrief = errors.New("invalid brief")
	// ErrInvalidThemeName reports a theme name outside the slug grammar.
	ErrInvalidThemeName = errors.New("invalid theme name")
	// ErrPathNotAllowed reports a theme file path outside the allowlist.
	ErrPathNotAllowed = errors.New("path not allowed")
	// ErrSizeLimitExceeded reports a theme file or total exceeding the caps.
	ErrSizeLimitExceeded = errors.New("size limit exceeded")
	// ErrGenerationFailed reports an AI generation or repair failure.
	ErrGenerationFailed = errors.New("generation failed")
	// ErrThemeExists reports an install colliding with an existing theme dir.
	ErrThemeExists = errors.New("theme exists")
	// ErrThemeNotFound reports a lookup for a missing theme.
	ErrThemeNotFound = errors.New("theme not found")
	// ErrThemeActive reports a delete while the theme is active.
	ErrThemeActive = errors.New("theme active")
	// ErrActivationFailed reports a validation or config-write activation failure.
	ErrActivationFailed = errors.New("activation failed")
	// ErrGenerationUnavailable reports generation without an AI key.
	ErrGenerationUnavailable = errors.New("generation unavailable")
)

const (
	// maxThemeNameRunes bounds the slug length (2–50 runes).
	maxThemeNameRunes = 50
	// minThemeNameRunes is the minimum slug length.
	minThemeNameRunes = 2
	// maxFileBytes caps a single theme file at 512 KiB.
	maxFileBytes = 512 << 10
	// maxTotalBytes caps a theme at 2 MiB total.
	maxTotalBytes = 2 << 20
)

var (
	themeNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,49}$`)

	// eventHandlerPattern spots inline event handlers (onclick=, onerror=,
	// ...) in generated markup. The leading-whitespace anchor keeps
	// attribute values that merely mention handlers out of the match.
	eventHandlerPattern = regexp.MustCompile(`(?i)\son[a-z]+\s*=`)
)

func isReservedThemeName(name string) bool {
	switch name {
	case "default", "theme-default", "theme-storefront":
		return true
	default:
		return false
	}
}

// isAllowedThemePath reports whether a relative theme path is installable:
// static/, templates/, root/ prefixes plus the bare manifest/readme files.
// Dotfiles, absolute paths, and parent traversals are rejected.
func isAllowedThemePath(path string) bool {
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
	if path == "README.md" || path == "LICENSE" || path == "theme.json" {
		return true
	}
	for _, prefix := range []string{"static/", "templates/", "root/"} {
		if strings.HasPrefix(path, prefix) && len(path) > len(prefix) {
			return true
		}
	}
	return false
}

// isScannableMarkup reports whether a theme path can reach a visitor as
// executable markup: page templates, installable same-origin HTML, and
// static SVG (script-capable when embedded via object or embed).
func isScannableMarkup(path string) bool {
	if strings.HasPrefix(path, "templates/") && strings.HasSuffix(path, ".html") {
		return true
	}
	if strings.HasPrefix(path, "static/") && (strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".svg")) {
		return true
	}
	return false
}

// ValidateThemeName reports whether name is an installable theme slug:
// lowercase alphanumerics and dashes, 2–50 runes, not reserved.
func ValidateThemeName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ErrInvalidThemeName
	}
	runes := len([]rune(trimmed))
	if runes < minThemeNameRunes || runes > maxThemeNameRunes {
		return ErrInvalidThemeName
	}
	if !themeNamePattern.MatchString(trimmed) {
		return ErrInvalidThemeName
	}
	if isReservedThemeName(trimmed) {
		return ErrInvalidThemeName
	}
	return nil
}

// ValidateThemeFiles enforces the v1 theme contract: at least one of
// static/base.css or static/style.css, allowlisted paths only, per-file
// 512 KiB and 2 MiB total caps.
func ValidateThemeFiles(files map[string]string) error {
	if len(files) == 0 {
		return ErrPathNotAllowed
	}
	var total int
	var hasCSS bool
	for path, content := range files {
		if !isAllowedThemePath(path) {
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
		if path == "static/base.css" || path == "static/style.css" {
			hasCSS = true
		}
	}
	if !hasCSS {
		return ErrPathNotAllowed
	}
	return nil
}

// ValidateBrief enforces the generation brief contract: non-empty and
// capped at 5,000 runes; style is a free-form preset hint.
func ValidateBrief(brief Brief) error {
	trimmed := strings.TrimSpace(brief.Description)
	if trimmed == "" {
		return ErrInvalidBrief
	}
	if len([]rune(trimmed)) > maxBriefRunes {
		return ErrInvalidBrief
	}
	return nil
}

// ScanGeneratedThemeFiles flags executable content a generated theme must
// not ship: script elements, inline event handlers, and javascript: URLs in
// renderable markup, plus any root/ file (served from the site origin).
// It runs only on the generation path — manual operator themes are trusted
// and pass through boot, reload, and activation with structural validation
// alone — so every finding means the model added something the repair loop
// must remove. Paths iterate sorted so findings (and repair prompts) are
// deterministic.
func ScanGeneratedThemeFiles(files map[string]string) []Finding {
	findings := make([]Finding, 0, 4)
	for _, path := range slices.Sorted(maps.Keys(files)) {
		if strings.HasPrefix(path, "root/") {
			findings = append(findings, NewFinding("theme-root-file", FindingSeverityError, path+" must not live under root/ in a generated theme"))
			continue
		}
		if !isScannableMarkup(path) {
			continue
		}
		lowered := strings.ToLower(files[path])
		if strings.Contains(lowered, "<script") {
			findings = append(findings, NewFinding("template-script", FindingSeverityError, path+" contains a <script> element; generated markup must not add scripts"))
		}
		if eventHandlerPattern.MatchString(files[path]) {
			findings = append(findings, NewFinding("template-handler", FindingSeverityError, path+" contains an inline event handler; generated markup must not add handlers"))
		}
		if strings.Contains(lowered, "javascript:") {
			findings = append(findings, NewFinding("template-js-url", FindingSeverityError, path+" contains a javascript: URL; generated markup must not use them"))
		}
	}
	return findings
}
