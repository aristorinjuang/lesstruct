package blog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateSnapshotTheme(t *testing.T) {
	tests := []struct {
		name         string
		themeName    string
		noCfg        bool
		setup        func(t *testing.T, themesRoot string)
		wantContains []string
		wantErr      bool
	}{
		{
			name:      "success - stylesheet theme renders sample homepage",
			themeName: "midnight",
			setup: func(t *testing.T, themesRoot string) {
				writeInstalledTheme(t, themesRoot, "midnight", "snapshot-marker", "")
			},
			wantContains: []string{"Sample Post", "snapshot-marker"},
			wantErr:      false,
		},
		{
			name:      "success - template override renders through",
			themeName: "custom",
			setup: func(t *testing.T, themesRoot string) {
				writeInstalledTheme(t, themesRoot, "custom", "snapshot-marker", "")
				dir := filepath.Join(themesRoot, "custom", "templates")
				require.NoError(t, os.MkdirAll(dir, 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(dir, "homepage.html"),
					[]byte(`{{define "body"}}<main>SNAPSHOT-CUSTOM</main>{{end}}`),
					0o644,
				))
			},
			wantContains: []string{"SNAPSHOT-CUSTOM", "snapshot-marker"},
			wantErr:      false,
		},
		{
			name:         "error - unknown theme",
			themeName:    "missing",
			setup:        nil,
			wantContains: nil,
			wantErr:      true,
		},
		{
			name:         "error - invalid theme name",
			themeName:    "Bad Name!",
			setup:        nil,
			wantContains: nil,
			wantErr:      true,
		},
		{
			name:      "error - broken template",
			themeName: "broken",
			setup: func(t *testing.T, themesRoot string) {
				writeInstalledTheme(t, themesRoot, "broken", "snapshot-marker", `{{define "body"}}<div>unclosed`)
			},
			wantContains: nil,
			wantErr:      true,
		},
		{
			name:         "error - unavailable before mount",
			themeName:    "midnight",
			noCfg:        true,
			setup:        nil,
			wantContains: nil,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installTestThemeRegistry(t)
			dir := t.TempDir()
			themesRoot := filepath.Join(dir, "themes")
			if tt.setup != nil {
				tt.setup(t, themesRoot)
			}
			cfg := &config.Config{
				ConfigDir:  dir,
				ConfigFile: "config.toml",
				ThemesDir:  themesRoot,
			}
			s := &state{cfg: cfg, snapshotSlugs: []string{"post"}}
			if tt.noCfg {
				s.cfg = nil
			}
			doc, err := s.SnapshotTheme(tt.themeName)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			for _, want := range tt.wantContains {
				assert.Contains(t, doc, want)
			}
		})
	}
}
