package media

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
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

// readHeader reads up to n bytes from the reader, returning what was read. An
// io.EOF may be returned alongside a shorter buffer; callers treat the result
// as a prefix of the stream.
func readHeader(r io.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	read, err := io.ReadFull(r, buf)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	}
	return buf[:read], err
}

// isWebP reports whether the header starts with the RIFF/WEBP magic.
func isWebP(head []byte) bool {
	return len(head) >= 12 &&
		string(head[0:4]) == "RIFF" &&
		string(head[8:12]) == "WEBP"
}

// decodeImage decodes any registered image format. WebP is decoded through
// golang.org/x/image/webp explicitly instead of image.Decode: deepteams/webp
// (used for encoding) also registers a "webp" decoder whose VP8X/ALPH path
// fails on real-world files, and image.Decode picks the winner by init order
// — never leave that to chance.
func decodeImage(r io.Reader) (image.Image, error) {
	head, err := readHeader(r, 12)
	if err != nil {
		return nil, err
	}
	stream := io.MultiReader(bytes.NewReader(head), r)
	if isWebP(head) {
		return ximagewebp.Decode(stream)
	}
	img, _, err := image.Decode(stream)
	return img, err
}

// decodeImageConfig decodes the dimensions/color model of an image without
// full decoding, using the same explicit WebP dispatch as decodeImage.
func decodeImageConfig(r io.Reader) (image.Config, error) {
	head, err := readHeader(r, 12)
	if err != nil {
		return image.Config{}, err
	}
	stream := io.MultiReader(bytes.NewReader(head), r)
	if isWebP(head) {
		return ximagewebp.DecodeConfig(stream)
	}
	config, _, err := image.DecodeConfig(stream)
	return config, err
}

// sanitizeWebP validates an already-sniffed WebP file and strips the metadata
// chunks (EXIF/XMP/ICC) that the previous decode-re-encode pipeline would
// have discarded — including GPS-bearing EXIF that would otherwise be served
// to every visitor. Files without metadata pass through byte-identical.
// Animated containers and frame-less (truncated) containers are rejected with
// clear errors instead of failing later at the thumbnail stage.
func sanitizeWebP(data []byte) ([]byte, error) {
	riffSize := int(binary.LittleEndian.Uint32(data[4:8]))
	dataEnd := 8 + riffSize
	if dataEnd > len(data) {
		return nil, errors.New("webp: RIFF size overruns the file")
	}

	var (
		out      []byte
		dropped  bool
		hasFrame bool
		hasAnim  bool
	)
	out = append(out, data[:12]...)
	offset := 12
	for offset < dataEnd {
		if offset+8 > dataEnd {
			return nil, errors.New("webp: truncated chunk header")
		}
		fcc := data[offset : offset+4]
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		payloadStart := offset + 8
		payloadEnd := payloadStart + size
		if payloadEnd > dataEnd {
			return nil, fmt.Errorf("webp: chunk %q size %d overruns the container", fcc, size)
		}
		paddedEnd := payloadEnd + (size % 2)
		if paddedEnd > dataEnd {
			return nil, fmt.Errorf("webp: chunk %q is missing its padding byte", fcc)
		}

		switch string(fcc) {
		case "VP8 ", "VP8L":
			hasFrame = true
		case "ANIM":
			hasAnim = true
		case "EXIF", "XMP ", "ICCP":
			dropped = true
			offset = paddedEnd
			continue
		}

		out = append(out, data[offset:paddedEnd]...)
		offset = paddedEnd
	}

	if hasAnim {
		return nil, errors.New("animated WebP is not supported")
	}
	if !hasFrame {
		return nil, errors.New("webp: no image frame (truncated or unsupported container)")
	}
	if !dropped {
		return data, nil
	}
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out, nil
}

// ProcessResult contains the result of image processing
type ProcessResult struct {
	WebpData []byte
	Metadata *ImageMetadata
	Hash     string
}

// Processor handles image processing operations
type Processor struct{}

// ConvertToWebP ensures the input is a WebP image: already-WebP input is
// passed through (metadata chunks stripped, still frames validated — no
// re-encode, no generation loss); every other registered format is decoded
// and re-encoded as WebP.
func (p *Processor) ConvertToWebP(reader io.Reader) ([]byte, *ImageMetadata, error) {
	head, err := readHeader(reader, 12)
	if err != nil {
		return nil, nil, err
	}

	if isWebP(head) {
		rest, err := io.ReadAll(reader)
		if err != nil {
			return nil, nil, err
		}
		data := append(head, rest...)
		clean, err := sanitizeWebP(data)
		if err != nil {
			return nil, nil, err
		}
		config, err := ximagewebp.DecodeConfig(bytes.NewReader(clean))
		if err != nil {
			return nil, nil, err
		}
		return clean, &ImageMetadata{
			Width:  config.Width,
			Height: config.Height,
		}, nil
	}

	img, _, err := image.Decode(io.MultiReader(bytes.NewReader(head), reader))
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
	config, err := decodeImageConfig(reader)
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
	img, err := decodeImage(reader)
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
