// Package store persists generated module projects on disk: one directory per
// project under modules-gen, a project.json metadata file, and stdlib zip
// archives for download. Staged projects are inert source: nothing executes
// until an operator copies the tree into a checkout and rebuilds.
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

	modgendomain "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
)

const (
	// manifestFilename is the on-disk metadata file per installed project.
	manifestFilename = "project.json"
	// sourceGenerated marks AI-generated projects.
	sourceGenerated = "generated"
	// generatorName is the project.json generator tag.
	generatorName = "modgen@0.1.0"
)

// installGuide renders the INSTALL.md shipped in every download: the four
// documented steps plus copy-paste-ready custom-edition wiring. Tier 1
// modules link at compile time, so there is no runtime install — the
// operator rebuilds their own binary.
func installGuide(name, kernelVersion string) string {
	var b strings.Builder
	b.WriteString("# Install " + name + "\n\n")
	b.WriteString("Tier 1 modules compile into the binary: there is no runtime install.\n")
	b.WriteString("Stage this tree into a source checkout, wire it into a custom\n")
	b.WriteString("edition, and rebuild that binary.\n\n")
	b.WriteString("## Steps\n\n")
	b.WriteString("1. Unzip this archive.\n")
	b.WriteString("2. Copy the tree to `modules/" + name + "/` in your Lesstruct checkout.\n")
	b.WriteString("3. Register it:\n")
	b.WriteString("   - `go.work`: add `use ./modules/" + name + "`\n")
	b.WriteString("   - `Makefile` SUBMODULES: append `modules/" + name + "`\n")
	b.WriteString("   - Custom edition package `internal/editions/custom/` that wraps your base edition:\n\n")
	b.WriteString("     ```go\n")
	b.WriteString("     package custom\n\n")
	b.WriteString("     import (\n")
	b.WriteString("         \"github.com/aristorinjuang/lesstruct/internal/app\"\n")
	b.WriteString("         mymod \"" + "github.com/aristorinjuang/lesstruct/modules/" + name + "\"\n")
	b.WriteString("     )\n\n")
	b.WriteString("     func Wrap(base app.Edition) app.Edition {\n")
	b.WriteString("         wire, mount := base.Wire, base.Mount\n")
	b.WriteString("         base.Name = \"custom\"\n")
	b.WriteString("         base.Wire = func(core *app.Core) error {\n")
	b.WriteString("             if wire != nil {\n")
	b.WriteString("                 if err := wire(core); err != nil {\n")
	b.WriteString("                     return err\n")
	b.WriteString("                 }\n")
	b.WriteString("             }\n")
	b.WriteString("             core.Candidates = append(core.Candidates,\n")
	b.WriteString("                 mymod.New(\n")
	b.WriteString("                     mymod.WithDB(core.DB, core.Dialect),\n")
	b.WriteString("                     mymod.WithLogger(modules.NewLogger(core.Logger)),\n")
	b.WriteString("                 ),\n")
	b.WriteString("             )\n")
	b.WriteString("             return nil\n")
	b.WriteString("         }\n")
	b.WriteString("         base.Mount = mount\n")
	b.WriteString("         return base\n")
	b.WriteString("     }\n")
	b.WriteString("     ```\n\n")
	b.WriteString("     Never edit `internal/app/app.go` for a third-party module: edition\n")
	b.WriteString("     packages are the documented wiring point.\n")
	b.WriteString("4. Add a thin main that runs your edition (e.g. `cmd/lesstruct-custom/main.go`):\n\n")
	b.WriteString("     ```go\n")
	b.WriteString("     package main\n\n")
	b.WriteString("     import (\n")
	b.WriteString("         \"github.com/aristorinjuang/lesstruct/internal/app\"\n")
	b.WriteString("         blogedition \"github.com/aristorinjuang/lesstruct/internal/editions/blog\"\n")
	b.WriteString("         customedition \"github.com/aristorinjuang/lesstruct/internal/editions/custom\"\n")
	b.WriteString("     )\n\n")
	b.WriteString("     func main() {\n")
	b.WriteString("         app.Run(customedition.Wrap(blogedition.Edition()))\n")
	b.WriteString("     }\n")
	b.WriteString("     ```\n\n")
	b.WriteString("5. Verify then build the custom binary:\n\n")
	b.WriteString("     ```bash\n")
	b.WriteString("     go test ./modules/" + name + "/...\n")
	b.WriteString("     GOWORK=off go test ./modules/" + name + "/...\n")
	b.WriteString("     go build ./cmd/lesstruct-custom\n")
	b.WriteString("     ```\n\n")
	b.WriteString("Deploy the custom binary: the stock server binary never links your module.\n")
	b.WriteString("Disable anytime with `[modules] disabled = [\"" + name + "\"]`: Requires()\n")
	b.WriteString("prunes it like every other module.\n")
	if strings.TrimSpace(kernelVersion) != "" {
		b.WriteString("\nGenerated against kernel " + kernelVersion + ".\n")
	}
	return b.String()
}

// Store is the filesystem ProjectStore: root holds one directory per project
// under modules-gen. kernelVersion is the host kernel version recorded in
// each manifest and rendered into the download's INSTALL.md.
type Store struct {
	root          string
	kernelVersion string
}

func (s *Store) projectDir(name string) string {
	return filepath.Join(s.root, name)
}

func (s *Store) manifestPath(name string) string {
	return filepath.Join(s.projectDir(name), manifestFilename)
}

func (s *Store) readManifest(name string) (modgendomain.ProjectManifest, bool) {
	data, err := os.ReadFile(s.manifestPath(name))
	if err != nil {
		return modgendomain.ProjectManifest{}, false
	}
	var manifest modgendomain.ProjectManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return modgendomain.ProjectManifest{}, false
	}
	return manifest, true
}

