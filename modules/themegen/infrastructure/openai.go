package infrastructure

import (
	"context"
	"fmt"
	"sync"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
)

// OpenAIThemeService implements themegendomain.ThemeGenerationService over
// any OpenAI-compatible chat-completion API.
type OpenAIThemeService struct {
	client     *openai.Client
	clientOnce sync.Once
	apiKey     string
	baseURL    string
	model      string
}

var _ themegendomain.ThemeGenerationService = (*OpenAIThemeService)(nil)

func (s *OpenAIThemeService) ensureClient() {
	s.clientOnce.Do(func() {
		opts := []option.RequestOption{
			option.WithAPIKey(s.apiKey),
		}
		if s.baseURL != "" {
			opts = append(opts, option.WithBaseURL(s.baseURL))
		}
		client := openai.NewClient(opts...)
		s.client = &client
	})
}

func (s *OpenAIThemeService) completeChat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	s.ensureClient()
	params := openai.ChatCompletionNewParams{
		Model: s.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		MaxCompletionTokens: openai.Int(16384),
		Temperature:         openai.Float(0.7),
	}
	completion, err := s.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to generate theme: %w", err)
	}
	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("no completions returned")
	}
	return completion.Choices[0].Message.Content, nil
}

// GenerateTheme asks the model for the JSON envelope and validates it via
// the domain parser. Files stay verbatim so validation and the repair loop
// see the model's original output; minification happens at install time.
// The system prompt is built per call so template overrides and their
// sources are honored.
func (s *OpenAIThemeService) GenerateTheme(
	ctx context.Context,
	brief themegendomain.Brief,
	edition themegendomain.Edition,
	sources themegendomain.ThemeSources,
) (themegendomain.Theme, error) {
	systemPrompt := BuildSystemPrompt(sources, brief.TemplateOverrides)
	userPrompt := BuildUserPrompt(brief.Description, brief.Style)
	responseText, err := s.completeChat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return themegendomain.Theme{}, err
	}
	theme, err := themegendomain.ParseThemeEnvelope(responseText, edition)
	if err != nil {
		return themegendomain.Theme{}, err
	}
	return theme, nil
}

// RepairTheme feeds validation problems back to the model (the caller bounds
// the attempts). The system prompt matches the generation one, so template
// overrides keep their contract across repairs.
func (s *OpenAIThemeService) RepairTheme(
	ctx context.Context,
	brief themegendomain.Brief,
	sources themegendomain.ThemeSources,
	theme themegendomain.Theme,
	problems []string,
) (themegendomain.Theme, error) {
	systemPrompt := BuildSystemPrompt(sources, brief.TemplateOverrides)
	userPrompt := BuildRepairPrompt(brief, theme.Files, problems)
	responseText, err := s.completeChat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return themegendomain.Theme{}, err
	}
	repaired, err := themegendomain.ParseThemeEnvelope(responseText, theme.Edition)
	if err != nil {
		return themegendomain.Theme{}, err
	}
	return repaired, nil
}

// NewOpenAIThemeService builds the OpenAI-compatible theme generation
// service. baseURL is optional ("" = provider default); model names the chat
// model. Sources and the template-override flag arrive per generation call.
func NewOpenAIThemeService(apiKey, baseURL, model string) *OpenAIThemeService {
	return &OpenAIThemeService{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
	}
}
