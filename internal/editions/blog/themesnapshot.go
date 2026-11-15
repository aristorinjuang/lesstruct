package blog

import (
	"fmt"
	"os"
	"path/filepath"

	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
)

// SnapshotTheme renders the installed theme's homepage to a standalone HTML
// document with populated sample data (the same datasets the render gate
// smoke-renders). The admin panel frames it as a visual snapshot on every
// installed theme card. A missing or broken theme is an error; the handler
// degrades those to a placeholder document so one bad theme cannot break
// the themes listing.
func (s *state) SnapshotTheme(name string) (string, error) {
	if err := themegendomain.ValidateThemeName(name); err != nil {
		return "", err
	}
	if s.cfg == nil {
		return "", fmt.Errorf("theme snapshot unavailable before mount")
	}
	dir := filepath.Join(s.cfg.ThemesRoot(), name)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("theme %q not found", name)
	}
	templates, err := sitessr.NewTemplates(&sitessr.Theme{Dir: dir}, s.snapshotCatalog, s.snapshotSlugs...)
	if err != nil {
		return "", err
	}
	return templates.RenderHomeToString(sampleIndexData())
}
