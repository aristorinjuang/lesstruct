package media_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"

	media "github.com/aristorinjuang/lesstruct/modules/media/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zeroDimMagic identifies the synthetic format whose DecodeConfig reports
// zero dimensions, exercising the pixel-budget guard's pass-through branch.
const zeroDimMagic = "ZDIMTEST"

// readerOnly hides the wrapped reader's Seek method, forcing decodeImage's
// non-seeker buffering path.
type readerOnly struct {
	io.Reader
}

// failingSeekReader is a valid read-seeker whose rewinds always fail.
type failingSeekReader struct {
	*bytes.Reader
}

func (f failingSeekReader) Seek(int64, int) (int64, error) {
	return 0, errors.New("seek error")
}

// zeroDimImage is a one-pixel image returned by the synthetic format.
type zeroDimImage struct{}

func (zeroDimImage) ColorModel() color.Model { return color.RGBAModel }

func (zeroDimImage) Bounds() image.Rectangle { return image.Rect(0, 0, 1, 1) }

func (zeroDimImage) At(int, int) color.Color { return color.RGBA{} }

func init() {
	image.RegisterFormat(
		"lesstruct-zero-dim-test",
		zeroDimMagic,
		func(io.Reader) (image.Image, error) { return zeroDimImage{}, nil },
		func(io.Reader) (image.Config, error) { return image.Config{}, nil },
	)
}

// testPNGBytes encodes a solid PNG of the given dimensions.
func testPNGBytes(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{30, 120, 200, 255})
		}
	}

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestProcessor_Resize_NonSeekerReader(t *testing.T) {
	tests := []struct {
		name           string
		width          int
		height         int
		maxWidth       int
		expectedWidth  int
		expectedHeight int
	}{
		{
			name:           "buffers a reader without Seek",
			width:          200,
			height:         100,
			maxWidth:       370,
			expectedWidth:  200,
			expectedHeight: 100,
		},
		{
			name:           "downscales a reader without Seek",
			width:          800,
			height:         600,
			maxWidth:       370,
			expectedWidth:  370,
			expectedHeight: 277,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := media.NewProcessor()
			reader := readerOnly{Reader: bytes.NewReader(testPNGBytes(t, tt.width, tt.height))}

			out, metadata, err := processor.Resize(reader, tt.maxWidth)

			require.NoError(t, err)
			assert.NotEmpty(t, out)
			assert.Equal(t, tt.expectedWidth, metadata.Width)
			assert.Equal(t, tt.expectedHeight, metadata.Height)
		})
	}
}

func TestProcessor_Resize_SeekFailure(t *testing.T) {
	tests := []struct {
		name            string
		call            func(*media.Processor) error
		wantErrContains string
	}{
		{
			name: "resize surfaces a failed rewind",
			call: func(p *media.Processor) error {
				_, _, err := p.Resize(failingSeekReader{Reader: bytes.NewReader(testPNGBytes(t, 100, 100))}, 370)
				return err
			},
			wantErrContains: "failed to rewind image after dimension probe",
		},
		{
			name: "convert surfaces a failed rewind",
			call: func(p *media.Processor) error {
				_, _, err := p.ConvertToWebP(failingSeekReader{Reader: bytes.NewReader(testPNGBytes(t, 100, 100))})
				return err
			},
			wantErrContains: "seek error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call(media.NewProcessor())

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrContains)
		})
	}
}

func TestProcessor_Resize_NonSeekerReaderFailures(t *testing.T) {
	tests := []struct {
		name            string
		reader          func(*testing.T) io.Reader
		wantErrContains string
	}{
		{
			name: "surfaces a read error after the header",
			reader: func(*testing.T) io.Reader {
				return &headerThenErrorReader{}
			},
			wantErrContains: "read error",
		},
		{
			name: "still enforces the pixel budget on buffered readers",
			reader: func(t *testing.T) io.Reader {
				return readerOnly{Reader: bytes.NewReader(hugeDimsPNG(t, 100_000, 100_000))}
			},
			wantErrContains: "image dimensions exceed the supported maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := media.NewProcessor().Resize(tt.reader(t), 370)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrContains)
		})
	}
}

// headerThenErrorReader serves a PNG signature once, then fails every read.
type headerThenErrorReader struct {
	served bool
}

func (r *headerThenErrorReader) Read(p []byte) (int, error) {
	if !r.served {
		r.served = true
		return copy(p, []byte{
			0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
			0x00, 0x00, 0x00, 0x0D,
		}), nil
	}
	return 0, errors.New("read error")
}

func TestProcessor_Resize_ZeroDimensionConfig(t *testing.T) {
	tests := []struct {
		name           string
		data           []byte
		expectedWidth  int
		expectedHeight int
	}{
		{
			name:           "zero dimensions pass the budget guard",
			data:           append([]byte(zeroDimMagic), make([]byte, 8)...),
			expectedWidth:  1,
			expectedHeight: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := media.NewProcessor()

			out, metadata, err := processor.Resize(bytes.NewReader(tt.data), 370)

			require.NoError(t, err)
			assert.NotEmpty(t, out)
			assert.Equal(t, tt.expectedWidth, metadata.Width)
			assert.Equal(t, tt.expectedHeight, metadata.Height)
		})
	}
}
