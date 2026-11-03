package privacy_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	privacy "github.com/aristorinjuang/lesstruct/modules/privacy/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSegmenter is a programmable Segmenter double.
type stubSegmenter struct {
	detections  []privacy.Detection
	err         error
	calls       int
	gotConcepts [][]string
}

func (s *stubSegmenter) Segment(
	_ context.Context,
	_ []byte,
	concepts []string,
) ([]privacy.Detection, error) {
	s.calls++
	s.gotConcepts = append(s.gotConcepts, append([]string(nil), concepts...))
	if s.err != nil {
		return nil, s.err
	}
	return s.detections, nil
}

func createPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	return img
}

func TestNewServiceDefaults(t *testing.T) {
	svc := privacy.NewService(nil, nil)
	require.NotNil(t, svc)
	// Inert: nil segmenter returns input unchanged.
	out, err := svc.Redact(context.Background(), []byte("raw"))
	require.NoError(t, err)
	assert.Equal(t, []byte("raw"), out)
}

func TestNewServiceCopiesConcepts(t *testing.T) {
	input := []string{"face", "nudity"}
	svc := privacy.NewService(&stubSegmenter{}, input)
	input[0] = "mutated"
	out, err := svc.Redact(context.Background(), createPNG(t, 8, 8))
	require.NoError(t, err)
	require.NotNil(t, out)
}

func TestRedactNilSegmenterInert(t *testing.T) {
	tests := []struct {
		name string
		call func(svc *privacy.Service, ctx context.Context, data []byte) ([]byte, error)
	}{
		{
			name: "Redact",
			call: func(svc *privacy.Service, ctx context.Context, data []byte) ([]byte, error) {
				return svc.Redact(ctx, data)
			},
		},
		{
			name: "RedactMedia",
			call: func(svc *privacy.Service, ctx context.Context, data []byte) ([]byte, error) {
				return svc.RedactMedia(ctx, data)
			},
		},
		{
			name: "RedactProfilePicture",
			call: func(svc *privacy.Service, ctx context.Context, data []byte) ([]byte, error) {
				return svc.RedactProfilePicture(ctx, data)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := privacy.NewService(nil, []string{"face"})
			raw := []byte("unchanged")
			got, err := tt.call(svc, context.Background(), raw)
			require.NoError(t, err)
			assert.Equal(t, raw, got)
		})
	}
}

func TestRedactEmptyEffectiveSkipsScan(t *testing.T) {
	stub := &stubSegmenter{}
	svc := privacy.NewService(stub, []string{"face"})
	raw := createPNG(t, 16, 16)
	got, err := svc.RedactProfilePicture(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, raw, got)
	assert.Equal(t, 0, stub.calls)
}

func TestRedactSegmentError(t *testing.T) {
	tests := []struct {
		name     string
		concepts []string
		call     func(svc *privacy.Service, ctx context.Context, data []byte) ([]byte, error)
		wantErr  bool
	}{
		{
			name:     "Redact wraps scan failure",
			concepts: []string{"face"},
			call: func(svc *privacy.Service, ctx context.Context, data []byte) ([]byte, error) {
				return svc.Redact(ctx, data)
			},
			wantErr: true,
		},
		{
			name:     "RedactMedia wraps scan failure",
			concepts: []string{"face"},
			call: func(svc *privacy.Service, ctx context.Context, data []byte) ([]byte, error) {
				return svc.RedactMedia(ctx, data)
			},
			wantErr: true,
		},
		{
			name:     "RedactProfilePicture wraps scan failure",
			concepts: []string{"face", "nudity"},
			call: func(svc *privacy.Service, ctx context.Context, data []byte) ([]byte, error) {
				return svc.RedactProfilePicture(ctx, data)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubSegmenter{err: errors.New("boom")}
			svc := privacy.NewService(stub, tt.concepts)
			_, err := tt.call(svc, context.Background(), createPNG(t, 16, 16))
			require.Error(t, err)
			assert.ErrorIs(t, err, privacy.ErrScanFailed)
		})
	}
}

func TestRedactNoDetections(t *testing.T) {
	stub := &stubSegmenter{detections: nil}
	svc := privacy.NewService(stub, []string{"face"})
	raw := createPNG(t, 16, 16)
	got, err := svc.Redact(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, raw, got)
	assert.Equal(t, 1, stub.calls)
}

