package media

import (
	"context"
)

// ImageGenerationService defines the interface for AI image generation services.
type ImageGenerationService interface {
	GenerateImage(ctx context.Context, prompt string) ([]byte, error)
}
