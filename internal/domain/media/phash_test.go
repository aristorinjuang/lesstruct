package media_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/deepteams/webp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/draw"
)

// splitImage builds a w×h image that is black except for one white half —
// right half when horizontal, bottom half when vertical — so scaled-down
// hashes differ strongly between the two orientations.
func splitImage(vertical bool, w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			white := x >= w/2
			if vertical {
				white = y >= h/2
			}
			if white {
				img.Set(x, y, color.White)
			} else {
				img.Set(x, y, color.Black)
			}
		}
	}
	return img
}

// gradientImage builds a w×h smooth diagonal luma ramp.
func gradientImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := uint8((x + y) * 255 / (w + h))
			img.Set(x, y, color.RGBA{R: v, G: v, B: v, A: 255})
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

func encodeWebP(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, webp.Encode(&buf, img, &webp.EncoderOptions{Quality: 80, Method: 4}))
	return buf.Bytes()
}

func TestPerceptualHash(t *testing.T) {
	tests := []struct {
		name    string
		input   func(t *testing.T) io.Reader
		wantErr bool
	}{
		{
			name: "success - png gradient",
			input: func(t *testing.T) io.Reader {
				return bytes.NewReader(encodePNG(t, gradientImage(64, 64)))
			},
			wantErr: false,
		},
		{
			name: "success - webp input",
			input: func(t *testing.T) io.Reader {
				return bytes.NewReader(encodeWebP(t, gradientImage(64, 64)))
			},
			wantErr: false,
		},
		{
			name: "success - jpeg input",
			input: func(t *testing.T) io.Reader {
				return bytes.NewReader(encodeJPEG(t, gradientImage(64, 64)))
			},
			wantErr: false,
		},
		{
			name: "error - not an image",
			input: func(*testing.T) io.Reader {
				return bytes.NewReader([]byte("definitely not an image"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := media.PerceptualHash(tt.input(t))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotZero(t, hash)

			again, err := media.PerceptualHash(tt.input(t))
			require.NoError(t, err)
			assert.Equal(t, hash, again, "hash must be deterministic")
		})
	}
}

func TestPerceptualHash_SimilarVariants(t *testing.T) {
	tests := []struct {
		name  string
		varnt func(t *testing.T) io.Reader
	}{
		{
			name: "identical bytes",
			varnt: func(t *testing.T) io.Reader {
				return bytes.NewReader(encodePNG(t, gradientImage(128, 128)))
			},
		},
		{
			name: "jpeg re-encode of the same pixels",
			varnt: func(t *testing.T) io.Reader {
				return bytes.NewReader(encodeJPEG(t, gradientImage(128, 128)))
			},
		},
		{
			name: "downscaled webp re-encode",
			varnt: func(t *testing.T) io.Reader {
				src := gradientImage(128, 128)
				dst := image.NewRGBA(image.Rect(0, 0, 48, 48))
				draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
				return bytes.NewReader(encodeWebP(t, dst))
			},
		},
	}

	base := encodePNG(t, gradientImage(128, 128))
	baseHash, err := media.PerceptualHash(bytes.NewReader(base))
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := media.PerceptualHash(tt.varnt(t))
			require.NoError(t, err)
			assert.True(t, media.PerceptuallySimilar(baseHash, hash),
				"expected variant to be perceptually similar (distance %d)", media.HammingDistance(baseHash, hash))
		})
	}
}

func TestPerceptualHash_DistinctImages(t *testing.T) {
	horizontal, err := media.PerceptualHash(bytes.NewReader(encodePNG(t, splitImage(false, 128, 128))))
	require.NoError(t, err)

	vertical, err := media.PerceptualHash(bytes.NewReader(encodePNG(t, splitImage(true, 128, 128))))
	require.NoError(t, err)

	distance := media.HammingDistance(horizontal, vertical)
	assert.Greater(t, distance, media.MaxPerceptualDistance)
	assert.False(t, media.PerceptuallySimilar(horizontal, vertical))
}

func TestHammingDistance(t *testing.T) {
	tests := []struct {
		name     string
		a        uint64
		b        uint64
		expected int
	}{
		{name: "identical hashes", a: 0xDEADBEEF, b: 0xDEADBEEF, expected: 0},
		{name: "all bits differ", a: 0, b: ^uint64(0), expected: 64},
		{name: "half the bits differ", a: 0b1010, b: 0b0101, expected: 4},
		{name: "single bit differs", a: 1 << 3, b: 0, expected: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, media.HammingDistance(tt.a, tt.b))
		})
	}
}

func TestPerceptuallySimilar(t *testing.T) {
	tests := []struct {
		name     string
		a        uint64
		b        uint64
		expected bool
	}{
		{
			name:     "distance zero",
			a:        42,
			b:        42,
			expected: true,
		},
		{
			name:     "distance exactly at threshold",
			a:        0,
			b:        (1 << media.MaxPerceptualDistance) - 1,
			expected: true,
		},
		{
			name:     "distance just past threshold",
			a:        0,
			b:        (1 << (media.MaxPerceptualDistance + 1)) - 1,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, media.PerceptuallySimilar(tt.a, tt.b))
		})
	}
}
