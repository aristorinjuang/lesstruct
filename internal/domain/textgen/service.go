package textgen

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// tiptapSchemaPrompt describes the TipTap JSON structure to the AI model so it
// can output valid TipTap documents compatible with the editor.
const tiptapSchemaPrompt = `You must output VALID TipTap JSON matching this exact schema:

{
  "type": "doc",
  "content": [
    {
      "type": "paragraph",
      "content": [{"type": "text", "text": "plain text"}]
    },
    {
      "type": "heading",
      "attrs": {"level": 2},
      "content": [{"type": "text", "text": "Heading"}]
    },
    {
      "type": "bulletList",
      "content": [{"type": "listItem", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "item"}]}]}]
    },
    {
      "type": "orderedList",
      "content": [{"type": "listItem", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "item"}]}]}]
    },
    {
      "type": "blockquote",
      "content": [{"type": "paragraph", "content": [{"type": "text", "text": "quote"}]}]
    },
    {
      "type": "codeBlock",
      "attrs": {"language": "javascript"},
      "content": [{"type": "text", "text": "console.log('hello')"}]
    },
    {"type": "horizontalRule"},
    {
      "type": "image",
      "attrs": {"src": "https://...", "alt": "description"}
    },
    {
      "type": "table",
      "content": [
        {"type": "tableRow", "content": [
          {"type": "tableHeader", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Header"}]}]},
          {"type": "tableCell", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Cell"}]}]}
        ]}
      ]
    },
    {"type": "youtube", "attrs": {"src": "https://www.youtube.com/embed/..."}},
    {
      "type": "paragraph",
      "content": [{"type": "text", "marks": [{"type": "bold"}], "text": "bold"}, {"type": "text", "marks": [{"type": "italic"}], "text": "italic"}, {"type": "text", "marks": [{"type": "underline"}], "text": "underline"}]
    },
    {
      "type": "paragraph",
      "content": [{"type": "emoji", "attrs": {"name": "rocket"}}, {"type": "text", "text": " with emoji"}]
    },
    {
      "type": "paragraph",
      "content": [{"type": "text", "marks": [{"type": "link", "attrs": {"href": "https://example.com"}}], "text": "link text"}]
    },
    {
      "type": "paragraph",
      "content": [
        {"type": "text", "text": "Inline math: "},
        {"type": "inlineMath", "attrs": {"latex": "E=mc^2"}}
      ]
    },
    {
      "type": "blockMath",
      "attrs": {"latex": "\\int_0^\\infty e^{-x^2} dx = \\frac{\\sqrt{\\pi}}{2}"}
    }
  ]
}

IMPORTANT RULES:
1. Output ONLY the JSON — no markdown fences, no explanation, no commentary.
2. Preserve all inline marks: bold, italic, underline, link, code.
3. For code blocks, always include the "attrs": {"language": "..."} field.
4. For images, keep the original src and alt attributes unchanged.
5. For YouTube embeds, keep the original src attribute unchanged.
6. For math (inlineMath, blockMath), keep the original latex attribute unchanged.
7. Do not invent new node types or mark types — use only the types shown above.
8. NEVER use heading level 1 (h1) — headings must start at level 2 or higher.
9. Always wrap text content inside a paragraph node unless it belongs in a heading, list, blockquote, or code block.`

// TextGenerationService defines the interface for AI text generation services.
type TextGenerationService interface {
	EnhanceText(ctx context.Context, content string) (string, error)
	TranslateText(ctx context.Context, content, sourceLang, targetLang string) (string, error)
}

// OpenAITextService implements TextGenerationService using any OpenAI-compatible API.
type OpenAITextService struct {
	client  *openai.Client // singleton client
	apiKey  string
	baseURL string
	model   string
}

func (s *OpenAITextService) callChatCompletion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
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
		return "", fmt.Errorf("failed to generate text: %w", err)
	}

	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("no completions returned")
	}

	responseText := completion.Choices[0].Message.Content

	// Strip markdown code fences if present
	responseText = strings.TrimSpace(responseText)
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	// Validate that the response is valid JSON
	var parsed any
	if err := json.Unmarshal([]byte(responseText), &parsed); err != nil {
		return "", fmt.Errorf("AI response is not valid JSON: %w", err)
	}

	// Ensure it has the expected TipTap doc structure
	if doc, ok := parsed.(map[string]any); ok {
		if docType, ok := doc["type"].(string); !ok || docType != "doc" {
			return "", fmt.Errorf("AI response is not a valid TipTap document: missing 'doc' type")
		}
	}

	return responseText, nil
}

// EnhanceText takes existing TipTap JSON content and returns an enhanced version.
func (s *OpenAITextService) EnhanceText(ctx context.Context, content string) (string, error) {
	if s.client == nil {
		opts := []option.RequestOption{
			option.WithAPIKey(s.apiKey),
		}
		if s.baseURL != "" {
			opts = append(opts, option.WithBaseURL(s.baseURL))
		}
		client := openai.NewClient(opts...)
		s.client = &client
	}

	systemPrompt := `You are an expert content editor. Your task is to enhance the provided content to be more engaging, compelling, and well-structured.

Understand the content's language, tone, and subject matter. Then:
- Improve clarity, flow, and readability
- Add more vivid descriptions where appropriate
- Make headings more compelling
- Ensure logical structure and organization
- Correct any grammatical issues
- Maintain the original language — do NOT translate
- Preserve the original meaning and key information

` + tiptapSchemaPrompt

	userPrompt := "Enhance this content to be more engaging. Output only the enhanced TipTap JSON:\n\n" + content

	return s.callChatCompletion(ctx, systemPrompt, userPrompt)
}

// TranslateText translates content from sourceLang to targetLang, preserving TipTap structure.
func (s *OpenAITextService) TranslateText(ctx context.Context, content, sourceLang, targetLang string) (string, error) {
	if s.client == nil {
		opts := []option.RequestOption{
			option.WithAPIKey(s.apiKey),
		}
		if s.baseURL != "" {
			opts = append(opts, option.WithBaseURL(s.baseURL))
		}
		client := openai.NewClient(opts...)
		s.client = &client
	}

	systemPrompt := fmt.Sprintf(
		`You are an expert translator. Translate the provided content from %s to %s.

- Preserve the TipTap JSON structure exactly — only translate the text content
- Maintain all formatting: bold, italic, underline, links, headings, lists, etc.
- Keep code blocks, image URLs, YouTube URLs, and math formulas unchanged
- Preserve all node types, attributes, and marks — only change text values and alt text
- Output ONLY the translated TipTap JSON with no additional commentary

%s`,
		strings.ToUpper(sourceLang),
		strings.ToUpper(targetLang),
		tiptapSchemaPrompt,
	)

	userPrompt := fmt.Sprintf("Translate this content from %s to %s. Output only the translated TipTap JSON:\n\n%s", strings.ToUpper(sourceLang), strings.ToUpper(targetLang), content)

	return s.callChatCompletion(ctx, systemPrompt, userPrompt)
}

// NewOpenAITextService creates a new OpenAI-compatible text generation service.
// baseURL is optional — pass "" to use the OpenAI default.
// model is the chat model name (e.g. "gpt-5-mini", "gpt-5.4-mini", "openai/gpt-5-mini" for OpenRouter).
func NewOpenAITextService(apiKey, baseURL, model string) *OpenAITextService {
	return &OpenAITextService{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
	}
}