// Install persists a validated project. It validates the name and files again
// (defense in depth), claims the project directory exclusively — a
// concurrent generation resolving the same name gets ErrProjectExists
// instead of interleaving files — enforces zip-slip protection via the
// allowlist, and writes project.json last.
func (s *Store) Install(_ context.Context, project modgendomain.Project) error {
	if err := modgendomain.ValidateProjectName(project.Name); err != nil {
		return err
	}
	if err := modgendomain.ValidateProjectFiles(project.Files); err != nil {
		return err
	}
	dir := s.projectDir(project.Name)
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return fmt.Errorf("failed to create project root: %w", err)
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		if os.IsExist(err) {
			return modgendomain.ErrProjectExists
		}
		return fmt.Errorf("failed to create project dir: %w", err)
	}
	for relPath, content := range project.Files {
		full := filepath.Join(dir, filepath.FromSlash(relPath))
		if !strings.HasPrefix(full, dir+string(os.PathSeparator)) {
			_ = os.RemoveAll(dir)
			return modgendomain.ErrPathNotAllowed
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
	manifest := modgendomain.NewProjectManifest(project.Name, project.Surfaces, s.kernelVersion, generatorName, time.Now().UTC(), sourceGenerated)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("failed to marshal project manifest: %w", err)
	}
	if err := os.WriteFile(s.manifestPath(project.Name), data, 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("failed to write project manifest: %w", err)
	}
	return nil
}

// List returns every installed project.
func (s *Store) List(_ context.Context) ([]modgendomain.InstalledProject, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return []modgendomain.InstalledProject{}, nil
		}
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	result := make([]modgendomain.InstalledProject, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if err := modgendomain.ValidateProjectName(name); err != nil {
			continue
		}
		createdAt := time.Time{}
		if info, statErr := entry.Info(); statErr == nil {
			createdAt = info.ModTime()
		}
		var surfaces []string
		source := sourceGenerated
		if manifest, ok := s.readManifest(name); ok {
			surfaces = manifest.Surfaces
			source = manifest.Source
			if !manifest.CreatedAt.IsZero() {
				createdAt = manifest.CreatedAt
			}
		}
		fileCount := 0
		_ = filepath.WalkDir(s.projectDir(name), func(_ string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return nil
			}
			if entry.Name() == manifestFilename {
				return nil
			}
			fileCount++
			return nil
		})
		result = append(result, modgendomain.NewInstalledProject(name, surfaces, fileCount, createdAt, source, false))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// Get reads one installed project back into a Project value.
func (s *Store) Get(_ context.Context, name string) (modgendomain.Project, error) {
	if err := modgendomain.ValidateProjectName(name); err != nil {
		return modgendomain.Project{}, modgendomain.ErrProjectNotFound
	}
	dir := s.projectDir(name)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return modgendomain.Project{}, modgendomain.ErrProjectNotFound
	}
	var surfaces []string
	if manifest, ok := s.readManifest(name); ok {
		surfaces = manifest.Surfaces
	}
	files := make(map[string]string)
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		// Symlinks are never followed: a planted link would otherwise turn
		// the admin download zip into an arbitrary-file-read primitive.
		if info, err := entry.Info(); err != nil || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %q", modgendomain.ErrPathNotAllowed, filepath.ToSlash(path))
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
		return modgendomain.Project{}, fmt.Errorf("failed to read project: %w", err)
	}
	if len(files) == 0 {
		return modgendomain.Project{}, modgendomain.ErrProjectNotFound
	}
	return modgendomain.NewProject(name, surfaces, files, ""), nil
}

// Delete removes an installed project. Staged projects are inert source, so
// no artifact cascade applies: nothing executes from modules-gen/.
func (s *Store) Delete(_ context.Context, name string) error {
	if err := modgendomain.ValidateProjectName(name); err != nil {
		return modgendomain.ErrProjectNotFound
	}
	dir := s.projectDir(name)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return modgendomain.ErrProjectNotFound
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	return nil
}

// Zip builds the download archive: the project dir at the archive root plus
// the generated INSTALL.md with the compile-time install steps.
func (s *Store) Zip(_ context.Context, name string) ([]byte, error) {
	project, err := s.Get(context.Background(), name)
	if err != nil {
		return nil, err
	}
	kernelVersion := s.kernelVersion
	if manifest, ok := s.readManifest(name); ok && strings.TrimSpace(manifest.KernelVersion) != "" {
		kernelVersion = manifest.KernelVersion
	}
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	paths := make([]string, 0, len(project.Files))
	for relPath := range project.Files {
		paths = append(paths, relPath)
	}
	sort.Strings(paths)
	for _, relPath := range paths {
		entry, err := writer.Create(name + "/" + relPath)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("failed to create zip entry: %w", err)
		}
		if _, err := entry.Write([]byte(project.Files[relPath])); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("failed to write zip entry: %w", err)
		}
	}
	readme, err := writer.Create(name + "/INSTALL.md")
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("failed to create zip readme: %w", err)
	}
	if _, err := readme.Write([]byte(installGuide(name, kernelVersion))); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("failed to write zip readme: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip: %w", err)
	}
	return buf.Bytes(), nil
}

// NewStore builds the filesystem store rooted at root (modules-gen).
// kernelVersion is recorded in each manifest and rendered into INSTALL.md;
// empty disables the pin line.
func NewStore(root, kernelVersion string) *Store {
	return &Store{
		root:          root,
		kernelVersion: kernelVersion,
	}
}
