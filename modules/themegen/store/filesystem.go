// Package store persists generated themes on disk: one directory per theme
// under THEMES_DIR, a theme.json manifest, and stdlib zip archives for
// download and manual THEME_DIR installs.
package store

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
)

const (
	// manifestFilename is the on-disk metadata file per installed theme.
	manifestFilename = "theme.json"
	// sourceGenerated marks AI-generated themes.
	sourceGenerated = "generated"
	// sourceManual marks manually dropped theme dirs without a manifest.
	sourceManual = "manual"
	// generatorName is the theme.json generator tag.
	generatorName = "themegen@0.1.0"
)

// minifyCSS minifies one CSS file; unparseable input returns the error so
// the caller keeps the original for the validator to report.
func minifyCSS(content string) (string, error) {
	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	out, err := m.String("text/css", content)
	if err != nil {
		return "", err
	}
	return out, nil
}

// minifyThemeFiles minifies every .css file in the theme. A minify failure
// keeps the original so validation reports the parse problem, not a
// transport error.
func minifyThemeFiles(files map[string]string) map[string]string {
	out := make(map[string]string, len(files))
	for path, content := range files {
		if strings.HasSuffix(path, ".css") {
			if minified, err := minifyCSS(content); err == nil {
				out[path] = minified
				continue
			}
		}
		out[path] = content
	}
	return out
}

// Store is the filesystem ThemeStore: root is THEMES_DIR, activeName
// resolves the [theme] active value for list markers and delete guards,
// and themeDirOverride resolves the raw THEME_DIR value so the live manual
// override target is listed active and cannot be deleted.
type Store struct {
	root             string
	activeName       func() string
	themeDirOverride func() string
}

func (s *Store) themeDir(name string) string {
	return filepath.Join(s.root, name)
}

func (s *Store) active() string {
	if s.activeName == nil {
		return ""
	}
	return s.activeName()
}

func (s *Store) manifestPath(name string) string {
	return filepath.Join(s.themeDir(name), manifestFilename)
}

// themeDirTarget returns the theme name THEME_DIR resolves to inside the
// store root (exact-directory match only), or "" when unset, outside the
// root, or not a valid theme name.
func (s *Store) themeDirTarget() string {
	if s.themeDirOverride == nil {
		return ""
	}
	raw := strings.TrimSpace(s.themeDirOverride())
	if raw == "" {
		return ""
	}
	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		return ""
	}
	targetAbs, err := filepath.Abs(raw)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return ""
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return ""
	}
	if strings.Contains(rel, string(os.PathSeparator)) {
		return ""
	}
	if err := themegendomain.ValidateThemeName(rel); err != nil {
		return ""
	}
	return rel
}

func (s *Store) readManifest(name string) (themegendomain.ThemeManifest, bool) {
	data, err := os.ReadFile(s.manifestPath(name))
	if err != nil {
		return themegendomain.ThemeManifest{}, false
	}
	var manifest themegendomain.ThemeManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return themegendomain.ThemeManifest{}, false
	}
	return manifest, true
}

// Install persists a validated theme as inactive. It validates the name and
// files again (defense in depth), refuses collisions, minifies the CSS (a
// minify failure keeps the original for the validator to report), enforces
// zip-slip protection via the allowlist, and writes theme.json last.
func (s *Store) Install(_ context.Context, theme themegendomain.Theme) error {
	if err := themegendomain.ValidateThemeName(theme.Name); err != nil {
		return err
	}
	if err := themegendomain.ValidateThemeFiles(theme.Files); err != nil {
		return err
	}
	dir := s.themeDir(theme.Name)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return themegendomain.ErrThemeExists
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create theme dir: %w", err)
	}
	files := minifyThemeFiles(theme.Files)
	for path, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if !strings.HasPrefix(full, dir+string(os.PathSeparator)) {
			_ = os.RemoveAll(dir)
			return themegendomain.ErrPathNotAllowed
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			_ = os.RemoveAll(dir)
			return fmt.Errorf("failed to create theme subdir: %w", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			_ = os.RemoveAll(dir)
			return fmt.Errorf("failed to write theme file: %w", err)
		}
	}
	manifest := themegendomain.NewThemeManifest(theme.Name, string(theme.Edition), generatorName, time.Now().UTC(), sourceGenerated)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("failed to marshal theme manifest: %w", err)
	}
	if err := os.WriteFile(s.manifestPath(theme.Name), data, 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("failed to write theme manifest: %w", err)
	}
	return nil
}

