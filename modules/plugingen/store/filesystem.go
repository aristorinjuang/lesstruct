// Package store persists generated plugin projects on disk: one directory per
// project under plugins/src, a project.json metadata file, the synthesized
// capability manifest sidecar, and stdlib zip archives for download and
// manual installs.
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

	plugingendomain "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
)

const (
	// manifestFilename is the on-disk metadata file per installed project.
	manifestFilename = "project.json"
	// sourceGenerated marks AI-generated projects.
	sourceGenerated = "generated"
	// generatorName is the project.json generator tag.
	generatorName = "plugingen@0.1.0"
)

// capabilitySidecarName is the synthesized host capability manifest shipped
// inside every project directory: a manual install copies it next to the
// .wasm so the loader can provide the host functions the plugin imports.
func capabilitySidecarName(name string) string {
	return name + ".manifest"
}

// Store is the filesystem ProjectStore: root holds one directory per
// project under plugins/src, and artifactsDir is the host's plugins/
// directory receiving the built .wasm plus the synthesized manifest.
type Store struct {
	root         string
	artifactsDir string
}

func (s *Store) projectDir(name string) string {
	return filepath.Join(s.root, name)
}

func (s *Store) manifestPath(name string) string {
	return filepath.Join(s.projectDir(name), manifestFilename)
}

func (s *Store) readManifest(name string) (plugingendomain.ProjectManifest, bool) {
	data, err := os.ReadFile(s.manifestPath(name))
	if err != nil {
		return plugingendomain.ProjectManifest{}, false
	}
	var manifest plugingendomain.ProjectManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return plugingendomain.ProjectManifest{}, false
	}
	return manifest, true
}

// Install persists a validated project. It validates the name and files again
// (defense in depth), claims the project directory exclusively — a
// concurrent generation resolving the same name gets ErrProjectExists
// instead of interleaving files — enforces zip-slip protection via the
// allowlist, and writes project.json last.
func (s *Store) Install(_ context.Context, project plugingendomain.Project) error {
	if err := plugingendomain.ValidateProjectName(project.Name); err != nil {
		return err
	}
	if err := plugingendomain.ValidateProjectFiles(project.Files); err != nil {
		return err
	}
	dir := s.projectDir(project.Name)
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return fmt.Errorf("failed to create project root: %w", err)
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		if os.IsExist(err) {
			return plugingendomain.ErrProjectExists
		}
		return fmt.Errorf("failed to create project dir: %w", err)
	}
	for path, content := range project.Files {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if !strings.HasPrefix(full, dir+string(os.PathSeparator)) {
			_ = os.RemoveAll(dir)
			return plugingendomain.ErrPathNotAllowed
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			_ = os.RemoveAll(dir)
			return fmt.Errorf("failed to create project subdir: %w", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			_ = os.RemoveAll(dir)
			return fmt.Errorf("failed to write project file: %w", err)
		}
	}
	manifest := plugingendomain.NewProjectManifest(project.Name, project.Language, project.Hooks, project.Capabilities, project.HTTPPatterns, project.DBPermissions, generatorName, time.Now().UTC(), sourceGenerated)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("failed to marshal project manifest: %w", err)
	}
	if err := os.WriteFile(s.manifestPath(project.Name), data, 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("failed to write project manifest: %w", err)
	}
	// Ship the synthesized capability manifest as a sidecar: a manual
	// install (unzip, build, copy) needs this exact file next to the
	// .wasm, or the loader provides no host functions.
	sidecar, err := plugingendomain.SynthesizeManifest(project)
	if err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("failed to synthesize capability manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, capabilitySidecarName(project.Name)), []byte(sidecar), 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("failed to write capability manifest: %w", err)
	}
	return nil
}

