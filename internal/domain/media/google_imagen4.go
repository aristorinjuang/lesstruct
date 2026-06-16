package media

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// GoogleImagen4Service implements ImageGenerationService using Google Imagen 4.
type GoogleImagen4Service struct {
	client      *genai.Client               // singleton client
	config      *genai.GenerateImagesConfig // singleton config
	apiKey      string
	model       string
	size        string
	aspectRatio string
}

// GenerateImage generates a single image using Google Imagen 4.
func (s *GoogleImagen4Service) GenerateImage(ctx context.Context, prompt string) ([]byte, error) {
	if s.client == nil {
		var err error
		s.client, err = genai.NewClient(ctx, &genai.ClientConfig{
			APIKey: s.apiKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create genai client: %w", err)
		}
	}
	if s.config == nil {
		s.config = &genai.GenerateImagesConfig{
			NumberOfImages: 1,
		}
		if s.size != "" {
			s.config.ImageSize = s.size
		}
		if s.aspectRatio != "" {
			s.config.AspectRatio = s.aspectRatio
		}
	}

	response, err := s.client.Models.GenerateImages(ctx, s.model, prompt, s.config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate image: %w", err)
	}

	if len(response.GeneratedImages) == 0 {
		return nil, fmt.Errorf("no images generated")
	}

	return response.GeneratedImages[0].Image.ImageBytes, nil
}

// NewGoogleImagen4Service creates a new Google Imagen 4 image generation service.
func NewGoogleImagen4Service(apiKey, model, size, aspectRatio string) *GoogleImagen4Service {
	return &GoogleImagen4Service{
		apiKey:      apiKey,
		model:       model,
		size:        size,
		aspectRatio: aspectRatio,
	}
}
