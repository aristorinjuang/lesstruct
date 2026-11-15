package store_test

import (
	"context"
	"testing"

	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
	"github.com/aristorinjuang/lesstruct/modules/themegen/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testTheme(name string) themegendomain.Theme {
	return themegendomain.NewTheme(name, themegendomain.EditionBlog, map[string]string{
		"static/base.css":  ":root{--color-bg:#fff}",
		"static/style.css": ".btn{color:red}",
	}, "rationale")
}

func TestStoreRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		theme   themegendomain.Theme
		wantErr bool
	}{
		{
			name:    "success - install list get zip delete",
			theme:   testTheme("midnight-serif"),
			wantErr: false,
		},
		{
			name:    "error - invalid name",
			theme:   testTheme("default"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			s := store.NewStore(root, nil, nil)
			ctx := context.Background()
			err := s.Install(ctx, tt.theme)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			listed, err := s.List(ctx)
			require.NoError(t, err)
			require.Len(t, listed, 1)
			assert.Equal(t, tt.theme.Name, listed[0].Name)
			got, err := s.Get(ctx, tt.theme.Name)
			require.NoError(t, err)
			assert.Equal(t, tt.theme.Name, got.Name)
			payload, err := s.Zip(ctx, tt.theme.Name)
			require.NoError(t, err)
			assert.NotEmpty(t, payload)
			require.NoError(t, s.Delete(ctx, tt.theme.Name))
			_, err = s.Get(ctx, tt.theme.Name)
			require.Error(t, err)
		})
	}
}

func TestStoreActiveGuard(t *testing.T) {
	tests := []struct {
		name    string
		active  string
		target  string
		wantErr bool
	}{
		{
			name:    "success - delete inactive",
			active:  "other-theme",
			target:  "midnight-serif",
			wantErr: false,
		},
		{
			name:    "error - delete active",
			active:  "midnight-serif",
			target:  "midnight-serif",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			s := store.NewStore(root, func() string { return tt.active }, nil)
			ctx := context.Background()
			require.NoError(t, s.Install(ctx, testTheme(tt.target)))
			err := s.Delete(ctx, tt.target)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestStoreThemeDirGuard(t *testing.T) {
	tests := []struct {
		name     string
		themeDir func(root string) string
		target   string
		wantErr  bool
	}{
		{
			name:     "error - delete live theme dir target",
			themeDir: func(root string) string { return root + "/midnight-serif" },
			target:   "midnight-serif",
			wantErr:  true,
		},
		{
			name:     "success - delete when theme dir unset",
			themeDir: func(root string) string { return "" },
			target:   "midnight-serif",
			wantErr:  false,
		},
		{
			name:     "success - delete when theme dir outside root",
			themeDir: func(root string) string { return "/elsewhere/manual" },
			target:   "midnight-serif",
			wantErr:  false,
		},
		{
			name:     "success - delete when theme dir is a deeper path",
			themeDir: func(root string) string { return root + "/midnight-serif/sub" },
			target:   "midnight-serif",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			s := store.NewStore(root, nil, func() string { return tt.themeDir(root) })
			ctx := context.Background()
			require.NoError(t, s.Install(ctx, testTheme(tt.target)))
			listed, err := s.List(ctx)
			require.NoError(t, err)
			require.Len(t, listed, 1)
			assert.Equal(t, tt.themeDir(root) == root+"/"+tt.target, listed[0].Active)
			err = s.Delete(ctx, tt.target)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, themegendomain.ErrThemeActive)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestStoreInstallMinifiesCSS(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		wantFiles map[string]string
	}{
		{
			name: "success - minifies stylesheets on install",
			files: map[string]string{
				"static/base.css":  ":root{\n  --color-bg: #fff;\n}",
				"static/style.css": ".btn {\n  color: red;\n}",
			},
			wantFiles: map[string]string{
				"static/base.css":  ":root{--color-bg:#fff}",
				"static/style.css": ".btn{color:red}",
			},
		},
		{
			name: "success - leaves templates byte-identical",
			files: map[string]string{
				"static/base.css":      ":root{}",
				"templates/index.html": "{{define \"body\"}}\n  <main>Hi</main>\n{{end}}",
			},
			wantFiles: map[string]string{
				"static/base.css":      ":root{}",
				"templates/index.html": "{{define \"body\"}}\n  <main>Hi</main>\n{{end}}",
			},
		},
		{
			name: "success - unbalanced css still installs with declarations intact",
			files: map[string]string{
				"static/base.css": ":root{\n  --color-bg: #fff;\n",
			},
			wantFiles: map[string]string{
				"static/base.css": ":root{--color-bg:#fff}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			s := store.NewStore(root, nil, nil)
			ctx := context.Background()
			theme := themegendomain.NewTheme("midnight-serif", themegendomain.EditionBlog, tt.files, "")
			require.NoError(t, s.Install(ctx, theme))
			got, err := s.Get(ctx, "midnight-serif")
			require.NoError(t, err)
			assert.Equal(t, tt.wantFiles, got.Files)
		})
	}
}
