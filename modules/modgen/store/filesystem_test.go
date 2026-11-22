package store_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	modgen "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
	"github.com/aristorinjuang/lesstruct/modules/modgen/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testModuleProject() modgen.Project {
	return modgen.NewProject(
		"forum",
		[]string{"routes"},
		map[string]string{
			"go.mod":          "module x",
			"module.go":       "package forum",
			"README.md":       "forum",
			"domain/types.go": "package forum",
		},
		"R.",
	)
}

func TestStoreInstallListGet(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - install list get",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			s := store.NewStore(root, "v1.6.0")
			require.Equal(t, tt.wantErr, false)
			require.NoError(t, s.Install(context.Background(), testModuleProject()))
			projects, err := s.List(context.Background())
			require.NoError(t, err)
			require.Len(t, projects, 1)
			assert.Equal(t, "forum", projects[0].Name)
			assert.Equal(t, []string{"routes"}, projects[0].Surfaces)
			assert.Equal(t, 4, projects[0].FileCount)
			assert.Equal(t, "generated", projects[0].Source)
			assert.False(t, projects[0].Applied)
			got, err := s.Get(context.Background(), "forum")
			require.NoError(t, err)
			assert.Equal(t, "forum", got.Name)
			assert.Equal(t, []string{"routes"}, got.Surfaces)
			assert.Len(t, got.Files, 4)
		})
	}
}

func TestStoreInstallErrors(t *testing.T) {
	tests := []struct {
		name    string
		project modgen.Project
		wantErr bool
	}{
		{
			name:    "error - invalid name",
			project: modgen.NewProject("Bad Name!", nil, map[string]string{"module.go": "x"}, ""),
			wantErr: true,
		},
		{
			name:    "error - invalid files",
			project: modgen.NewProject("forum", nil, map[string]string{"module.go": "x"}, ""),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewStore(t.TempDir(), "")
			err := s.Install(context.Background(), tt.project)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestStoreInstallCollision(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "error - project exists",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewStore(t.TempDir(), "")
			require.NoError(t, s.Install(context.Background(), testModuleProject()))
			err := s.Install(context.Background(), testModuleProject())
			if tt.wantErr {
				require.ErrorIs(t, err, modgen.ErrProjectExists)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestStoreListEmpty(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - missing root lists empty",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewStore(t.TempDir()+"/nonexistent", "")
			projects, err := s.List(context.Background())
			require.Equal(t, tt.wantErr, false)
			require.NoError(t, err)
			assert.Empty(t, projects)
		})
	}
}

func TestStoreGetDeleteZipNotFound(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "error - get missing",
			input:   "missing",
			wantErr: true,
		},
		{
			name:    "error - invalid name",
			input:   "Bad Name!",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewStore(t.TempDir(), "")
			_, err := s.Get(context.Background(), tt.input)
			require.ErrorIs(t, err, modgen.ErrProjectNotFound)
			require.ErrorIs(t, s.Delete(context.Background(), tt.input), modgen.ErrProjectNotFound)
			_, err = s.Zip(context.Background(), tt.input)
			require.ErrorIs(t, err, modgen.ErrProjectNotFound)
			require.Equal(t, tt.wantErr, true)
		})
	}
}

func TestStoreSymlinkRefused(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "error - symlinked file is not followed",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			s := store.NewStore(root, "v1.6.0")
			require.NoError(t, s.Install(context.Background(), testModuleProject()))
			require.NoError(t, os.WriteFile(filepath.Join(root, "forum", "secret.txt"), []byte("x"), 0o644))
			require.NoError(t, os.Symlink("secret.txt", filepath.Join(root, "forum", "link.go")))
			_, err := s.Get(context.Background(), "forum")
			require.Equal(t, tt.wantErr, true)
			require.ErrorIs(t, err, modgen.ErrPathNotAllowed)
			_, err = s.Zip(context.Background(), "forum")
			require.Error(t, err)
		})
	}
}

func TestStoreDeleteZip(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - delete and zip",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewStore(t.TempDir(), "v1.6.0")
			require.Equal(t, tt.wantErr, false)
			require.NoError(t, s.Install(context.Background(), testModuleProject()))
			data, err := s.Zip(context.Background(), "forum")
			require.NoError(t, err)
			assert.NotEmpty(t, data)
			reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			require.NoError(t, err)
			names := make([]string, 0, len(reader.File))
			var readme []byte
			for _, entry := range reader.File {
				names = append(names, entry.Name)
				assert.Contains(t, entry.Name, "forum/", "every zip entry lives under the project dir")
				if entry.Name == "forum/INSTALL.md" {
					opened, openErr := entry.Open()
					require.NoError(t, openErr)
					readme, err = io.ReadAll(opened)
					require.NoError(t, err)
					require.NoError(t, opened.Close())
				}
			}
			assert.Contains(t, names, "forum/go.mod")
			assert.Contains(t, names, "forum/module.go")
			assert.Contains(t, names, "forum/README.md")
			assert.Contains(t, names, "forum/domain/types.go")
			assert.Contains(t, names, "forum/INSTALL.md")
			assert.Contains(t, string(readme), "modules/forum")
			assert.Contains(t, string(readme), "lesstruct-custom")
			require.NoError(t, s.Delete(context.Background(), "forum"))
			_, err = s.Get(context.Background(), "forum")
			require.ErrorIs(t, err, modgen.ErrProjectNotFound)
		})
	}
}
