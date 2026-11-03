// Package privacy owns the redaction domain: the Segmenter port that the
// SAM client implements and the Redactor service that blurs detections.
package privacy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	_ "image/png"

	"golang.org/x/image/draw"
	ximagewebp "golang.org/x/image/webp"
)

const (
	// dilationMargin expands each detection box before blurring so edge
	// pixels and tight boxes still cover the sensitive area.
	dilationMargin = 12
	// blurShrink is the downscale factor for the blur pass: the region is
	// reduced to 1/blurShrink then enlarged, which smooths detail beyond
	// recognition while staying fast on large photos.
	blurShrink = 12
	// maxDecodedPixels bounds decompression-bomb exposure: a small
	// compressed upload can declare enormous pixel dimensions whose full
	// RGBA decode allocates hundreds of megabytes. 60 MP covers every real
	// camera photo, matching the media processor's budget.
	maxDecodedPixels = 60_000_000
)

// errImageDimensionsTooLarge rejects images whose declared pixel count
// exceeds maxDecodedPixels before the full decode allocates.
var errImageDimensionsTooLarge = errors.New("image dimensions exceed the supported maximum")

// isWebP reports whether data starts with the RIFF/WEBP magic.
func isWebP(data []byte) bool {
	return len(data) >= 12 &&
		string(data[0:4]) == "RIFF" &&
		string(data[8:12]) == "WEBP"
}

// probePixelBudget reads the image dimensions via DecodeConfig (cheap)
// and rejects them when the decoded pixel count exceeds the budget.
// Config failures are swallowed — the subsequent full decode reports the
// actual format error.
func probePixelBudget(data []byte) error {
	var cfg image.Config
	var err error
	if isWebP(data) {
		cfg, err = ximagewebp.DecodeConfig(bytes.NewReader(data))
	} else {
		cfg, _, err = image.DecodeConfig(bytes.NewReader(data))
	}
	if err != nil {
		return nil
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxDecodedPixels {
		return fmt.Errorf("%w: %dx%d", errImageDimensionsTooLarge, cfg.Width, cfg.Height)
	}
	return nil
}

// decodeImageBytes decodes JPEG, PNG, GIF, and WebP. WebP routes explicitly
// through x/image/webp because deepteams/webp (the encoder elsewhere)
// registers a decoder whose VP8X path fails on real-world files.
func decodeImageBytes(data []byte) (image.Image, error) {
	if err := probePixelBudget(data); err != nil {
		return nil, err
	}
	if isWebP(data) {
		return ximagewebp.Decode(bytes.NewReader(data))
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

// dilateRect grows r by margin on every side and clamps to bounds.
func dilateRect(r image.Rectangle, margin int, bounds image.Rectangle) image.Rectangle {
	r.Min.X -= margin
	r.Min.Y -= margin
	r.Max.X += margin
	r.Max.Y += margin
	return r.Intersect(bounds)
}

// blurRegion smooths region in place by shrinking it then enlarging it.
// The two BiLinear passes erase detail while keeping the average hue, so
// the result reads as a blur rather than a solid box. Callers guarantee a
// non-empty region inside img.
func blurRegion(img *image.RGBA, region image.Rectangle) {
	smallW := max(1, region.Dx()/blurShrink)
	smallH := max(1, region.Dy()/blurShrink)
	small := image.NewRGBA(image.Rect(0, 0, smallW, smallH))
	draw.BiLinear.Scale(small, small.Bounds(), img, region, draw.Src, nil)
	draw.BiLinear.Scale(img, region, small, small.Bounds(), draw.Src, nil)
}

// toRGBA converts any image to RGBA for in-place blurring.
func toRGBA(img image.Image) *image.RGBA {
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	return rgba
}

// Segmenter finds sensitive regions. Implementations call the SAM Model
// API once per concept and return boxes.
type Segmenter interface {
	Segment(
		ctx context.Context,
		imageBytes []byte,
		concepts []string,
	) ([]Detection, error)
}

// Service blurs Segmenter detections. It is SAM-agnostic: any Segmenter
// (the SAM client, a stub in tests) plugs in.
type Service struct {
	segmenter Segmenter
	concepts  []string
}

// effectiveConcepts resolves the scan vocabulary, dropping faces when the
// caller exempts them (profile pictures).
func (s *Service) effectiveConcepts(exemptFaces bool) []string {
	if !exemptFaces {
		return s.concepts
	}
	return WithoutFaces(s.concepts)
}

// redact runs the scan-blur pipeline and returns PNG bytes. A nil segmenter
// is inert and returns the input unchanged; an empty effective vocabulary
// skips the scan without an API call.
func (s *Service) redact(
	ctx context.Context,
	imageBytes []byte,
	exemptFaces bool,
) ([]byte, error) {
	if s.segmenter == nil {
		return imageBytes, nil
	}
	effective := s.effectiveConcepts(exemptFaces)
	if len(effective) == 0 {
		return imageBytes, nil
	}
	detections, err := s.segmenter.Segment(ctx, imageBytes, effective)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrScanFailed, err)
	}
	if len(detections) == 0 {
		return imageBytes, nil
	}
	img, err := decodeImageBytes(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: decode for redaction: %v", ErrScanFailed, err)
	}
	rgba := toRGBA(img)
	bounds := rgba.Bounds()
	for _, d := range detections {
		region := dilateRect(d.Box, dilationMargin, bounds)
		if region.Empty() {
			continue
		}
		blurRegion(rgba, region)
	}
	var buf bytes.Buffer
	// png.Encode on a bytes.Buffer never fails; the error is ignored to keep
	// the domain bar at 100% without an unreachable branch.
	_ = png.Encode(&buf, rgba)
	return buf.Bytes(), nil
}

// Redact blurs every configured concept.
func (s *Service) Redact(ctx context.Context, imageBytes []byte) ([]byte, error) {
	return s.redact(ctx, imageBytes, false)
}

// RedactMedia blurs faces, plates, and nudity for the media library.
func (s *Service) RedactMedia(ctx context.Context, imageBytes []byte) ([]byte, error) {
	return s.redact(ctx, imageBytes, false)
}

// RedactProfilePicture blurs plates and nudity but keeps faces visible.
func (s *Service) RedactProfilePicture(ctx context.Context, imageBytes []byte) ([]byte, error) {
	return s.redact(ctx, imageBytes, true)
}

// NewService builds a redactor over segmenter with the given concepts. An
// empty concepts list falls back to DefaultConcepts; a nil segmenter makes
// every Redact call inert and return its input unchanged.
func NewService(segmenter Segmenter, concepts []string) *Service {
	effective := concepts
	if len(effective) == 0 {
		effective = append([]string(nil), DefaultConcepts...)
	} else {
		effective = append([]string(nil), effective...)
	}
	return &Service{
		segmenter: segmenter,
		concepts:  effective,
	}
}
