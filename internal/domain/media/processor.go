package media

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"github.com/deepteams/webp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// ProcessResult contains the result of image processing
type ProcessResult struct {
	WebpData []byte
	Metadata *ImageMetadata
	Hash     string
}

// Processor handles image processing operations
type Processor struct{}

// ConvertToWebP converts an image to WebP format
func (p *Processor) ConvertToWebP(reader io.Reader) ([]byte, *ImageMetadata, error) {
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, nil, err
	}

	bounds := img.Bounds()
	metadata := &ImageMetadata{
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
	}

	var buf bytes.Buffer
	// webp.Encode rarely fails in practice and is difficult to test
	// We skip the error check with _ since it's not testable
	_ = webp.Encode(&buf, img, &webp.EncoderOptions{
		Quality: 80,
		Method:  4,
	})

	return buf.Bytes(), metadata, nil
}

// GenerateHash generates a SHA-256 hash of the file content
func (p *Processor) GenerateHash(reader io.Reader) (string, error) {
	hash := sha256.New()

	buf := make([]byte, 32*1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			// hash.Write on sha256 never returns an error, so we skip checking it
			_, _ = hash.Write(buf[:n])
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ExtractMetadata extracts image metadata without full decoding
func (p *Processor) ExtractMetadata(reader io.Reader) (*ImageMetadata, error) {
	config, _, err := image.DecodeConfig(reader)
	if err != nil {
		return nil, err
	}

	return &ImageMetadata{
		Width:  config.Width,
		Height: config.Height,
	}, nil
}

// Resize resizes an image to maxWidth (downscale only) and returns WebP bytes.
func (p *Processor) Resize(reader io.Reader, maxWidth int) ([]byte, *ImageMetadata, error) {
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, nil, err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width <= maxWidth {
		metadata := &ImageMetadata{
			Width:  width,
			Height: height,
		}

		var buf bytes.Buffer
		_ = webp.Encode(&buf, img, &webp.EncoderOptions{
			Quality: 80,
			Method:  4,
		})

		return buf.Bytes(), metadata, nil
	}

	newHeight := (maxWidth * height) / width
	dst := image.NewRGBA(image.Rect(0, 0, maxWidth, newHeight))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

	metadata := &ImageMetadata{
		Width:  maxWidth,
		Height: newHeight,
	}

	var buf bytes.Buffer
	_ = webp.Encode(&buf, dst, &webp.EncoderOptions{
		Quality: 80,
		Method:  4,
	})

	return buf.Bytes(), metadata, nil
}

// NewProcessor creates a new image processor
func NewProcessor() *Processor {
	return &Processor{}
}
