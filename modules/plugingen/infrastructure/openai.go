package infrastructure

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	plugingendomain "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
)

// firstLine keeps the head of a multi-line error for repair prompts.
func firstLine(message string) string {
	if head, _, ok := strings.Cut(message, "\n"); ok {
		return head
	}
	return message
}

// OpenAIPluginService implements plugingendomain.PluginGenerationService over
// any OpenAI-compatible chat-completion API.
type OpenAIPluginService struct {
	client     *openai.Client
	clientOnce sync.Once
	apiKey     string
	baseURL    string
	model      string
}

func (s *OpenAIPluginService) ensureClient() {
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

func (s *OpenAIPluginService) completeChat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
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
		return "", fmt.Errorf("failed to generate plugin: %w", err)
	}
	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("no completions returned")
	}
	return completion.Choices[0].Message.Content, nil
}

// GenerateProject asks the model for the JSON envelope and validates it via
// the domain parser. Files stay verbatim so validation and the repair loop
// see the model's original output. A response that is not valid JSON gets
// one immediate re-ask through the repair prompt — malformed output is the
// most common real failure mode, and without this the service-level repair
// loop (which only runs on parsed projects) would never engage.
func (s *OpenAIPluginService) GenerateProject(
	ctx context.Context,
	brief plugingendomain.Brief,
	sources plugingendomain.PluginSources,
) (plugingendomain.Project, error) {
	systemPrompt := BuildSystemPrompt(sources)
	userPrompt := BuildUserPrompt(brief)
	responseText, err := s.completeChat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return plugingendomain.Project{}, err
	}
	project, err := plugingendomain.ParseProjectEnvelope(responseText)
	if err == nil {
		return project, nil
	}
	repaired, repairErr := s.RepairProject(ctx, brief, sources, plugingendomain.Project{}, []string{"previous response was not valid plugin JSON: " + firstLine(err.Error())})
	if repairErr != nil {
		return plugingendomain.Project{}, err
	}
	return repaired, nil
}

// RepairProject feeds validation problems back to the model (the caller bounds
// the attempts). The system prompt matches the generation one so the contract
// stays identical across repairs.
func (s *OpenAIPluginService) RepairProject(
	ctx context.Context,
	brief plugingendomain.Brief,
	sources plugingendomain.PluginSources,
	project plugingendomain.Project,
	problems []string,
) (plugingendomain.Project, error) {
	systemPrompt := BuildSystemPrompt(sources)
	userPrompt := BuildRepairPrompt(brief, project.Files, problems)
	responseText, err := s.completeChat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return plugingendomain.Project{}, err
	}
	repaired, err := plugingendomain.ParseProjectEnvelope(responseText)
	if err != nil {
		return plugingendomain.Project{}, err
	}
	return repaired, nil
}

// NewOpenAIPluginService builds the OpenAI-compatible plugin generation
// service. baseURL is optional ("" = provider default); model names the chat
// model.
func NewOpenAIPluginService(apiKey, baseURL, model string) *OpenAIPluginService {
	return &OpenAIPluginService{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
	}
}

var _ plugingendomain.PluginGenerationService = (*OpenAIPluginService)(nil)
