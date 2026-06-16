package thumbnail_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aristorinjuang/lesstruct/internal/domain/thumbnail"
)

func TestThumbnailConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  thumbnail.ThumbnailConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: thumbnail.ThumbnailConfig{
				MaxWidth: 370,
				Suffix:   "_thumb",
			},
			wantErr: false,
		},
		{
			name: "valid config with large width",
			config: thumbnail.ThumbnailConfig{
				MaxWidth: 1600,
				Suffix:   "_large",
			},
			wantErr: false,
		},
		{
			name: "invalid - zero max_width",
			config: thumbnail.ThumbnailConfig{
				MaxWidth: 0,
				Suffix:   "_thumb",
			},
			wantErr: true,
		},
		{
			name: "invalid - negative max_width",
			config: thumbnail.ThumbnailConfig{
				MaxWidth: -1,
				Suffix:   "_thumb",
			},
			wantErr: true,
		},
		{
			name: "invalid - empty suffix",
			config: thumbnail.ThumbnailConfig{
				MaxWidth: 370,
				Suffix:   "",
			},
			wantErr: true,
		},
		{
			name: "invalid - suffix without underscore",
			config: thumbnail.ThumbnailConfig{
				MaxWidth: 370,
				Suffix:   "thumb",
			},
			wantErr: true,
		},
		{
			name: "invalid - suffix only underscore",
			config: thumbnail.ThumbnailConfig{
				MaxWidth: 370,
				Suffix:   "_",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateUnique(t *testing.T) {
	tests := []struct {
		name    string
		configs []thumbnail.ThumbnailConfig
		wantErr bool
	}{
		{
			name: "unique suffixes",
			configs: []thumbnail.ThumbnailConfig{
				{MaxWidth: 370, Suffix: "_thumb"},
				{MaxWidth: 800, Suffix: "_medium"},
				{MaxWidth: 1600, Suffix: "_large"},
			},
			wantErr: false,
		},
		{
			name: "duplicate suffixes",
			configs: []thumbnail.ThumbnailConfig{
				{MaxWidth: 370, Suffix: "_thumb"},
				{MaxWidth: 800, Suffix: "_thumb"},
			},
			wantErr: true,
		},
		{
			name: "empty slice",
			configs: []thumbnail.ThumbnailConfig{},
			wantErr: false,
		},
		{
			name: "single entry",
			configs: []thumbnail.ThumbnailConfig{
				{MaxWidth: 370, Suffix: "_thumb"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := thumbnail.ValidateUnique(tt.configs)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestThumbnailConfigErrors(t *testing.T) {
	t.Run("ErrInvalidMaxWidth", func(t *testing.T) {
		assert.EqualError(t, thumbnail.ErrInvalidMaxWidth, "max_width must be an integer greater than 0")
	})
	t.Run("ErrInvalidSuffix", func(t *testing.T) {
		assert.EqualError(t, thumbnail.ErrInvalidSuffix, "suffix must be non-empty and start with '_'")
	})
	t.Run("ErrDuplicateSuffix", func(t *testing.T) {
		assert.EqualError(t, thumbnail.ErrDuplicateSuffix, "suffix must be unique across all thumbnail entries")
	})
}