// List returns every installed theme: generated (manifest) plus manually
// dropped dirs (no manifest → source manual, best-effort edition).
func (s *Store) List(_ context.Context) ([]themegendomain.InstalledTheme, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return []themegendomain.InstalledTheme{}, nil
		}
		return nil, fmt.Errorf("failed to list themes: %w", err)
	}
	active := s.active()
	target := s.themeDirTarget()
	result := make([]themegendomain.InstalledTheme, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if err := themegendomain.ValidateThemeName(name); err != nil {
			continue
		}
		isActive := name == active || name == target
		if manifest, ok := s.readManifest(name); ok {
			createdAt := manifest.CreatedAt
			if createdAt.IsZero() {
				if info, statErr := entry.Info(); statErr == nil {
					createdAt = info.ModTime()
				}
			}
			result = append(result, themegendomain.NewInstalledTheme(name, themegendomain.Edition(manifest.Edition), createdAt, isActive, manifest.Source))
			continue
		}
		createdAt := time.Time{}
		if info, statErr := entry.Info(); statErr == nil {
			createdAt = info.ModTime()
		}
		result = append(result, themegendomain.NewInstalledTheme(name, "", createdAt, isActive, sourceManual))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// Get reads one installed theme back into a Theme value.
func (s *Store) Get(_ context.Context, name string) (themegendomain.Theme, error) {
	if err := themegendomain.ValidateThemeName(name); err != nil {
		return themegendomain.Theme{}, themegendomain.ErrThemeNotFound
	}
	dir := s.themeDir(name)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return themegendomain.Theme{}, themegendomain.ErrThemeNotFound
	}
	edition := themegendomain.Edition("")
	rationale := ""
	if manifest, ok := s.readManifest(name); ok {
		edition = themegendomain.Edition(manifest.Edition)
	}
	files := make(map[string]string)
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == manifestFilename {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = string(data)
		return nil
	})
	if err != nil {
		return themegendomain.Theme{}, fmt.Errorf("failed to read theme: %w", err)
	}
	if len(files) == 0 {
		return themegendomain.Theme{}, themegendomain.ErrThemeNotFound
	}
	return themegendomain.NewTheme(name, edition, files, rationale), nil
}

// Delete removes an installed theme. The active theme and the live
// THEME_DIR target cannot be deleted; deactivate or unset first.
func (s *Store) Delete(_ context.Context, name string) error {
	if err := themegendomain.ValidateThemeName(name); err != nil {
		return themegendomain.ErrThemeNotFound
	}
	if name == s.active() || name == s.themeDirTarget() {
		return themegendomain.ErrThemeActive
	}
	dir := s.themeDir(name)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return themegendomain.ErrThemeNotFound
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to delete theme: %w", err)
	}
	return nil
}

// Zip builds the download archive: the theme dir at the archive root plus
// a generated README with the THEME_DIR install steps.
func (s *Store) Zip(_ context.Context, name string) ([]byte, error) {
	theme, err := s.Get(context.Background(), name)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	paths := make([]string, 0, len(theme.Files))
	for path := range theme.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		entry, err := writer.Create(name + "/" + path)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("failed to create zip entry: %w", err)
		}
		if _, err := entry.Write([]byte(theme.Files[path])); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("failed to write zip entry: %w", err)
		}
	}
	readme, err := writer.Create(name + "/README.md")
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("failed to create zip readme: %w", err)
	}
	_, _ = readme.Write([]byte("Install manually with THEME_DIR:\n\n1. Unzip this archive.\n2. Set THEME_DIR to the extracted \"" + name + "\" directory.\n3. Restart the server.\n"))
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip: %w", err)
	}
	return buf.Bytes(), nil
}

// NewStore builds the filesystem store rooted at root (THEMES_DIR).
// activeName resolves the [theme] active value; nil disables active markers.
// themeDirOverride resolves the raw THEME_DIR value; nil disables the manual
// override guard.
func NewStore(root string, activeName func() string, themeDirOverride func() string) *Store {
	return &Store{
		root:             root,
		activeName:       activeName,
		themeDirOverride: themeDirOverride,
	}
}
