package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// defaultDirective holds one CSP directive name and its default sources.
type defaultDirective struct {
	Name    string
	Sources []string
}

// defaultDirectives defines the Lesstruct baseline CSP. The order is stable
// and determines the output order of the policy header.
var defaultDirectives = []defaultDirective{
	{Name: "default-src", Sources: []string{"'self'"}},
	{Name: "script-src", Sources: []string{"'self'", "'unsafe-inline'", "https://cdn.jsdelivr.net"}},
	{Name: "style-src", Sources: []string{"'self'", "'unsafe-inline'", "https://fonts.googleapis.com", "https://cdn.jsdelivr.net"}},
	{Name: "img-src", Sources: []string{"'self'", "data:", "https:"}},
	{Name: "font-src", Sources: []string{"'self'", "https://fonts.gstatic.com", "https://cdn.jsdelivr.net"}},
	{Name: "connect-src", Sources: []string{"'self'"}},
	{Name: "frame-src", Sources: []string{"'self'", "https://www.youtube.com", "https://www.youtube-nocookie.com"}},
	{Name: "frame-ancestors", Sources: []string{"'none'"}},
	{Name: "base-uri", Sources: []string{"'self'"}},
	{Name: "form-action", Sources: []string{"'self'"}},
}

type cspConfigFile struct {
	CSPConfig CSPConfig `toml:"csp"`
}

