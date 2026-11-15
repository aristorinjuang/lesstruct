package blog

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/config"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
)

// validationSummary condenses the error-severity findings into a one-line
// reason for boot/reload logs (first three messages).
func validationSummary(report themegendomain.ValidationReport) string {
	messages := make([]string, 0, 3)
	for _, finding := range report.Findings {
		if finding.Severity != themegendomain.FindingSeverityError {
			continue
		}
		messages = append(messages, finding.Message)
		if len(messages) == 3 {
			break
		}
	}
	if len(messages) == 0 {
		return "validation failed"
	}
	return strings.Join(messages, "; ")
}

// resolveActiveTheme implements the theme resolution precedence:
// THEME_DIR wins (legacy manual override), then [theme] active at
// THEMES_DIR/<name>, otherwise the embedded default (nil theme).
// A configured-but-unusable active theme returns an error; callers decide
// what it means: boot logs and falls back to embedded (failed=true), hot
// reload refuses the swap so the previous theme keeps serving.
func resolveActiveTheme(cfg *config.Config, validator themegendomain.ThemeValidator) (*sitessr.Theme, bool, error) {
	if strings.TrimSpace(cfg.ThemeDir) != "" {
		return &sitessr.Theme{Dir: cfg.ThemeDir}, false, nil
	}
	themeCfg, err := config.LoadTheme(cfg)
	if err != nil {
		log.Printf("Warning: Failed to load theme config, using embedded default: %v", err)
		return nil, false, nil
	}
	active := strings.TrimSpace(themeCfg.Active)
	if active == "" {
		return nil, false, nil
	}
	if err := themegendomain.ValidateThemeName(active); err != nil {
		return nil, true, fmt.Errorf("active theme %q has an invalid name", active)
	}
	dir := filepath.Join(cfg.ThemesRoot(), active)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, true, fmt.Errorf("active theme %q not found", active)
	}
	if validator == nil {
		validator = themegendomain.NewValidator()
	}
	theme, err := readInstalledTheme(dir, active)
	if err != nil {
		return nil, true, fmt.Errorf("active theme %q unreadable: %w", active, err)
	}
	report := validator.Validate(context.Background(), theme)
	if !report.Valid {
		return nil, true, fmt.Errorf("active theme %q failed validation: %s", active, validationSummary(report))
	}
	return &sitessr.Theme{Dir: dir}, false, nil
}

// readInstalledTheme reads an installed theme dir back for the startup and
// reload fail-safe check: the stylesheet pair plus every top-level template.
func readInstalledTheme(dir, name string) (themegendomain.Theme, error) {
	files := readThemeStylesheets(dir)
	entries, err := os.ReadDir(filepath.Join(dir, "templates"))
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, "templates", entry.Name()))
			if err != nil {
				continue
			}
			files["templates/"+entry.Name()] = string(data)
		}
	}
	if len(files) == 0 {
		return themegendomain.Theme{}, fmt.Errorf("no theme files")
	}
	return themegendomain.NewTheme(name, themegendomain.EditionBlog, files, ""), nil
}

// readInstalledThemeCSS reads only the stylesheet pair. The preview guard
// uses it: full template validation would re-parse the whole theme on every
// request, while the preview builder already refuses a broken template set.
func readInstalledThemeCSS(dir, name string) (themegendomain.Theme, error) {
	files := readThemeStylesheets(dir)
	if len(files) == 0 {
		return themegendomain.Theme{}, fmt.Errorf("no theme files")
	}
	return themegendomain.NewTheme(name, themegendomain.EditionBlog, files, ""), nil
}

func readThemeStylesheets(dir string) map[string]string {
	files := make(map[string]string)
	for _, candidate := range []string{"static/base.css", "static/style.css"} {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(candidate)))
		if err != nil {
			continue
		}
		files[candidate] = string(data)
	}
	return files
}

// themeActivation adapts config.SaveTheme/LoadTheme to the themegen API
// seam: active state lives in [theme] active, never in a manifest.
type themeActivation struct {
	cfg *config.Config
}

func (a themeActivation) ActiveTheme(_ context.Context) string {
	if a.cfg == nil {
		return ""
	}
	themeCfg, err := config.LoadTheme(a.cfg)
	if err != nil {
		return ""
	}
	return themeCfg.Active
}

func (a themeActivation) SaveActive(_ context.Context, name string) error {
	if a.cfg == nil {
		return fmt.Errorf("%w: missing config", themegendomain.ErrActivationFailed)
	}
	if err := themegendomain.ValidateThemeName(name); err != nil {
		return err
	}
	dir := filepath.Join(a.cfg.ThemesRoot(), name)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return themegendomain.ErrThemeNotFound
	}
	if err := config.SaveTheme(a.cfg, name); err != nil {
		return fmt.Errorf("%w: %v", themegendomain.ErrActivationFailed, err)
	}
	return nil
}

func (a themeActivation) ClearActive(_ context.Context) error {
	if a.cfg == nil {
		return fmt.Errorf("%w: missing config", themegendomain.ErrActivationFailed)
	}
	if err := config.SaveTheme(a.cfg, ""); err != nil {
		return fmt.Errorf("%w: %v", themegendomain.ErrActivationFailed, err)
	}
	return nil
}

// NewThemeActivation builds the config-backed activation store.
func NewThemeActivation(cfg *config.Config) themeActivation {
	return themeActivation{cfg: cfg}
}

// Reload resolves the active theme and swaps the live SSR surfaces to it
// without a restart: templates re-parse, static and root filesystems
// re-resolve, and the textgen brand refreshes. A broken active theme is
// refused before any swap (the previous surfaces keep serving) so the
// caller can revert the activation config write.
func (s *state) Reload(_ context.Context) error {
	if s.templates == nil || s.staticFiles == nil || s.rootHandler == nil {
		return fmt.Errorf("theme reload unavailable before mount")
	}
	theme, _, err := resolveActiveTheme(s.cfg, s.validator)
	if err != nil {
		return err
	}
	if err := s.templates.ReloadFromDisk(theme); err != nil {
		return err
	}
	s.staticFiles.Reload(theme)
	s.rootHandler.Reload(theme)
	baseCSS, styleCSS := sitessr.ReadThemeStyles(theme)
	if s.textgenRefresher != nil {
		s.textgenRefresher(baseCSS, styleCSS)
	}
	return nil
}
