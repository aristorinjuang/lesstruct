package media

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// decodeGPTImageResponse base64-decodes the first image of an image API response.
func decodeGPTImageResponse(response *openai.ImagesResponse) ([]byte, error) {
	if len(response.Data) == 0 {
		return nil, fmt.Errorf("no images generated")
	}

	imageBytes, err := base64.StdEncoding.DecodeString(response.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 image: %w", err)
	}

	return imageBytes, nil
}

// GPTImageService implements ImageGenerationService using OpenAI GPT Image models.
type GPTImageService struct {
	client *openai.Client // singleton client
	apiKey string
	model  string
	size   string
}

// editImage generates a single image guided by reference images using OpenAI's
// image edits endpoint.
func (s *GPTImageService) editImage(
	ctx context.Context,
	prompt string,
	references []ImageReference,
) ([]byte, error) {
	params := openai.ImageEditParams{
		Model:  s.model,
		Prompt: prompt,
		Image:  openai.ImageEditParamsImageUnion{OfFileArray: buildImageEditFiles(references)},
		N:      openai.Int(1),
		Size:   openai.ImageEditParamsSize(s.size),
	}

	response, err := s.client.Images.Edit(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to edit image: %w", err)
	}

	return decodeGPTImageResponse(response)
}

// buildImageEditFiles wraps reference images as named files for the edits
// endpoint. The API rejects anonymous parts (sent as application/octet-stream),
// so each part carries a filename and the detected MIME type.
func buildImageEditFiles(references []ImageReference) []io.Reader {
	images := make([]io.Reader, 0, len(references))
	for i, ref := range references {
		images = append(images, openai.File(bytes.NewReader(ref.Data), referenceFilename(ref.MimeType, i), ref.MimeType))
	}
	return images
}

// referenceFilename derives a synthetic filename from the detected MIME type so
// the edits endpoint can infer the file format.
func referenceFilename(mimeType string, index int) string {
	ext := "jpg"
	switch mimeType {
	case string(MimeTypePNG):
		ext = "png"
	case string(MimeTypeWebP):
		ext = "webp"
	}
	return fmt.Sprintf("reference-%d.%s", index+1, ext)
}

// SupportsImageReferences reports whether the service accepts reference images.
// GPT Image models accept reference images through the edits endpoint.
func (s *GPTImageService) SupportsImageReferences() bool {
	return true
}

// GenerateImage generates a single image using OpenAI's GPT Image API.
func (s *GPTImageService) GenerateImage(
	ctx context.Context,
	prompt string,
	references []ImageReference,
) ([]byte, error) {
	if s.client == nil {
		client := openai.NewClient(
			option.WithAPIKey(s.apiKey),
		)
		s.client = &client
	}

	if len(references) > 0 {
		return s.editImage(ctx, prompt, references)
	}

	params := openai.ImageGenerateParams{
		Model:  s.model,
		Prompt: prompt,
		N:      openai.Int(1),
		Size:   openai.ImageGenerateParamsSize(s.size),
	}

	response, err := s.client.Images.Generate(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to generate image: %w", err)
	}

	return decodeGPTImageResponse(response)
}

// NewGPTImageService creates a new OpenAI GPT image generation service.
func NewGPTImageService(apiKey, model, size string) *GPTImageService {
	return &GPTImageService{
		apiKey: apiKey,
		model:  model,
		size:   size,
	}
}
