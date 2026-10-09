package profilepicture_test

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/users/domain/profilepicture"
	"github.com/deepteams/webp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessor_CropAndConvertToWebP(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		targetSize int
		wantErr    bool
	}{
		{
			name:       "square image cropped to 96x96",
			width:      200,
			height:     200,
			targetSize: 96,
			wantErr:    false,
		},
		{
			name:       "landscape image center-cropped to 96x96",
			width:      300,
			height:     200,
			targetSize: 96,
			wantErr:    false,
		},
		{
			name:       "portrait image center-cropped to 96x96",
			width:      200,
			height:     300,
			targetSize: 96,
			wantErr:    false,
		},
		{
			name:       "already small image cropped",
			width:      50,
			height:     50,
			targetSize: 96,
			wantErr:    false,
		},
		{
			name:       "large image cropped to 96x96",
			width:      1920,
			height:     1080,
			targetSize: 96,
			wantErr:    false,
		},
		{
			name:       "custom target size 64x64",
			width:      200,
			height:     200,
			targetSize: 64,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := profilepicture.NewProcessor()

			img := image.NewRGBA(image.Rect(0, 0, tt.width, tt.height))
			for y := range tt.height {
				for x := range tt.width {
					img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
				}
			}

			var buf bytes.Buffer
			err := png.Encode(&buf, img)
			require.NoError(t, err, "Failed to create test image")

			reader := bytes.NewReader(buf.Bytes())
			result, err := processor.CropAndConvertToWebP(reader, tt.targetSize)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify result is non-empty WebP data
			assert.NotEmpty(t, result, "Expected non-empty WebP output")

			// WebP files start with "RIFF" magic
			assert.True(t, len(result) >= 4, "Result too short for magic bytes")
			assert.Equal(t, byte('R'), result[0])
			assert.Equal(t, byte('I'), result[1])
			assert.Equal(t, byte('F'), result[2])
			assert.Equal(t, byte('F'), result[3])
		})
	}
}

func TestProcessor_CropAndConvertToWebP_UnsupportedFormat(t *testing.T) {
	processor := profilepicture.NewProcessor()

	// Pass a raw byte slice that is not a valid image
	buf := bytes.NewReader([]byte("not an image"))
	_, err := processor.CropAndConvertToWebP(buf, 96)
	require.Error(t, err)
}

func TestProcessor_CropAndConvertToWebP_VP8XAlphaInput(t *testing.T) {
	processor := profilepicture.NewProcessor()

	// VP8X (extended container with alpha) WebP input — the deepteams
	// self-registered decoder fails on these; the processor must route WebP
	// through the explicit x/image driver.
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := range 200 {
		for x := range 200 {
			a := uint8(255)
			if (x+y)%3 == 0 {
				a = 128
			}
			img.Set(x, y, color.RGBA{100, 150, 200, a})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, webp.Encode(&buf, img, &webp.EncoderOptions{Quality: 80, Method: 4}))

	result, err := processor.CropAndConvertToWebP(bytes.NewReader(buf.Bytes()), 96)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Equal(t, byte('R'), result[0])
}

func TestProcessor_CropAndConvertToWebP_ReadErrorPropagated(t *testing.T) {
	processor := profilepicture.NewProcessor()

	_, err := processor.CropAndConvertToWebP(errorReader{}, 96)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced read error")
}

type errorReader struct{}

func (errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("forced read error")
}

func TestProcessor_CropAndConvertToWebP_EncodeError(t *testing.T) {
	processor := profilepicture.NewProcessor()

	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := range 200 {
		for x := range 200 {
			img.Set(x, y, color.RGBA{100, 150, 200, 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	// A zero target size decodes fine but the WebP encoder rejects the
	// empty canvas, covering the encode-failure branch.
	_, err := processor.CropAndConvertToWebP(bytes.NewReader(buf.Bytes()), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to encode webp")
}

// hugeDimsPNG builds a minimal PNG whose IHDR declares width×height without
// any pixel data — enough for DecodeConfig, a decompression bomb for a full
// decode if the pixel budget is not enforced.
func hugeDimsPNG(t *testing.T, width, height int) []byte {
	t.Helper()

	var buf bytes.Buffer
	buf.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})

	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], uint32(width))
	binary.BigEndian.PutUint32(ihdr[4:8], uint32(height))
	ihdr[8] = 8 // bit depth
	ihdr[9] = 6 // color type: RGBA

	writeChunk := func(typ string, data []byte) {
		var lenb [4]byte
		binary.BigEndian.PutUint32(lenb[:], uint32(len(data)))
		buf.Write(lenb[:])
		buf.WriteString(typ)
		buf.Write(data)
		var crc [4]byte
		binary.BigEndian.PutUint32(crc[:], crc32.ChecksumIEEE(append([]byte(typ), data...)))
		buf.Write(crc[:])
	}

	writeChunk("IHDR", ihdr)

	// A single-row IDAT keeps the container structurally valid; DecodeConfig
	// only reads the IHDR, and a full decode is what the budget must prevent.
	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	_, _ = zw.Write([]byte{0})
	_ = zw.Close()
	writeChunk("IDAT", zbuf.Bytes())

	writeChunk("IEND", nil)
	return buf.Bytes()
}

func TestProcessor_CropAndConvertToWebP_DecompressionBomb(t *testing.T) {
	// A tiny upload declaring 100000×10000 pixels (1 GP) must be rejected by
	// the pixel budget before the full decode allocates width×height memory.
	data := hugeDimsPNG(t, 100000, 10000)
	processor := profilepicture.NewProcessor()

	_, err := processor.CropAndConvertToWebP(bytes.NewReader(data), profilepicture.ProfilePictureSize)

	require.Error(t, err)
	assert.ErrorContains(t, err, "image dimensions exceed")
}
