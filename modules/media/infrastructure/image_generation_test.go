package infrastructure_test

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

	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	mediainfra "github.com/aristorinjuang/lesstruct/modules/media/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageGenerationService_SupportsImageReferences(t *testing.T) {
	tests := []struct {
		name      string
		service   mediadomain.ImageGenerationService
		supported bool
	}{
		{
			name:      "imagen does not support reference images",
			service:   mediainfra.NewGoogleImagen4Service("key", "imagen-4.0-fast-generate-001", "", ""),
			supported: false,
		},
		{
			name:      "gemini supports reference images",
			service:   mediainfra.NewGoogleGeminiService("key", "gemini-2.5-flash-image", "", ""),
			supported: true,
		},
		{
			name:      "gpt image supports reference images",
			service:   mediainfra.NewGPTImageService("key", "", "gpt-image-1", ""),
			supported: true,
		},
		{
			name:      "muse image supports reference images",
			service:   mediainfra.NewGPTImageService("key", "https://api.meta.ai/v1", "muse-image-1.0", ""),
			supported: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.supported, tt.service.SupportsImageReferences())
		})
	}
}

func TestGPTImageService_GenerateImageWithReferences_PartNaming(t *testing.T) {
	pngData := encodeTestImage(t, "png", 8, 6)
	jpegData := encodeTestImage(t, "jpeg", 6, 8)

	var (
		gotPrompt       string
		gotFieldNames   []string
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
		for fieldName, headers := range r.MultipartForm.File {
			for _, h := range headers {
				gotFieldNames = append(gotFieldNames, fieldName)
				gotFilenames = append(gotFilenames, h.Filename)
				gotContentTypes = append(gotContentTypes, h.Header.Get("Content-Type"))
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"created":1,"data":[{"b64_json":"%s"}]}`, base64.StdEncoding.EncodeToString(pngData))
	}))
	defer server.Close()

	tests := []struct {
		name       string
		baseURL    string
		wantFields []string
	}{
		{
			name:       "openai keeps the SDK image array naming",
			baseURL:    "",
			wantFields: []string{"image[]", "image[]"},
		},
		{
			name:       "compatible providers get indexed part names",
			baseURL:    server.URL,
			wantFields: []string{"image[0]", "image[1]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPrompt = ""
			gotFieldNames = nil
			gotFilenames = nil
			gotContentTypes = nil
			if tt.baseURL == "" {
				t.Setenv("OPENAI_BASE_URL", server.URL)
			}
			service := mediainfra.NewGPTImageService("test-key", tt.baseURL, "gpt-image-1", "")
			out, err := service.GenerateImage(context.Background(), "a sunset", []mediadomain.ImageReference{
				{Data: pngData, MimeType: "image/png"},
				{Data: jpegData, MimeType: "image/jpeg"},
			})
			require.NoError(t, err)
			assert.Equal(t, pngData, out)
			assert.Equal(t, "a sunset", gotPrompt)
			assert.ElementsMatch(t, tt.wantFields, gotFieldNames)
			assert.ElementsMatch(t, []string{"reference-1.png", "reference-2.jpg"}, gotFilenames)
			assert.ElementsMatch(t, []string{"image/png", "image/jpeg"}, gotContentTypes)
		})
	}
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
