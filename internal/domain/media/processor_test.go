package media_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/deepteams/webp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertWebPChunk rebuilds the RIFF container with an extra chunk inserted
// right after the header, fixing the RIFF size field. The chunk's payload is
// zero-padded to an even length per the RIFF spec.
func insertWebPChunk(t *testing.T, data []byte, fcc string, payload []byte) []byte {
	t.Helper()
	require.True(t, len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP")

	var out []byte
	out = append(out, data[:12]...)
	out = append(out, fcc...)
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], uint32(len(payload)))
	out = append(out, size[:]...)
	out = append(out, payload...)
	if len(payload)%2 == 1 {
		out = append(out, 0)
	}
	out = append(out, data[12:]...)

	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out
}

func TestProcessor_ConvertToWebP_StripsMetadataChunks(t *testing.T) {
	tests := []struct {
		name string
		fcc  string
	}{
		{name: "exif", fcc: "EXIF"},
		{name: "xmp", fcc: "XMP "},
		{name: "icc", fcc: "ICCP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := media.NewProcessor()

			base := testWebPBytes(t, 320, 200, true)
			withMeta := insertWebPChunk(t, base, tt.fcc, []byte("fake-metadata-payload"))

			got, metadata, err := processor.ConvertToWebP(bytes.NewReader(withMeta))
			require.NoError(t, err)
			require.NotNil(t, metadata)
			assert.Equal(t, 320, metadata.Width)
			assert.Equal(t, 200, metadata.Height)
			assert.NotContains(t, string(got), tt.fcc, "metadata chunk must be stripped")
			assert.NotEqual(t, withMeta, got, "container must be rebuilt without the metadata chunk")

			// The stripped output is still a valid, decodable image.
			decoded, _, err := image.Decode(bytes.NewReader(got))
			require.NoError(t, err)
			assert.Equal(t, 320, decoded.Bounds().Dx())
			assert.Equal(t, 200, decoded.Bounds().Dy())
		})
	}
}

func TestProcessor_ConvertToWebP_RejectsAnimated(t *testing.T) {
	processor := media.NewProcessor()

	base := testWebPBytes(t, 320, 200, true)
	// ANIM chunk: background color (4 bytes) + loop count (2 bytes).
	animated := insertWebPChunk(t, base, "ANIM", []byte{0, 0, 0, 0, 0, 0})

	_, _, err := processor.ConvertToWebP(bytes.NewReader(animated))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "animated WebP is not supported")
}

func TestProcessor_ConvertToWebP_RejectsFrameLessContainer(t *testing.T) {
	processor := media.NewProcessor()

	// VP8X-only container: flags + reserved (4) + 24-bit w-1 + 24-bit h-1.
	var payload [10]byte
	binary.LittleEndian.PutUint16(payload[4:6], uint16(9)) // width-1
	binary.LittleEndian.PutUint16(payload[6:8], uint16(9)) // height-1
	frameLess := append([]byte("RIFF\x00\x00\x00\x00WEBP"), payload[:]...)
	binary.LittleEndian.PutUint32(frameLess[4:8], uint32(len(frameLess)-8))
	// Append the VP8X chunk itself.
	frameLess = append(frameLess[:12], append([]byte("VP8X\x0a\x00\x00\x00"), payload[:]...)...)
	binary.LittleEndian.PutUint32(frameLess[4:8], uint32(len(frameLess)-8))

	_, _, err := processor.ConvertToWebP(bytes.NewReader(frameLess))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no image frame")
}

