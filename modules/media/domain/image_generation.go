package media

import (
	"context"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"
)

// OpenGraphPromptSuffix guides the model toward a wide social-preview
// composition when generating an Open Graph image.
const OpenGraphPromptSuffix = "Design this as a wide Open Graph social preview image (1200x630, landscape). Keep the key subject centered with generous margins on all sides, as the edges may be cropped."

// ImageReference is a single reference image guiding AI image generation. Data holds
// the raw image bytes and MimeType is the detected image MIME type.
type ImageReference struct {
	Data     []byte
	MimeType string
}

// ImageGenerationService defines the interface for AI image generation services.
type ImageGenerationService interface {
	GenerateImage(
		ctx context.Context,
		prompt string,
		references []ImageReference,
	) ([]byte, error)
	SupportsImageReferences() bool
}

// BuildGenerationPrompt returns the provider prompt, with Open Graph
// composition guidance appended when og is true.
func BuildGenerationPrompt(prompt string, og bool) string {
	if !og {
		return prompt
	}
	return prompt + " " + OpenGraphPromptSuffix
}

// AltTextFromPrompt derives alt text for a generated image from the generation
// prompt. Alt text caps at MaxAltTextLength runes, so over-long prompts are
// cut at the last whitespace boundary inside the cap — or hard-cut when a
// single token exceeds it.
func AltTextFromPrompt(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if utf8.RuneCountInString(trimmed) <= MaxAltTextLength {
		return trimmed
	}
	cut := []rune(trimmed)[:MaxAltTextLength]
	boundary := -1
	for i := len(cut) - 1; i >= 0; i-- {
		if unicode.IsSpace(cut[i]) {
			boundary = i
			break
		}
	}
	if boundary > 0 {
		cut = cut[:boundary]
	}
	return strings.TrimSpace(string(cut))
}

// NewImageReference validates raw bytes as an AI image-generation reference and
// returns the reference with its detected MIME type. Only JPEG, PNG, and WebP
// are accepted: GIF is rejected because the reference-capable providers
// (Gemini image models, GPT Image edits) do not support it.
func NewImageReference(data []byte) (ImageReference, error) {
	if len(data) == 0 {
		return ImageReference{}, ErrInvalidFileContent
	}
	if int64(len(data)) > MaxFileSize {
		return ImageReference{}, ErrFileTooLarge
	}
	if err := ValidateFileSignature(data); err != nil {
		return ImageReference{}, err
	}
	mimeType := DetectImageMimeType(data)
	if mimeType == string(MimeTypeGIF) {
		return ImageReference{}, ErrInvalidFileContent
	}
	return ImageReference{
		Data:     data,
		MimeType: mimeType,
	}, nil
}

// DetectImageMimeType detects the MIME type of image bytes. The bytes must have
// passed ValidateFileSignature first; anything DetectContentType cannot identify
// at that point is WebP, which it does not detect.
func DetectImageMimeType(data []byte) string {
	if detected := http.DetectContentType(data); SupportedMimeTypes[detected] {
		return detected
	}
	return string(MimeTypeWebP)
}
