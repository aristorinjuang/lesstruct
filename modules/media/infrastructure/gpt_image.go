package infrastructure

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
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

// buildImageEditFiles wraps reference images as named files for the edits
// endpoint. The API rejects anonymous parts (sent as application/octet-stream),
// so each part carries a filename and the detected MIME type.
func buildImageEditFiles(references []mediadomain.ImageReference) []io.Reader {
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
	case string(mediadomain.MimeTypePNG):
		ext = "png"
	case string(mediadomain.MimeTypeWebP):
		ext = "webp"
	}
	return fmt.Sprintf("reference-%d.%s", index+1, ext)
}

// buildImageEditBody assembles the multipart edits request body by hand so the
// reference-image parts carry indexed names (image[0], image[1], ...) instead
// of the SDK's image[] naming, which stricter OpenAI-compatible APIs (Meta
// Model API) reject. model, prompt, n, and size mirror the typed OpenAI
// params; size is omitted when empty so the provider default applies.
func buildImageEditBody(
	model, prompt, size string,
	references []mediadomain.ImageReference,
) (body *bytes.Buffer, contentType string, err error) {
	body = &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	writeField := func(name, value string) error {
		if err := writer.WriteField(name, value); err != nil {
			return fmt.Errorf("failed to write edits field %s: %w", name, err)
		}
		return nil
	}
	if err := writeField("model", model); err != nil {
		return nil, "", err
	}
	if err := writeField("prompt", prompt); err != nil {
		return nil, "", err
	}
	if err := writeField("n", "1"); err != nil {
		return nil, "", err
	}
	if size != "" {
		if err := writeField("size", size); err != nil {
			return nil, "", err
		}
	}
	for i, ref := range references {
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", fmt.Sprintf(
			`form-data; name="image[%d]"; filename="%s"`,
			i,
			referenceFilename(ref.MimeType, i),
		))
		header.Set("Content-Type", ref.MimeType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, "", fmt.Errorf("failed to build edits image part %d: %w", i, err)
		}
		if _, err := part.Write(ref.Data); err != nil {
			return nil, "", fmt.Errorf("failed to write edits image part %d: %w", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to close edits request body: %w", err)
	}
	return body, writer.FormDataContentType(), nil
}

// GPTImageService implements mediadomain.ImageGenerationService using OpenAI
// GPT Image models. A non-empty baseURL redirects the client to an
// OpenAI-compatible image API (e.g. Meta Model API for Muse Image); empty
// keeps OpenAI's default endpoint.
type GPTImageService struct {
	client  *openai.Client // singleton client
	apiKey  string
	baseURL string
	model   string
	size    string
}

var _ mediadomain.ImageGenerationService = (*GPTImageService)(nil)

// editImage generates a single image guided by reference images using the
// image edits endpoint.
func (s *GPTImageService) editImage(
	ctx context.Context,
	prompt string,
	references []mediadomain.ImageReference,
) ([]byte, error) {
	if s.baseURL != "" {
		return s.editImageCompatible(ctx, prompt, references)
	}
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

// editImageCompatible generates a single image guided by reference images on
// an OpenAI-compatible edits endpoint. The request body is assembled by hand
// so reference parts carry indexed names (image[0], image[1], ...): the
// openai-go SDK hardcodes image[] naming, which stricter providers (Meta
// Model API) reject. OpenAI's own endpoint keeps the typed SDK call above.
func (s *GPTImageService) editImageCompatible(
	ctx context.Context,
	prompt string,
	references []mediadomain.ImageReference,
) ([]byte, error) {
	body, contentType, err := buildImageEditBody(s.model, prompt, s.size, references)
	if err != nil {
		return nil, err
	}

	var response openai.ImagesResponse
	err = s.client.Post(
		ctx,
		"images/edits",
		nil,
		&response,
		option.WithRequestBody(contentType, body.Bytes()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to edit image: %w", err)
	}

	return decodeGPTImageResponse(&response)
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
	references []mediadomain.ImageReference,
) ([]byte, error) {
	if s.client == nil {
		options := []option.RequestOption{option.WithAPIKey(s.apiKey)}
		if s.baseURL != "" {
			options = append(options, option.WithBaseURL(s.baseURL))
		}
		client := openai.NewClient(options...)
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
// A non-empty baseURL points the client at an OpenAI-compatible image API
// (e.g. Meta Model API for Muse Image); pass "" for OpenAI's default.
func NewGPTImageService(apiKey, baseURL, model, size string) *GPTImageService {
	return &GPTImageService{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		size:    size,
	}
}
