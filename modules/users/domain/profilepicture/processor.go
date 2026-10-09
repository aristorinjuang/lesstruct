package profilepicture

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"github.com/deepteams/webp"
	"golang.org/x/image/draw"
	ximagewebp "golang.org/x/image/webp"
)

// decodeImage decodes any registered image format, routing WebP through
// golang.org/x/image/webp explicitly (deepteams/webp also registers a webp
// decoder whose VP8X/ALPH path fails on real-world files; image.Decode would
// leave the winner to registration order).
// maxDecodedPixels bounds decompression-bomb exposure: a small compressed
// upload can declare enormous pixel dimensions whose full RGBA decode
// allocates hundreds of megabytes. 60 MP covers every real camera photo.
const maxDecodedPixels = 60_000_000

// errImageDimensionsTooLarge rejects images whose declared pixel count
// exceeds maxDecodedPixels before the full decode allocates.
var errImageDimensionsTooLarge = errors.New("image dimensions exceed the supported maximum")

func decodeImage(r io.Reader) (image.Image, error) {
	head := make([]byte, 12)
	n, err := io.ReadFull(r, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	head = head[:n]

	// Decompression-bomb defense: probe dimensions via DecodeConfig before
	// the full decode allocates width×height memory. The probe consumes
	// reader bytes; rewind to replay them for the decode. Callers pass
	// io.ReadSeeker (multipart sections, byte buffers).
	if err := probePixelBudget(head, r); err != nil {
		return nil, err
	}
	if seeker, ok := r.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
	}

	stream := r
	if n >= 12 && string(head[0:4]) == "RIFF" && string(head[8:12]) == "WEBP" {
		return ximagewebp.Decode(stream)
	}
	img, _, err := image.Decode(stream)
	return img, err
}

// probePixelBudget reads the image dimensions from head+remainder and
// rejects them when the decoded pixel count exceeds the budget. Config
// failures are swallowed — the subsequent full decode reports the actual
// format error.
func probePixelBudget(head []byte, r io.Reader) error {
	stream := io.MultiReader(bytes.NewReader(head), r)
	cfg, _, err := image.DecodeConfig(stream)
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

// Processor handles profile picture image processing operations
type Processor struct{}

// CropAndConvertToWebP decodes an image, center-crops to a square,
// resizes to the target size, and encodes as WebP.
func (p *Processor) CropAndConvertToWebP(reader io.Reader, size int) ([]byte, error) {
	img, err := decodeImage(reader)
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
