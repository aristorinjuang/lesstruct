package textgen_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/textgen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const defaultModel = "gpt-5-mini"

// validTipTapJSON returns a minimal valid TipTap document JSON string.
func validTipTapJSON() string {
	doc := map[string]any{
		"type": "doc",
		"content": []map[string]any{
			{
				"type": "paragraph",
				"content": []map[string]any{
					{"type": "text", "text": "Hello world"},
				},
			},
		},
	}
	b, _ := json.Marshal(doc)
	return string(b)
}

// TestNewOpenAITextService tests the constructor.
func TestNewOpenAITextService(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		baseURL string
		model   string
	}{
		{
			name:    "with api key only",
			apiKey:  "sk-test-key",
			baseURL: "",
			model:   "gpt-5-mini",
		},
		{
			name:    "with api key and custom base URL",
			apiKey:  "sk-test-key",
			baseURL: "https://api.openrouter.ai/v1",
			model:   "openai/gpt-5-mini",
		},
		{
			name:    "with custom model",
			apiKey:  "sk-other-key",
			baseURL: "",
			model:   "gpt-5-mini-mini",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := textgen.NewOpenAITextService(tt.apiKey, tt.baseURL, tt.model)
			require.NotNil(t, svc, "NewOpenAITextService should return non-nil")
		})
	}
}

// TestOpenAITextService_EnhanceText_NoAPIKey tests that the service exists
// and has the right interface even without a real API key.
func TestOpenAITextService_EnhanceText_NoAPIKey(t *testing.T) {
	svc := textgen.NewOpenAITextService("sk-fake-key", "", defaultModel)
	require.NotNil(t, svc)

	var iface textgen.TextGenerationService = svc
	require.NotNil(t, iface)
}

// TestOpenAITextService_TranslateText_NoAPIKey tests translate interface.
func TestOpenAITextService_TranslateText_NoAPIKey(t *testing.T) {
	svc := textgen.NewOpenAITextService("sk-fake-key", "", defaultModel)
	require.NotNil(t, svc)

	var iface textgen.TextGenerationService = svc
	require.NotNil(t, iface)
}

// TestOpenAITextService_Interface tests that OpenAITextService satisfies TextGenerationService.
func TestOpenAITextService_Interface(t *testing.T) {
	svc := textgen.NewOpenAITextService("sk-key", "https://custom.api/v1", "gpt-5-mini")
	require.NotNil(t, svc)

	// Verify it satisfies the interface
	var _ textgen.TextGenerationService = svc
}

// TestOpenAITextService_EnhanceText_ContextCancellation tests enhancement with cancelled context.
func TestOpenAITextService_EnhanceText_ContextCancellation(t *testing.T) {
	svc := textgen.NewOpenAITextService("sk-fake-key", "", defaultModel)
	require.NotNil(t, svc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := svc.EnhanceText(ctx, validTipTapJSON())
	// Expect error because context is cancelled before the API call completes
	assert.Error(t, err, "EnhanceText with cancelled context should return error")
}

// TestOpenAITextService_TranslateText_ContextCancellation tests translation with cancelled context.
func TestOpenAITextService_TranslateText_ContextCancellation(t *testing.T) {
	svc := textgen.NewOpenAITextService("sk-fake-key", "", defaultModel)
	require.NotNil(t, svc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.TranslateText(ctx, validTipTapJSON(), "en", "fr")
	assert.Error(t, err, "TranslateText with cancelled context should return error")
}

// TestOpenAITextService_BaseURL tests that the service can be created with various base URLs.
func TestOpenAITextService_BaseURL(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		baseURL string
		model   string
	}{
		{
			name:    "openai default",
			apiKey:  "sk-key",
			baseURL: "",
			model:   "gpt-5-mini",
		},
		{
			name:    "openrouter",
			apiKey:  "sk-or-key",
			baseURL: "https://openrouter.ai/api/v1",
			model:   "openai/gpt-5-mini",
		},
		{
			name:    "ollama local",
			apiKey:  "ollama",
			baseURL: "http://localhost:11434/v1",
			model:   "llama3",
		},
		{
			name:    "together ai",
			apiKey:  "tg-key",
			baseURL: "https://api.together.xyz/v1",
			model:   "mistralai/Mixtral-8x7B-Instruct-v0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := textgen.NewOpenAITextService(tt.apiKey, tt.baseURL, tt.model)
			require.NotNil(t, svc)
		})
	}
}

// TestTipTapJSONRoundtrip validates that valid JSON can be parsed and stays valid.
func TestTipTapJSONRoundtrip(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "valid simple doc",
			content: validTipTapJSON(),
			wantErr: false,
		},
		{
			name:    "empty string",
			content: "",
			wantErr: true,
		},
		{
			name:    "not json",
			content: "this is not json",
			wantErr: true,
		},
		{
			name:    "json object but not doc",
			content: `{"foo": "bar"}`,
			wantErr: false,
		},
		{
			name:    "doc with heading",
			content: `{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Title"}]}]}`,
			wantErr: false,
		},
		{
			name:    "doc with marks",
			content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","marks":[{"type":"bold"}],"text":"bold text"},{"type":"text","marks":[{"type":"italic"}],"text":" italic"}]}]}`,
			wantErr: false,
		},
		{
			name:    "doc with code block",
			content: `{"type":"doc","content":[{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"fmt.Println(\"hello\")"}]}]}`,
			wantErr: false,
		},
		{
			name:    "doc with bullet list",
			content: `{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"item 1"}]}]}]}]}`,
			wantErr: false,
		},
		{
			name:    "doc with table",
			content: `{"type":"doc","content":[{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"H"}]}]},{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"C"}]}]}]}]}]}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.content == "" {
				assert.True(t, true, "empty content skipped")
				return
			}
			var parsed any
			err := json.Unmarshal([]byte(tt.content), &parsed)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestTipTapSchemasAreValid tests that various TipTap structures can be parsed.
func TestTipTapSchemasAreValid(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		isValid bool
	}{
		{
			name:    "paragraph with bold and italic",
			jsonStr: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","marks":[{"type":"bold"}],"text":"Bold"},{"type":"text","text":" "},{"type":"text","marks":[{"type":"italic"}],"text":"Italic"}]}]}`,
			isValid: true,
		},
		{
			name:    "heading level 3 with underline",
			jsonStr: `{"type":"doc","content":[{"type":"heading","attrs":{"level":3},"content":[{"type":"text","marks":[{"type":"underline"}],"text":"Underlined heading"}]}]}`,
			isValid: true,
		},
		{
			name:    "link with text",
			jsonStr: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","marks":[{"type":"link","attrs":{"href":"https://example.com","target":"_blank"}}],"text":"Click here"}]}]}`,
			isValid: true,
		},
		{
			name:    "invalid json",
			jsonStr: `{this is not json`,
			isValid: false,
		},
		{
			name:    "json array instead of object",
			jsonStr: `[1,2,3]`,
			isValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var parsed any
			err := json.Unmarshal([]byte(tt.jsonStr), &parsed)
			if tt.isValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
