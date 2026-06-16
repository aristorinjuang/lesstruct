package thumbnail_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aristorinjuang/lesstruct/internal/domain/thumbnail"
)

func TestNewService(t *testing.T) {
	svc := thumbnail.NewService()
	configs := svc.GetAll()
	require.Len(t, configs, 1)
	assert.Equal(t, 370, configs[0].MaxWidth)
	assert.Equal(t, "_thumb", configs[0].Suffix)
}

func TestGetAll(t *testing.T) {
	svc := thumbnail.NewService()

	t.Run("returns default configs", func(t *testing.T) {
		configs := svc.GetAll()
		require.Len(t, configs, 1)
		assert.Equal(t, 370, configs[0].MaxWidth)
		assert.Equal(t, "_thumb", configs[0].Suffix)
	})
}

func TestGetBySuffix(t *testing.T) {
	svc := thumbnail.NewService()

	t.Run("found", func(t *testing.T) {
		cfg, err := svc.GetBySuffix("_thumb")
		require.NoError(t, err)
		assert.Equal(t, 370, cfg.MaxWidth)
		assert.Equal(t, "_thumb", cfg.Suffix)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.GetBySuffix("_nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, thumbnail.ErrSuffixNotFound)
	})
}

func TestLoadConfigFromFile(t *testing.T) {
	t.Run("valid toml file", func(t *testing.T) {
		svc := thumbnail.NewService()
		path := filepath.Join(t.TempDir(), "thumbnails.toml")
		data := `
[[thumbnail]]
max_width = 800
suffix = "_medium"

[[thumbnail]]
max_width = 1600
suffix = "_large"
`
		require.NoError(t, os.WriteFile(path, []byte(data), 0644))

		err := svc.LoadConfigFromFile(path)
		require.NoError(t, err)

		configs := svc.GetAll()
		require.Len(t, configs, 2)

		med, err := svc.GetBySuffix("_medium")
		require.NoError(t, err)
		assert.Equal(t, 800, med.MaxWidth)

		lrg, err := svc.GetBySuffix("_large")
		require.NoError(t, err)
		assert.Equal(t, 1600, lrg.MaxWidth)
	})

	t.Run("missing file uses defaults", func(t *testing.T) {
		svc := thumbnail.NewService()
		path := filepath.Join(t.TempDir(), "nonexistent.toml")

		err := svc.LoadConfigFromFile(path)
		require.NoError(t, err)

		configs := svc.GetAll()
		require.Len(t, configs, 1)
		assert.Equal(t, "_thumb", configs[0].Suffix)
	})

	t.Run("invalid toml syntax", func(t *testing.T) {
		svc := thumbnail.NewService()
		path := filepath.Join(t.TempDir(), "invalid.toml")
		data := `[[thumbnail`
		require.NoError(t, os.WriteFile(path, []byte(data), 0644))

		err := svc.LoadConfigFromFile(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing thumbnail config")
	})

	t.Run("invalid config entries", func(t *testing.T) {
		svc := thumbnail.NewService()
		path := filepath.Join(t.TempDir(), "badconfig.toml")
		data := `
[[thumbnail]]
max_width = 0
suffix = "_thumb"
`
		require.NoError(t, os.WriteFile(path, []byte(data), 0644))

		err := svc.LoadConfigFromFile(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validating thumbnail config")
	})

	t.Run("duplicate suffixes", func(t *testing.T) {
		svc := thumbnail.NewService()
		path := filepath.Join(t.TempDir(), "dup.toml")
		data := `
[[thumbnail]]
max_width = 370
suffix = "_thumb"

[[thumbnail]]
max_width = 800
suffix = "_thumb"
`
		require.NoError(t, os.WriteFile(path, []byte(data), 0644))

		err := svc.LoadConfigFromFile(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validating thumbnail config")
	})

	t.Run("unreadable file path is a directory", func(t *testing.T) {
		svc := thumbnail.NewService()
		path := t.TempDir()

		err := svc.LoadConfigFromFile(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reading thumbnail config")
	})

	t.Run("empty toml file preserves defaults", func(t *testing.T) {
		svc := thumbnail.NewService()
		path := filepath.Join(t.TempDir(), "empty.toml")
		data := `# Just a comment, no [[thumbnail]] entries`
		require.NoError(t, os.WriteFile(path, []byte(data), 0644))

		err := svc.LoadConfigFromFile(path)
		require.NoError(t, err)

		configs := svc.GetAll()
		require.Len(t, configs, 1)
		assert.Equal(t, "_thumb", configs[0].Suffix)
	})
}
