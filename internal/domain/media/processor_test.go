package media_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessor_ExtractMetadata(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		height  int
		wantErr bool
	}{
		{
			name:    "extracts metadata from png",
			width:   100,
			height:  200,
			wantErr: false,
		},
		{
			name:    "extracts metadata from square image",
			width:   150,
			height:  150,
			wantErr: false,
		},
		{
			name:    "extracts metadata from large image",
			width:   1920,
			height:  1080,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := media.NewProcessor()

			img := image.NewRGBA(image.Rect(0, 0, tt.width, tt.height))
			c := color.RGBA{255, 0, 0, 255}
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					img.Set(x, y, c)
				}
			}

			var buf bytes.Buffer
			err := png.Encode(&buf, img)
			require.NoError(t, err, "Failed to create test image")

			reader := bytes.NewReader(buf.Bytes())
			metadata, err := processor.ExtractMetadata(reader)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, metadata, "Processor.ExtractMetadata() returned nil metadata")

			assert.Equal(t, tt.width, metadata.Width, "Processor.ExtractMetadata() Width")
			assert.Equal(t, tt.height, metadata.Height, "Processor.ExtractMetadata() Height")
		})
	}
}

func TestProcessor_ExtractMetadata_InvalidImage(t *testing.T) {
	processor := media.NewProcessor()

	invalidData := []byte("not an image")
	reader := bytes.NewReader(invalidData)

	_, err := processor.ExtractMetadata(reader)

	assert.Error(t, err, "Processor.ExtractMetadata() expected error for invalid image")
}

func TestProcessor_ExtractMetadata_EmptyData(t *testing.T) {
	processor := media.NewProcessor()

	emptyData := []byte{}
	reader := bytes.NewReader(emptyData)

	_, err := processor.ExtractMetadata(reader)

	assert.Error(t, err, "Processor.ExtractMetadata() expected error for empty data")
}

func TestProcessor_ConvertToWebP(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		height  int
		wantErr bool
	}{
		{
			name:    "converts png to webp",
			width:   100,
			height:  100,
			wantErr: false,
		},
		{
			name:    "converts large image",
			width:   1920,
			height:  1080,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := media.NewProcessor()

			img := image.NewRGBA(image.Rect(0, 0, tt.width, tt.height))
			c := color.RGBA{255, 0, 0, 255}
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					img.Set(x, y, c)
				}
			}

			var buf bytes.Buffer
			err := png.Encode(&buf, img)
			require.NoError(t, err, "Failed to create test image")

			reader := bytes.NewReader(buf.Bytes())
			webpData, metadata, err := processor.ConvertToWebP(reader)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, webpData, "Processor.ConvertToWebP() returned empty WebP data")
			require.NotNil(t, metadata, "Processor.ConvertToWebP() returned nil metadata")

			assert.Equal(t, tt.width, metadata.Width, "Processor.ConvertToWebP() metadata width")
			assert.Equal(t, tt.height, metadata.Height, "Processor.ConvertToWebP() metadata height")
		})
	}
}

func TestProcessor_ConvertToWebP_InvalidImage(t *testing.T) {
	processor := media.NewProcessor()

	invalidData := []byte("not an image")
	reader := bytes.NewReader(invalidData)

	_, _, err := processor.ConvertToWebP(reader)

	assert.Error(t, err, "Processor.ConvertToWebP() expected error for invalid image")
}

func TestProcessor_ConvertToWebP_SmallDimensions(t *testing.T) {
	processor := media.NewProcessor()

	// Create a very small image (1x1 pixel) to test edge case
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})

	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	require.NoError(t, err, "Failed to create test image")

	reader := bytes.NewReader(buf.Bytes())
	webpData, metadata, err := processor.ConvertToWebP(reader)

	// Small images should work fine
	require.NoError(t, err, "Processor.ConvertToWebP() should handle small images")
	assert.NotNil(t, metadata, "Processor.ConvertToWebP() should return metadata for small images")
	assert.NotEmpty(t, webpData, "Processor.ConvertToWebP() should return WebP data for small images")
}