func dedup(s []string) []string {
	if len(s) < 2 {
		return s
	}
	seen := make(map[string]struct{}, len(s))
	result := make([]string, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}

func validateCSPSource(context, value string) error {
	if strings.Contains(value, ";") || strings.Contains(value, "\n") || strings.Contains(value, "\r") {
		return fmt.Errorf(
			"csp source %q in %s must not contain semicolons or control characters",
			value, context,
		)
	}
	return nil
}

// validateFrameAncestors applies the frame_ancestors-specific rules on top of
// validateCSPSource: an entry must be a single source (embedded whitespace
// would smuggle multiple sources into one entry), and 'none' must be the only
// source when present (browsers discard a frame-ancestors directive that
// mixes 'none' with other sources, which would open framing up entirely).
func validateFrameAncestors(sources []string) error {
	for _, src := range sources {
		if err := validateCSPSource("frame-ancestors", src); err != nil {
			return err
		}
		if strings.TrimSpace(src) != src || strings.ContainsAny(src, " \t") {
			return fmt.Errorf("csp frame_ancestors source %q must not contain whitespace", src)
		}
	}
	if len(sources) > 1 {
		for _, src := range sources {
			if strings.EqualFold(src, "'none'") {
				return fmt.Errorf("csp frame_ancestors: 'none' cannot be combined with other sources")
			}
		}
	}
	return nil
}

// iframeHostFromSource extracts an iframe host allowlist entry from a CSP
// source expression. "https://www.youtube.com" becomes "www.youtube.com",
// "https://*.disqus.com" stays "*.disqus.com" so the sanitizer allows only
// subdomains of disqus.com. Ports and paths are preserved
// ("https://localhost:8080" stays "localhost:8080",
// "https://media.example.com/path/embed" stays "media.example.com/path/embed")
// so the sanitizer restricts matching to them. The result is lowercased.
// Sources without a usable host ("'self'", "'none'", bare schemes, data:,
// blob:, userinfo, IPv6 literals, mid-host or bare wildcards) return "".
func iframeHostFromSource(src string) string {
	s := strings.ToLower(strings.TrimSpace(src))
	if s == "" || strings.HasPrefix(s, "'") {
		return ""
	}
	if after, ok := strings.CutPrefix(s, "https://"); ok {
		s = after
	} else if after, ok := strings.CutPrefix(s, "http://"); ok {
		s = after
	} else {
		// Bare scheme (https:, data:, blob:) or another non-host expression.
		if strings.Contains(s, ":") {
			return ""
		}
	}
	if s == "" || strings.ContainsAny(s, "@[]") {
		return ""
	}
	// A "*" is only meaningful as a leading "*.subdomain" prefix; any other
	// occurrence (https://*, mid-host wildcards) cannot be matched safely.
	if strings.Contains(s, "*") && !strings.HasPrefix(s, "*.") {
		return ""
	}
	return s
}

// extractIframeHosts converts CSP source expressions into iframe host
// allowlist entries, skipping sources that carry no host.
func extractIframeHosts(sources []string) []string {
	hosts := make([]string, 0, len(sources))
	for _, src := range sources {
		if host := iframeHostFromSource(src); host != "" {
			hosts = append(hosts, host)
		}
	}
	return dedup(hosts)
}

// frameSourcesFromPolicy extracts a named directive's sources from a full
// CSP policy string. Returns nil when the policy has no such directive.
func frameSourcesFromPolicy(policy, name string) []string {
	for directive := range strings.SplitSeq(policy, ";") {
		fields := strings.Fields(directive)
		if len(fields) < 2 || !strings.EqualFold(fields[0], name) {
			continue
		}
		return fields[1:]
	}
	return nil
}

// CSPConfig holds the optional [csp] block from config.toml. All fields have
// zero-value defaults that are backward compatible. Operators append sources to
// individual directives; they never replace. The exceptions are `policy` (full
// replacement) and `frame_ancestors` (replaces the `frame-ancestors` directive
// — appending to the default `'none'` is meaningless per the CSP spec, so this
// directive is a dedicated opt-in knob; it also drives the X-Frame-Options
// header, see XFrameOptions).
type CSPConfig struct {
	Disable         bool              `toml:"disable"`
	ReportOnly      bool              `toml:"report_only"`
	ScriptSrc       []string          `toml:"script_src"`
	StyleSrc        []string          `toml:"style_src"`
	ImgSrc          []string          `toml:"img_src"`
	FontSrc         []string          `toml:"font_src"`
	ConnectSrc      []string          `toml:"connect_src"`
	FrameSrc        []string          `toml:"frame_src"`
	MediaSrc        []string          `toml:"media_src"`
	ObjectSrc       []string          `toml:"object_src"`
	WorkerSrc       []string          `toml:"worker_src"`
	FrameAncestors  []string          `toml:"frame_ancestors"`
	ExtraDirectives map[string]string `toml:"extra_directives"`
	Policy          string            `toml:"policy"`
}

// validate checks that all operator-supplied source values are safe:
// no semicolons (would break CSP directive boundaries) and no control
// characters (would allow HTTP header injection).
func (c CSPConfig) validate() error {
	for directive, sources := range c.appendables() {
		for _, src := range sources {
			if err := validateCSPSource(directive, src); err != nil {
				return err
			}
		}
	}
	// frame_ancestors gets stricter checks than the other source lists:
	// embedded whitespace would smuggle multiple sources into one entry, and
	// mixing 'none' with other sources is invalid CSP (browsers discard the
	// whole directive, opening framing up entirely — the opposite of the
	// operator's intent).
	if err := validateFrameAncestors(c.FrameAncestors); err != nil {
		return err
	}
	for dirName, value := range c.ExtraDirectives {
		if err := validateCSPSource(dirName, value); err != nil {
			return err
		}
	}
	if c.Policy != "" {
		if strings.Contains(c.Policy, "\n") || strings.Contains(c.Policy, "\r") {
			return fmt.Errorf("csp.policy must not contain control characters")
		}
	}
	return nil
}

func (c CSPConfig) headerName() string {
	if c.ReportOnly {
		return "Content-Security-Policy-Report-Only"
	}
	return "Content-Security-Policy"
}

func (c CSPConfig) extraForDirective(name string) []string {
	return c.appendables()[name]
}

func (c CSPConfig) appendables() map[string][]string {
	return map[string][]string{
		"script-src":  c.ScriptSrc,
		"style-src":   c.StyleSrc,
		"img-src":     c.ImgSrc,
		"font-src":    c.FontSrc,
		"connect-src": c.ConnectSrc,
		"frame-src":   c.FrameSrc,
		"media-src":   c.MediaSrc,
		"object-src":  c.ObjectSrc,
		"worker-src":  c.WorkerSrc,
	}
}

// Build returns the CSP header name and value for this configuration.
// Empty header value means "do not emit a CSP header".
func (c CSPConfig) Build() (headerName, headerValue string) {
	if c.Disable {
		return "", ""
	}

	if c.Policy != "" {
		return c.headerName(), c.Policy
	}

	var (
		parts   []string
		emitted = make(map[string]struct{})
	)

	for _, dd := range defaultDirectives {
		sources := dd.Sources

		// frame-ancestors is a replace-only knob: the default 'none' cannot be
		// meaningfully appended to, so a configured value fully replaces it.
		if dd.Name == "frame-ancestors" && len(c.FrameAncestors) > 0 {
			sources = c.FrameAncestors
		} else if extra := c.extraForDirective(dd.Name); len(extra) > 0 {
			sources = append(sources, extra...)
		}

		sources = dedup(sources)

		parts = append(parts, dd.Name+" "+strings.Join(sources, " "))
		emitted[dd.Name] = struct{}{}
	}

	// Emit any appendable directive that has operator sources but is NOT
	// in the default set (e.g. media-src, object-src, worker-src).
	for directiveName, sources := range c.appendables() {
		if _, ok := emitted[directiveName]; ok {
			continue
		}
		if len(sources) == 0 {
			continue
		}
		parts = append(parts, directiveName+" "+strings.Join(dedup(sources), " "))
	}

	// Sort extra directive keys so the output order is deterministic.
	if len(c.ExtraDirectives) > 0 {
		keys := make([]string, 0, len(c.ExtraDirectives))
		for k := range c.ExtraDirectives {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, name := range keys {
			value := c.ExtraDirectives[name]
			if value == "" {
				parts = append(parts, name)
			} else {
				parts = append(parts, name+" "+value)
			}
		}
	}

	if len(parts) == 0 {
		return "", ""
	}

	return c.headerName(), strings.Join(parts, "; ")
}

// IFrameHosts derives the HTML sanitizer's iframe host allowlist from the
// frame-src directive: the built-in default sources plus operator appends.
// A full [csp] policy override replaces the defaults: the frame-src sources
// are parsed from that policy and used instead, mirroring the emitted
// header — an override without frame-src leaves the allowlist empty so
// iframes stay stripped. disable and report_only keep the default-based
// allowlist so the sanitizer stays a restrictive safety net even when the
// browser gate is off. Non-host sources ("'self'", bare schemes) are
// skipped; "*.host" entries keep their wildcard prefix so the sanitizer
// allows only subdomains. The result feeds sanitize.SanitizeHTMLDocument so
// embeds the CSP allows to be framed are also allowed through the sanitizer.
func (c CSPConfig) IFrameHosts() []string {
	if c.Policy != "" {
		return extractIframeHosts(frameSourcesFromPolicy(c.Policy, "frame-src"))
	}
	sources := []string{}
	for _, dd := range defaultDirectives {
		if dd.Name == "frame-src" {
			sources = append(sources, dd.Sources...)
			break
		}
	}
	sources = append(sources, c.FrameSrc...)
	return extractIframeHosts(sources)
}

// XFrameOptions derives the X-Frame-Options header value from the framing
// configuration so the legacy header can never contradict the CSP. A policy
// override owns the framing story when set (mirroring Build — the knob only
// applies without a policy). Rules:
//   - a frame-ancestors source list that is exactly "'self'" → "SAMEORIGIN"
//     (duplicates and case variants included; "'self'" is the only keyword
//     X-Frame-Options can express)
//   - any "'none'" source → "DENY" (even in a mixed list, which browsers
//     discard — the legacy header then holds the line)
//   - a host source list → "" (the header is omitted; frame-ancestors is the
//     control). Exception: under report_only the CSP is not enforced, so a
//     host list floors to "DENY" to keep a restriction in place during
//     rollout trials; under disable the operator manages CSP at the edge and
//     the omission stands (documented).
func (c CSPConfig) XFrameOptions() string {
	sources := c.FrameAncestors
	if c.Policy != "" {
		sources = frameSourcesFromPolicy(c.Policy, "frame-ancestors")
	}
	if len(sources) == 0 {
		return "DENY"
	}
	for _, src := range sources {
		if strings.EqualFold(src, "'none'") {
			return "DENY"
		}
	}
	hasSelf := false
	hasHosts := false
	for _, src := range sources {
		if strings.EqualFold(src, "'self'") {
			hasSelf = true
			continue
		}
		hasHosts = true
	}
	if hasSelf && !hasHosts {
		return "SAMEORIGIN"
	}
	if hasHosts && c.ReportOnly {
		return "DENY"
	}
	return ""
}

// LoadCSPConfig reads the optional [csp] block from config.toml. When the
// file, section, or directory is missing it returns a zero-value CSPConfig
// (fully backward compatible — Build() returns the default policy).
func LoadCSPConfig(cfg *Config) (CSPConfig, error) {
	if cfg == nil {
		return CSPConfig{}, fmt.Errorf("config cannot be nil")
	}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return CSPConfig{}, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return CSPConfig{}, nil
		}
		return CSPConfig{}, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return CSPConfig{}, nil
	}

	var ccf cspConfigFile
	if err := toml.Unmarshal(data, &ccf); err != nil {
		return CSPConfig{}, fmt.Errorf("failed to parse CSP config: %w", err)
	}

	if err := ccf.CSPConfig.validate(); err != nil {
		return CSPConfig{}, err
	}

	return ccf.CSPConfig, nil
}
