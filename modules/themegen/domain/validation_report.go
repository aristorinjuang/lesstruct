package themegen

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

const (
	// FindingSeverityError blocks install and activation.
	FindingSeverityError = "error"
	// FindingSeverityWarning surfaces in the UI without blocking.
	FindingSeverityWarning = "warning"
)

var (
	cssURLPattern      = regexp.MustCompile(`url\(\s*['"]?([^'")]+)['"]?\s*\)`)
	externalURLPattern = regexp.MustCompile(`(?i)^(?:https?:)?//`)
	dataURLPattern     = regexp.MustCompile(`(?i)^data:`)
)

// cssValidator checks generated CSS: parse shape after minification and the
// url() policy (relative, data:, and self-hosted font filenames allowed;
// external hosts warn but do not fail — CSP enforces at runtime).
type cssValidator struct{}

func (cssValidator) validateFile(path, content string) []Finding {
	findings := make([]Finding, 0, 2)
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return append(findings, NewFinding("css-empty", FindingSeverityError, path+" is empty"))
	}
	if strings.Count(trimmed, "{") != strings.Count(trimmed, "}") {
		return append(findings, NewFinding("css-parse", FindingSeverityError, path+" has unbalanced braces"))
	}
	for _, match := range cssURLPattern.FindAllStringSubmatch(trimmed, -1) {
		if len(match) < 2 {
			continue
		}
		target := strings.TrimSpace(match[1])
		if target == "" {
			continue
		}
		if dataURLPattern.MatchString(target) {
			continue
		}
		if strings.HasPrefix(target, "/static/") {
			continue
		}
		if externalURLPattern.MatchString(target) {
			findings = append(findings, NewFinding("css-url-external", FindingSeverityWarning, path+" references external URL "+target))
			continue
		}
		if strings.Contains(target, ":") && !strings.HasPrefix(target, "#") {
			findings = append(findings, NewFinding("css-url-external", FindingSeverityWarning, path+" references external URL "+target))
			continue
		}
	}
	return findings
}

// validator is the default ThemeValidator: file-contract checks plus CSS
// shape and url() policy. Template render validation is an edition-side
// decorator composed over this one (only SSR editions carry site-ssr).
type validator struct {
	css cssValidator
}

func (v validator) Validate(_ context.Context, theme Theme) ValidationReport {
	findings := make([]Finding, 0, 4)
	if err := ValidateThemeName(theme.Name); err != nil {
		findings = append(findings, NewFinding("theme-name", FindingSeverityError, fmt.Sprintf("invalid theme name %q", theme.Name)))
	}
	if err := ValidateThemeFiles(theme.Files); err != nil {
		findings = append(findings, NewFinding("theme-files", FindingSeverityError, err.Error()))
	}
	for path, content := range theme.Files {
		if strings.HasSuffix(path, ".css") {
			findings = append(findings, v.css.validateFile(path, content)...)
		}
	}
	return NewFindingsReport(findings)
}

// NewValidator builds the default theme validator.
func NewValidator() ThemeValidator {
	return validator{}
}
