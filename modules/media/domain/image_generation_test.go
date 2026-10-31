package media_test

import (
	"testing"

	media "github.com/aristorinjuang/lesstruct/modules/media/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewImageReference(t *testing.T) {
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	tests := []struct {
		name         string
		data         []byte
		wantMimeType string
		wantErr      bool
		expectErr    error
	}{
		{
			name:         "valid png",
			data:         pngHeader,
			wantMimeType: "image/png",
			wantErr:      false,
		},
		{
			name:         "valid jpeg",
			data:         []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10},
			wantMimeType: "image/jpeg",
			wantErr:      false,
		},
		{
			name:         "valid webp",
			data:         []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50},
			wantMimeType: "image/webp",
			wantErr:      false,
		},
		{
			name:      "error - empty data",
			data:      []byte{},
			wantErr:   true,
			expectErr: media.ErrInvalidFileContent,
		},
		{
			name:      "error - exceeds size limit",
			data:      append(pngHeader, make([]byte, media.MaxFileSize)...),
			wantErr:   true,
			expectErr: media.ErrFileTooLarge,
		},
		{
			name:      "error - unsupported content",
			data:      []byte("not an image"),
			wantErr:   true,
			expectErr: media.ErrInvalidFileContent,
		},
		{
			name:      "error - gif is not supported by reference-capable providers",
			data:      []byte("GIF89a"),
			wantErr:   true,
			expectErr: media.ErrInvalidFileContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := media.NewImageReference(tt.data)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.expectErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantMimeType, ref.MimeType)
			assert.Equal(t, tt.data, ref.Data)
		})
	}
}

func TestBuildGenerationPrompt(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		og       bool
		expected string
	}{
		{
			name:     "non-og prompt unchanged",
			prompt:   "A sunset over the ocean",
			og:       false,
			expected: "A sunset over the ocean",
		},
		{
			name:     "og prompt gains composition guidance",
			prompt:   "A sunset over the ocean",
			og:       true,
			expected: "A sunset over the ocean " + media.OpenGraphPromptSuffix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, media.BuildGenerationPrompt(tt.prompt, tt.og))
		})
	}
}
