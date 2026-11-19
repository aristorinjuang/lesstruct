package store_test

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	plugingendomain "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/aristorinjuang/lesstruct/modules/plugingen/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testProject(name string) plugingendomain.Project {
	project := plugingendomain.NewProject(name, "go", []string{"before_save"}, []string{"log_info"}, map[string]string{
		"main.go":      "package main\nfunc main() {}",
		"go.mod":       "module example",
		"README.md":    "readme",
		"build.sh":     "tinygo",
		"project.json": "{}",
	}, "rationale")
	project.HTTPPatterns = []string{"https://api.example.com/*"}
	project.DBPermissions = []string{"read:content"}
	return project
}

func TestStoreRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		project plugingendomain.Project
		wantErr bool
	}{
		{
			name:    "success - install list get zip delete",
			project: testProject("my-plugin"),
			wantErr: false,
		},
		{
			name:    "error - invalid name",
			project: testProject("default"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			s := store.NewStore(root, "")
			ctx := context.Background()
			err := s.Install(ctx, tt.project)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			listed, err := s.List(ctx)
			require.NoError(t, err)
			require.Len(t, listed, 1)
			assert.Equal(t, tt.project.Name, listed[0].Name)
			got, err := s.Get(ctx, tt.project.Name)
			require.NoError(t, err)
			assert.Equal(t, tt.project.Name, got.Name)
			assert.Equal(t, tt.project.HTTPPatterns, got.HTTPPatterns)
			assert.Equal(t, tt.project.DBPermissions, got.DBPermissions)
			assert.NotContains(t, got.Files, tt.project.Name+".manifest")
			sidecar, err := os.ReadFile(filepath.Join(root, tt.project.Name, tt.project.Name+".manifest"))
			require.NoError(t, err)
			expected, err := plugingendomain.SynthesizeManifest(tt.project)
			require.NoError(t, err)
			assert.Equal(t, expected, string(sidecar))
			payload, err := s.Zip(ctx, tt.project.Name)
			require.NoError(t, err)
			assert.NotEmpty(t, payload)
			entries := zipEntryNames(t, payload)
			assert.Contains(t, entries, tt.project.Name+"/main.go")
			assert.Contains(t, entries, tt.project.Name+"/"+tt.project.Name+".manifest")
			assert.Contains(t, entries, tt.project.Name+"/INSTALL.md")
			require.NoError(t, s.Delete(ctx, tt.project.Name))
			_, err = s.Get(ctx, tt.project.Name)
			require.Error(t, err)
		})
	}
}

func TestStoreZipSynthesizesMissingSidecar(t *testing.T) {
	tests := []struct {
		name    string
		project plugingendomain.Project
		wantErr bool
	}{
		{
			name:    "success - zip synthesizes the sidecar for pre-sidecar projects",
			project: testProject("legacy-plugin"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			s := store.NewStore(root, "")
			ctx := context.Background()
			require.NoError(t, s.Install(ctx, tt.project))
			require.NoError(t, os.Remove(filepath.Join(root, tt.project.Name, tt.project.Name+".manifest")))
			payload, err := s.Zip(ctx, tt.project.Name)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			entries := zipEntryNames(t, payload)
			assert.Contains(t, entries, tt.project.Name+"/"+tt.project.Name+".manifest")
		})
	}
}

// zipEntryNames lists the entry names of a zip archive payload.
func zipEntryNames(t *testing.T, payload []byte) []string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	require.NoError(t, err)
	names := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		names = append(names, file.Name)
	}
	return names
}

func TestStoreDeleteCascadesArtifacts(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - delete removes installed wasm and manifest",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			artifacts := t.TempDir()
			s := store.NewStore(root, artifacts)
			ctx := context.Background()
			require.NoError(t, s.Install(ctx, plugingendomain.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{
					"main.go": "package main\nfunc main() {}",
					"go.mod":  "module example",
				},
				"",
			)))
			require.NoError(t, os.WriteFile(filepath.Join(artifacts, "my-plugin.wasm"), []byte("wasm"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(artifacts, "my-plugin.manifest"), []byte("manifest"), 0o644))
			require.NoError(t, s.Delete(ctx, "my-plugin"))
			_, err := os.Stat(filepath.Join(artifacts, "my-plugin.wasm"))
			assert.True(t, os.IsNotExist(err))
			_, err = os.Stat(filepath.Join(artifacts, "my-plugin.manifest"))
			assert.True(t, os.IsNotExist(err))
			_, err = s.Get(ctx, "my-plugin")
			require.Error(t, err)
			require.Equal(t, false, tt.wantErr)
		})
	}
}
