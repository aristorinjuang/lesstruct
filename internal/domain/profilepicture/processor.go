package profilepicture

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"github.com/deepteams/webp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Processor handles profile picture image processing operations
type Processor struct{}

// CropAndConvertToWebP decodes an image, center-crops to a square,
// resizes to the target size, and encodes as WebP.
func (p *Processor) CropAndConvertToWebP(reader io.Reader, size int) ([]byte, error) {
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// Center-crop to square
	cropSize := min(w, h)
	x0 := (w - cropSize) / 2
	y0 := (h - cropSize) / 2
	cropped := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(x0, y0, x0+cropSize, y0+cropSize))

	// Resize to target size
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.BiLinear.Scale(dst, dst.Bounds(), cropped, cropped.Bounds(), draw.Over, nil)

	// Encode as WebP
	var buf bytes.Buffer
	if err := webp.Encode(&buf, dst, &webp.EncoderOptions{
		Quality: 80,
		Method:  4,
	}); err != nil {
		return nil, fmt.Errorf("failed to encode webp: %w", err)
	}

	return buf.Bytes(), nil
}

// NewProcessor creates a new profile picture processor
func NewProcessor() *Processor {
	return &Processor{}
}
