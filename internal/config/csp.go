package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// CSPConfig holds the optional [csp] block from config.toml. All fields have
// zero-value defaults that are backward compatible. Operators append sources to
// individual directives; they never replace.
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

		if extra := c.extraForDirective(dd.Name); len(extra) > 0 {
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
		sort.Strings(keys)
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
