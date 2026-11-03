package modules_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/modules"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	privacydomain "github.com/aristorinjuang/lesstruct/modules/privacy/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSegmenter is a programmable privacy Segmenter double that records
// every concept vocabulary it is asked to scan.
type stubSegmenter struct {
	detections  []privacydomain.Detection
	err         error
	gotConcepts [][]string
}

func (s *stubSegmenter) Segment(
	_ context.Context,
	_ []byte,
	concepts []string,
) ([]privacydomain.Detection, error) {
	s.gotConcepts = append(s.gotConcepts, append([]string(nil), concepts...))
	if s.err != nil {
		return nil, s.err
	}
	return s.detections, nil
}

// createAdapterPNG builds a 64x64 gradient with the red channel XORed by
// base, so redaction output is observably different from its input.
func createAdapterPNG(t *testing.T, base uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.Set(x, y, color.RGBA{uint8(x*4) ^ base, uint8(y * 4), 200, 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestNewMediaPrivacyRedactorNil(t *testing.T) {
	assert.Nil(t, modules.NewMediaPrivacyRedactor(nil))
	assert.Nil(t, modules.NewProfilePicturePrivacyRedactor(nil))
}

func TestMediaPrivacyRedactorInertNilService(t *testing.T) {
	// A nil service cannot happen via the constructor (it returns nil), but
	// the adapter methods guard it for safety: construct via a service with
	// nil segmenter, which is inert and returns input unchanged.
	svc := privacydomain.NewService(nil, nil)
	redactor := modules.NewMediaPrivacyRedactor(svc)
	require.NotNil(t, redactor)
	raw := []byte("raw")
	got, err := redactor.Redact(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, raw, got)
}

// TestMediaPrivacyRedactorBlursFaces proves the media adapter routes to the
// full-concept scan: faces reach the segmenter and the output bytes change.
func TestMediaPrivacyRedactorBlursFaces(t *testing.T) {
	raw := createAdapterPNG(t, 0)
	stub := &stubSegmenter{
		detections: []privacydomain.Detection{
			privacydomain.NewDetection("face", image.Rect(8, 8, 24, 24)),
		},
	}
	svc := privacydomain.NewService(stub, []string{"face", "license plate", "nudity"})
	redactor := modules.NewMediaPrivacyRedactor(svc)
	require.NotNil(t, redactor)

	got, err := redactor.Redact(context.Background(), raw)
	require.NoError(t, err)
	require.NotEqual(t, raw, got)
	require.Len(t, stub.gotConcepts, 1)
	assert.Contains(t, stub.gotConcepts[0], "face")
}

// TestProfilePrivacyRedactorFiltersFaces proves the profile-picture adapter
// routes to the face-exempt call: faces never reach the segmenter while
// plates still blur.
func TestProfilePrivacyRedactorFiltersFaces(t *testing.T) {
	raw := createAdapterPNG(t, 0)
	stub := &stubSegmenter{
		detections: []privacydomain.Detection{
			privacydomain.NewDetection("license plate", image.Rect(8, 8, 24, 24)),
		},
	}
	svc := privacydomain.NewService(stub, []string{"face", "license plate", "nudity"})
	redactor := modules.NewProfilePicturePrivacyRedactor(svc)
	require.NotNil(t, redactor)

	got, err := redactor.Redact(context.Background(), raw)
	require.NoError(t, err)
	require.NotEqual(t, raw, got)
	require.Len(t, stub.gotConcepts, 1)
	assert.NotContains(t, stub.gotConcepts[0], "face")
	assert.Contains(t, stub.gotConcepts[0], "license plate")
}

func TestPrivacyAdaptersFailClosed(t *testing.T) {
	tests := []struct {
		name     string
		concepts []string
		build    func(svc *privacydomain.Service) interface {
			Redact(context.Context, []byte) ([]byte, error)
		}
		wantErr bool
	}{
		{
			name:     "media maps scan failure",
			concepts: []string{"face"},
			build: func(svc *privacydomain.Service) interface {
				Redact(context.Context, []byte) ([]byte, error)
			} {
				return modules.NewMediaPrivacyRedactor(svc)
			},
			wantErr: true,
		},
		{
			name:     "profile maps scan failure",
			concepts: []string{"face", "nudity"},
			build: func(svc *privacydomain.Service) interface {
				Redact(context.Context, []byte) ([]byte, error)
			} {
				return modules.NewProfilePicturePrivacyRedactor(svc)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := privacydomain.NewService(&stubSegmenter{err: errors.New("boom")}, tt.concepts)
			r := tt.build(svc)
			_, err := r.Redact(context.Background(), []byte("img"))
			require.Error(t, err)
			assert.ErrorIs(t, err, mediadomain.ErrPrivacyScanFailed)
		})
	}
}