func TestRedactDecodeError(t *testing.T) {
	stub := &stubSegmenter{
		detections: []privacy.Detection{
			privacy.NewDetection("face", image.Rect(0, 0, 10, 10)),
		},
	}
	svc := privacy.NewService(stub, []string{"face"})
	_, err := svc.Redact(context.Background(), []byte("not an image"))
	require.Error(t, err)
	assert.ErrorIs(t, err, privacy.ErrScanFailed)
}

func TestRedactSuccessBlurs(t *testing.T) {
	raw := createPNG(t, 64, 64)
	before := decodePNG(t, raw)
	stub := &stubSegmenter{
		detections: []privacy.Detection{
			privacy.NewDetection("face", image.Rect(24, 24, 40, 40)),
		},
	}
	svc := privacy.NewService(stub, []string{"face"})
	got, err := svc.Redact(context.Background(), raw)
	require.NoError(t, err)
	require.NotEqual(t, raw, got)
	after, err := png.Decode(bytes.NewReader(got))
	require.NoError(t, err)
	assert.Equal(t, before.Bounds(), after.Bounds())
}

func TestRedactSkipsEmptyRegion(t *testing.T) {
	raw := createPNG(t, 32, 32)
	stub := &stubSegmenter{
		detections: []privacy.Detection{
			privacy.NewDetection("face", image.Rect(-1000, -1000, -900, -900)),
			privacy.NewDetection("nudity", image.Rect(8, 8, 16, 16)),
		},
	}
	svc := privacy.NewService(stub, []string{"face", "nudity"})
	got, err := svc.Redact(context.Background(), raw)
	require.NoError(t, err)
	require.NotEqual(t, raw, got)
}

func TestRedactProfilePictureFiltersFaces(t *testing.T) {
	raw := createPNG(t, 32, 32)
	stub := &stubSegmenter{}
	svc := privacy.NewService(stub, []string{"face", "license plate", "nudity"})
	_, err := svc.RedactProfilePicture(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, 1, stub.calls)
	require.Len(t, stub.gotConcepts, 1)
	assert.NotContains(t, stub.gotConcepts[0], "face")
	assert.Contains(t, stub.gotConcepts[0], "license plate")
	assert.Contains(t, stub.gotConcepts[0], "nudity")
}

func TestRedactWebPBranch(t *testing.T) {
	// RIFF....WEBP magic routes through the WebP decoder; garbage after the
	// header still exercises the isWebP true branch via a decode failure.
	stub := &stubSegmenter{
		detections: []privacy.Detection{
			privacy.NewDetection("face", image.Rect(0, 0, 8, 8)),
		},
	}
	svc := privacy.NewService(stub, []string{"face"})
	fake := append([]byte("RIFFxxxxWEBP"), bytes.Repeat([]byte{0}, 64)...)
	_, err := svc.Redact(context.Background(), fake)
	require.Error(t, err)
	assert.ErrorIs(t, err, privacy.ErrScanFailed)
}

func TestRedactRejectsOverBudgetDimensions(t *testing.T) {
	// GIF declares dimensions in two bytes each: 65535x65535 exceeds the
	// 60MP budget while the payload stays 13 bytes, so only the
	// DecodeConfig probe can reject it before the full decode allocates.
	stub := &stubSegmenter{
		detections: []privacy.Detection{
			privacy.NewDetection("face", image.Rect(0, 0, 8, 8)),
		},
	}
	svc := privacy.NewService(stub, []string{"face"})
	fake := []byte("GIF89a\xff\xff\xff\xff\x00\x00\x00")
	_, err := svc.Redact(context.Background(), fake)
	require.Error(t, err)
	assert.ErrorIs(t, err, privacy.ErrScanFailed)
}

func TestRedactZeroDimensionsDecodeError(t *testing.T) {
	// A zero-dimension header passes the pixel probe (nothing to budget)
	// and fails in the full decode, still wrapped fail-closed.
	stub := &stubSegmenter{
		detections: []privacy.Detection{
			privacy.NewDetection("face", image.Rect(0, 0, 8, 8)),
		},
	}
	svc := privacy.NewService(stub, []string{"face"})
	fake := []byte("GIF89a\x00\x00\x00\x00\x00\x00\x00")
	_, err := svc.Redact(context.Background(), fake)
	require.Error(t, err)
	assert.ErrorIs(t, err, privacy.ErrScanFailed)
}
