package media

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// GoogleGeminiService implements ImageGenerationService using Google Gemini models.
type GoogleGeminiService struct {
	client      *genai.Client                   // singleton client
	config      *genai.GenerateContentConfig    // singleton config
	apiKey      string
	model       string
	size        string
	aspectRatio string
}

// SupportsImageReferences reports whether the service accepts reference images.
// Gemini image models accept inline image parts alongside the prompt.
func (s *GoogleGeminiService) SupportsImageReferences() bool {
	return true
}

// GenerateImage generates a single image using Google Gemini.
func (s *GoogleGeminiService) GenerateImage(
	ctx context.Context,
	prompt string,
	references []ImageReference,
) ([]byte, error) {
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
		s.config = &genai.GenerateContentConfig{}
		if s.size != "" || s.aspectRatio != "" {
			s.config.ImageConfig = &genai.ImageConfig{}
			if s.size != "" {
				s.config.ImageConfig.ImageSize = s.size
			}
			if s.aspectRatio != "" {
				s.config.ImageConfig.AspectRatio = s.aspectRatio
			}
		}
	}

	result, err := s.client.Models.GenerateContent(ctx, s.model, buildGeminiContents(prompt, references), s.config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate image: %w", err)
	}

	if result == nil || len(result.Candidates) == 0 || result.Candidates[0].Content == nil {
		return nil, fmt.Errorf("no content in response")
	}

	for _, part := range result.Candidates[0].Content.Parts {
		if part.InlineData != nil && len(part.InlineData.Data) > 0 {
			return part.InlineData.Data, nil
		}
	}

	return nil, fmt.Errorf("no image generated in response")
}

// buildGeminiContents builds the text prompt plus one inline image part per
// reference image, matching how genai.Text builds a prompt-only request.
func buildGeminiContents(prompt string, references []ImageReference) []*genai.Content {
	parts := make([]*genai.Part, 0, len(references)+1)
	parts = append(parts, genai.NewPartFromText(prompt))
	for _, ref := range references {
		parts = append(parts, genai.NewPartFromBytes(ref.Data, ref.MimeType))
	}
	return []*genai.Content{{Role: genai.RoleUser, Parts: parts}}
}

// NewGoogleGeminiService creates a new Google Gemini image generation service.
func NewGoogleGeminiService(apiKey, model, size, aspectRatio string) *GoogleGeminiService {
	return &GoogleGeminiService{
		apiKey:      apiKey,
		model:       model,
		size:        size,
		aspectRatio: aspectRatio,
	}
}