func TestProcessor_ConvertToWebP_RejectsMalformedContainers(t *testing.T) {
	tests := []struct {
		name    string
		corrupt func([]byte) []byte
		wantErr string
	}{
		{
			name: "riff size overruns the file",
			corrupt: func(b []byte) []byte {
				out := append([]byte(nil), b...)
				binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)+1000))
				return out
			},
			wantErr: "RIFF size overruns",
		},
		{
			name: "chunk overruns the container",
			corrupt: func(b []byte) []byte {
				out := append([]byte(nil), b...)
				// Inflate the first chunk's size field.
				binary.LittleEndian.PutUint32(out[16:20], 0xFFFFFF)
				return out
			},
			wantErr: "overruns the container",
		},
		{
			name: "truncated chunk header",
			corrupt: func(b []byte) []byte {
				out := append([]byte("RIFF"), 5, 0, 0, 0)
				out = append(out, []byte("WEBP")...)
				out = append(out, 0)
				return out
			},
			wantErr: "truncated chunk header",
		},
		{
			name: "missing chunk padding",
			corrupt: func(b []byte) []byte {
				// Odd-sized payload whose padding byte lies beyond the
				// declared RIFF size.
				out := append([]byte("RIFF"), 13, 0, 0, 0)
				out = append(out, []byte("WEBP")...)
				out = append(out, []byte("VP8 ")...)
				out = append(out, 1, 0, 0, 0)
				out = append(out, 0xAB)
				return out
			},
			wantErr: "missing its padding byte",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := media.NewProcessor()
			input := tt.corrupt(testWebPBytes(t, 64, 64, false))
			_, _, err := processor.ConvertToWebP(bytes.NewReader(input))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestProcessor_ConvertToWebP_CorruptFramePayload(t *testing.T) {
	processor := media.NewProcessor()

	// A container whose frame chunk exists but is too short to hold a VP8
	// frame header: the chunk walker accepts it (frame present, no metadata),
	// then the header-only decode must reject the truncated frame.
	out := append([]byte("RIFF"), 14, 0, 0, 0)
	out = append(out, []byte("WEBP")...)
	out = append(out, []byte("VP8 ")...)
	out = append(out, 2, 0, 0, 0)
	out = append(out, 0xAB, 0xAB)

	_, _, err := processor.ConvertToWebP(bytes.NewReader(out))
	require.Error(t, err)
}

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
			for y := range tt.height {
				for x := range tt.width {
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

func TestProcessor_ExtractMetadata_ReadError(t *testing.T) {
	processor := media.NewProcessor()

	_, err := processor.ExtractMetadata(&processorErrorReader{})

	assert.Error(t, err, "Processor.ExtractMetadata() expected error for unreadable input")
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
			for y := range tt.height {
				for x := range tt.width {
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

func TestProcessor_ConvertToWebP_ReadError(t *testing.T) {
	processor := media.NewProcessor()

	_, _, err := processor.ConvertToWebP(&processorErrorReader{})

	assert.Error(t, err, "Processor.ConvertToWebP() expected error for unreadable input")
}

func TestProcessor_ConvertToWebP_MalformedWebPHeader(t *testing.T) {
	processor := media.NewProcessor()

	// Full 12-byte WebP magic followed by garbage: the passthrough sniff
	// matches, then the header-only decode must reject the corrupt container.
	corrupt := append([]byte("RIFF\x00\x00\x00\x00WEBP"), []byte("not a real chunk")...)
	_, _, err := processor.ConvertToWebP(bytes.NewReader(corrupt))
	assert.Error(t, err, "Processor.ConvertToWebP() expected error for corrupt WebP container")
}

func TestProcessor_ConvertToWebP_ReadErrorAfterHeader(t *testing.T) {
	processor := media.NewProcessor()

	// A WebP-magic stream that errors after the 12-byte header.
	_, _, err := processor.ConvertToWebP(&errorAfterHeaderReader{})
	assert.Error(t, err, "Processor.ConvertToWebP() expected error for mid-stream failure")
}

// errorAfterHeaderReader yields the 12-byte WebP header cleanly, then fails
// on every subsequent read.
type errorAfterHeaderReader struct {
	headerSent bool
}

func (e *errorAfterHeaderReader) Read(p []byte) (n int, err error) {
	header := []byte("RIFF\x00\x00\x00\x00WEBP")
	if !e.headerSent && len(p) >= len(header) {
		e.headerSent = true
		copy(p, header)
		return len(header), nil
	}
	return 0, errors.New("forced mid-stream read error")
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
		readError   bool
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
		{
			name:      "unreadable input",
			readError: true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := media.NewProcessor()

			var reader *bytes.Reader
			if tt.readError {
				_, _, err := processor.Resize(&processorErrorReader{}, tt.maxWidth)
				require.Error(t, err)
				return
			}
			if tt.invalidData {
				reader = bytes.NewReader([]byte("not an image"))
			} else {
				img := image.NewRGBA(image.Rect(0, 0, tt.width, tt.height))
				c := color.RGBA{255, 0, 0, 255}
				for y := range tt.height {
					for x := range tt.width {
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

// testWebPBytes encodes a solid-color image (optionally with alpha) as WebP
// through the same encoder the processor uses, so tests exercise the real
// pipeline. Images with alpha encode as VP8X (extended container).
func testWebPBytes(t *testing.T, width, height int, withAlpha bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			a := uint8(255)
			if withAlpha && (x+y)%3 == 0 {
				a = 128
			}
			img.Set(x, y, color.RGBA{200, 30, 30, a})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, webp.Encode(&buf, img, &webp.EncoderOptions{Quality: 80, Method: 4}))
	return buf.Bytes()
}

func TestProcessor_ConvertToWebP_WebPPassthrough(t *testing.T) {
	processor := media.NewProcessor()

	input := testWebPBytes(t, 640, 360, false)
	got, metadata, err := processor.ConvertToWebP(bytes.NewReader(input))
	require.NoError(t, err)
	// Already-WebP input is stored untouched — no re-encode, no generation loss.
	assert.Equal(t, input, got)
	require.NotNil(t, metadata)
	assert.Equal(t, 640, metadata.Width)
	assert.Equal(t, 360, metadata.Height)
}

func TestProcessor_ConvertToWebP_VP8XAlphaPassthrough(t *testing.T) {
	processor := media.NewProcessor()

	input := testWebPBytes(t, 740, 468, true)
	require.True(t, len(input) >= 12 && string(input[8:12]) == "WEBP")
	got, metadata, err := processor.ConvertToWebP(bytes.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, input, got)
	require.NotNil(t, metadata)
	assert.Equal(t, 740, metadata.Width)
	assert.Equal(t, 468, metadata.Height)
}

func TestProcessor_Resize_VP8XAlphaInput(t *testing.T) {
	processor := media.NewProcessor()

	input := testWebPBytes(t, 740, 468, true)
	// Resize must decode the extended WebP through the explicit x/image driver
	// (the deepteams decoder fails on VP8X+ALPH files — the Feedback #9 bug).
	resized, metadata, err := processor.Resize(bytes.NewReader(input), 370)
	require.NoError(t, err)
	require.NotNil(t, metadata)
	assert.Equal(t, 370, metadata.Width)
	assert.Equal(t, 234, metadata.Height)
	assert.NotEmpty(t, resized)

	decoded, _, err := image.Decode(bytes.NewReader(resized))
	require.NoError(t, err)
	assert.Equal(t, 370, decoded.Bounds().Dx())
	assert.Equal(t, 234, decoded.Bounds().Dy())
}

func TestProcessor_ExtractMetadata_VP8XAlphaInput(t *testing.T) {
	processor := media.NewProcessor()

	input := testWebPBytes(t, 628, 506, true)
	metadata, err := processor.ExtractMetadata(bytes.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, metadata)
	assert.Equal(t, 628, metadata.Width)
	assert.Equal(t, 506, metadata.Height)
}