// List returns every installed project.
func (s *Store) List(_ context.Context) ([]plugingendomain.InstalledProject, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return []plugingendomain.InstalledProject{}, nil
		}
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	result := make([]plugingendomain.InstalledProject, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if err := plugingendomain.ValidateProjectName(name); err != nil {
			continue
		}
		createdAt := time.Time{}
		if info, statErr := entry.Info(); statErr == nil {
			createdAt = info.ModTime()
		}
		language := "go"
		var hooks []string
		source := sourceGenerated
		if manifest, ok := s.readManifest(name); ok {
			language = manifest.Language
			hooks = manifest.Hooks
			source = manifest.Source
			if !manifest.CreatedAt.IsZero() {
				createdAt = manifest.CreatedAt
			}
		}
		result = append(result, plugingendomain.NewInstalledProject(name, language, hooks, createdAt, source))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// Get reads one installed project back into a Project value.
func (s *Store) Get(_ context.Context, name string) (plugingendomain.Project, error) {
	if err := plugingendomain.ValidateProjectName(name); err != nil {
		return plugingendomain.Project{}, plugingendomain.ErrProjectNotFound
	}
	dir := s.projectDir(name)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return plugingendomain.Project{}, plugingendomain.ErrProjectNotFound
	}
	language := "go"
	var hooks []string
	var capabilities []string
	var httpPatterns []string
	var dbPermissions []string
	if manifest, ok := s.readManifest(name); ok {
		language = manifest.Language
		hooks = manifest.Hooks
		capabilities = manifest.Capabilities
		httpPatterns = manifest.HTTPPatterns
		dbPermissions = manifest.DBPermissions
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
		// The capability sidecar travels with the project but is not a
		// build input: the builder synthesizes it fresh from metadata.
		if rel == capabilitySidecarName(name) {
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
		return plugingendomain.Project{}, fmt.Errorf("failed to read project: %w", err)
	}
	if len(files) == 0 {
		return plugingendomain.Project{}, plugingendomain.ErrProjectNotFound
	}
	project := plugingendomain.NewProject(name, language, hooks, capabilities, files, "")
	project.HTTPPatterns = httpPatterns
	project.DBPermissions = dbPermissions
	return project, nil
}

// Delete removes an installed project and, when the store knows the host's
// plugins directory, the compiled artifacts a previous build installed:
// without the cascade the .wasm would keep executing with its capabilities
// after its source is gone.
func (s *Store) Delete(_ context.Context, name string) error {
	if err := plugingendomain.ValidateProjectName(name); err != nil {
		return plugingendomain.ErrProjectNotFound
	}
	dir := s.projectDir(name)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return plugingendomain.ErrProjectNotFound
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	for _, artifact := range []string{name + ".wasm", name + ".manifest"} {
		if s.artifactsDir == "" || s.artifactsDir == s.root {
			continue
		}
		if err := os.Remove(filepath.Join(s.artifactsDir, artifact)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete installed artifact %s: %w", artifact, err)
		}
	}
	return nil
}

// Zip builds the download archive: the project dir at the archive root plus
// the capability manifest sidecar and a generated README with the build
// install steps. Projects installed before the sidecar shipped synthesize
// it from stored metadata instead.
func (s *Store) Zip(_ context.Context, name string) ([]byte, error) {
	project, err := s.Get(context.Background(), name)
	if err != nil {
		return nil, err
	}
	sidecar, err := os.ReadFile(filepath.Join(s.projectDir(name), capabilitySidecarName(name)))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read capability manifest: %w", err)
		}
		rendered, synthErr := plugingendomain.SynthesizeManifest(project)
		if synthErr != nil {
			return nil, synthErr
		}
		sidecar = []byte(rendered)
	}
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	paths := make([]string, 0, len(project.Files))
	for path := range project.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		entry, err := writer.Create(name + "/" + path)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("failed to create zip entry: %w", err)
		}
		if _, err := entry.Write([]byte(project.Files[path])); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("failed to write zip entry: %w", err)
		}
	}
	readme, err := writer.Create(name + "/INSTALL.md")
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("failed to create zip readme: %w", err)
	}
	_, _ = readme.Write([]byte("Build with TinyGo and install:\n\n1. Unzip this archive.\n2. cd " + name + " && tinygo build -o " + name + ".wasm -target=wasi main.go\n3. cp " + name + ".wasm " + name + ".manifest <lesstruct-project>/plugins/\n4. Restart the server (the DEV_MODE watcher hot-reloads it instead).\n"))
	sidecarEntry, err := writer.Create(name + "/" + name + ".manifest")
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("failed to create zip manifest: %w", err)
	}
	if _, err := sidecarEntry.Write(sidecar); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("failed to write zip manifest: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip: %w", err)
	}
	return buf.Bytes(), nil
}

// NewStore builds the filesystem store rooted at root (plugins/src).
// artifactsDir is the host's plugins/ directory receiving built artifacts;
// Delete cascades into it. An empty artifactsDir disables the cascade
// (tests, minimal embeddings).
func NewStore(root, artifactsDir string) *Store {
	return &Store{
		root:         root,
		artifactsDir: artifactsDir,
	}
}
