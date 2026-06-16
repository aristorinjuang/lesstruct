package media

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// GPTImageService implements ImageGenerationService using OpenAI GPT Image models.
type GPTImageService struct {
	client *openai.Client // singleton client
	apiKey string
	model  string
	size   string
}

// GenerateImage generates a single image using OpenAI's GPT Image API.
func (s *GPTImageService) GenerateImage(ctx context.Context, prompt string) ([]byte, error) {
	if s.client == nil {
		client := openai.NewClient(
			option.WithAPIKey(s.apiKey),
		)
		s.client = &client
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

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("no images generated")
	}

	imageBytes, err := base64.StdEncoding.DecodeString(response.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 image: %w", err)
	}

	return imageBytes, nil
}

// NewGPTImageService creates a new OpenAI GPT image generation service.
func NewGPTImageService(apiKey, model, size string) *GPTImageService {
	return &GPTImageService{
		apiKey: apiKey,
		model:  model,
		size:   size,
	}
}
