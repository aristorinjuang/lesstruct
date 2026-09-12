package media_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/media"
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

func TestImageGenerationService_SupportsImageReferences(t *testing.T) {
	tests := []struct {
		name      string
		service   media.ImageGenerationService
		supported bool
	}{
		{
			name:      "imagen does not support reference images",
			service:   media.NewGoogleImagen4Service("key", "imagen-4.0-fast-generate-001", "", ""),
			supported: false,
		},
		{
			name:      "gemini supports reference images",
			service:   media.NewGoogleGeminiService("key", "gemini-2.5-flash-image", "", ""),
			supported: true,
		},
		{
			name:      "gpt image supports reference images",
			service:   media.NewGPTImageService("key", "gpt-image-1", ""),
			supported: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.supported, tt.service.SupportsImageReferences())
		})
	}
}

func TestGPTImageService_GenerateImageWithReferences_SendsNamedFiles(t *testing.T) {
	pngData := encodeTestImage(t, "png", 8, 6)
	jpegData := encodeTestImage(t, "jpeg", 6, 8)

	var (
		gotPrompt       string
		gotFilenames    []string
		gotContentTypes []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/edits" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "cannot parse multipart form", http.StatusBadRequest)
			return
		}
		gotPrompt = r.FormValue("prompt")
		for _, headers := range r.MultipartForm.File {
			for _, h := range headers {
				gotFilenames = append(gotFilenames, h.Filename)
				gotContentTypes = append(gotContentTypes, h.Header.Get("Content-Type"))
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"created":1,"data":[{"b64_json":"%s"}]}`, base64.StdEncoding.EncodeToString(pngData))
	}))
	defer server.Close()

	t.Setenv("OPENAI_BASE_URL", server.URL)

	service := media.NewGPTImageService("test-key", "gpt-image-1", "")
	out, err := service.GenerateImage(context.Background(), "a sunset", []media.ImageReference{
		{Data: pngData, MimeType: "image/png"},
		{Data: jpegData, MimeType: "image/jpeg"},
	})
	require.NoError(t, err)
	assert.Equal(t, pngData, out)
	assert.Equal(t, "a sunset", gotPrompt)
	assert.ElementsMatch(t, []string{"reference-1.png", "reference-2.jpg"}, gotFilenames)
	assert.ElementsMatch(t, []string{"image/png", "image/jpeg"}, gotContentTypes)
}

// encodeTestImage builds small valid image bytes in the given format.
func encodeTestImage(t *testing.T, format string, width, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{30, 120, 200, 255})
		}
	}
	var buf bytes.Buffer
	var err error
	if format == "jpeg" {
		err = jpeg.Encode(&buf, img, nil)
	} else {
		err = png.Encode(&buf, img)
	}
	require.NoError(t, err)
	return buf.Bytes()
}