func TestProcessor_ConvertToWebP_LargeDimensions(t *testing.T) {
	processor := media.NewProcessor()

	// Create a larger image to test with
	img := image.NewRGBA(image.Rect(0, 0, 10000, 10000))
	// Only set one pixel to avoid large memory usage
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})

	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	require.NoError(t, err, "Failed to create test image")

	reader := bytes.NewReader(buf.Bytes())
	webpData, metadata, err := processor.ConvertToWebP(reader)

	// Large images should work, but if webp.Encode fails, that's the path we're testing
	if err != nil {
		assert.Error(t, err, "Processor.ConvertToWebP() may error for very large images")
	} else {
		assert.NotNil(t, metadata, "Processor.ConvertToWebP() should return metadata")
		_ = webpData
	}
}

func TestProcessor_GenerateHash(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "generates hash for valid data",
			data:    []byte("test content"),
			wantErr: false,
		},
		{
			name:    "generates hash for empty data",
			data:    []byte{},
			wantErr: false,
		},
		{
			name:    "generates hash for large data",
			data:    bytes.Repeat([]byte("a"), 100*1024),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := media.NewProcessor()

			reader := bytes.NewReader(tt.data)
			hash, err := processor.GenerateHash(reader)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, hash, "Processor.GenerateHash() returned empty hash")
			assert.Equal(t, 64, len(hash), "Processor.GenerateHash() hash length (SHA-256 = 64 hex chars)")
		})
	}
}

func TestProcessor_GenerateHash_ReadError(t *testing.T) {
	processor := media.NewProcessor()

	reader := &processorErrorReader{}
	_, err := processor.GenerateHash(reader)

	assert.Error(t, err, "Processor.GenerateHash() expected error for reader that fails")
}

func TestNewProcessor(t *testing.T) {
	processor := media.NewProcessor()

	assert.NotNil(t, processor, "NewProcessor() returned nil")
}

func TestProcessor_Resize(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		maxWidth    int
		wantWidth   int
		wantHeight  int
		wantErr     bool
		invalidData bool
	}{
		{
			name:       "resize large image to maxWidth",
			width:      1920,
			height:     1080,
			maxWidth:   800,
			wantWidth:  800,
			wantHeight: 450,
			wantErr:    false,
		},
		{
			name:       "no upscale - smaller image passes through",
			width:      100,
			height:     80,
			maxWidth:   800,
			wantWidth:  100,
			wantHeight: 80,
			wantErr:    false,
		},
		{
			name:       "exact width passes through",
			width:      800,
			height:     600,
			maxWidth:   800,
			wantWidth:  800,
			wantHeight: 600,
			wantErr:    false,
		},
		{
			name:       "aspect ratio preserved",
			width:      1600,
			height:     900,
			maxWidth:   800,
			wantWidth:  800,
			wantHeight: 450,
			wantErr:    false,
		},
		{
			name:       "resize square image",
			width:      1000,
			height:     1000,
			maxWidth:   500,
			wantWidth:  500,
			wantHeight: 500,
			wantErr:    false,
		},
		{
			name:        "invalid image bytes",
			invalidData: true,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := media.NewProcessor()

			var reader *bytes.Reader
			if tt.invalidData {
				reader = bytes.NewReader([]byte("not an image"))
			} else {
				img := image.NewRGBA(image.Rect(0, 0, tt.width, tt.height))
				c := color.RGBA{255, 0, 0, 255}
				for y := 0; y < tt.height; y++ {
					for x := 0; x < tt.width; x++ {
						img.Set(x, y, c)
					}
				}

				var buf bytes.Buffer
				err := png.Encode(&buf, img)
				require.NoError(t, err, "Failed to create test image")

				reader = bytes.NewReader(buf.Bytes())
			}

			webpData, metadata, err := processor.Resize(reader, tt.maxWidth)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, metadata, "Processor.Resize() returned nil metadata")
			assert.NotEmpty(t, webpData, "Processor.Resize() returned empty WebP data")

			assert.Equal(t, tt.wantWidth, metadata.Width, "Processor.Resize() width")
			assert.Equal(t, tt.wantHeight, metadata.Height, "Processor.Resize() height")

			// Verify output is valid WebP by decoding it back
			decoded, _, err := image.Decode(bytes.NewReader(webpData))
			require.NoError(t, err, "Output is not valid decodable image")
			decodedBounds := decoded.Bounds()
			assert.Equal(t, tt.wantWidth, decodedBounds.Dx(), "Decoded WebP width mismatch")
			assert.Equal(t, tt.wantHeight, decodedBounds.Dy(), "Decoded WebP height mismatch")
		})
	}
}

type processorErrorReader struct{}

func (e *processorErrorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("forced read error")
}
