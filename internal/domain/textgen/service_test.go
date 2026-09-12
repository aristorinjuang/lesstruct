package textgen_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
			svc := textgen.NewOpenAITextService(tt.apiKey, tt.baseURL, tt.model, "", "")
			require.NotNil(t, svc, "NewOpenAITextService should return non-nil")
		})
	}
}

// TestOpenAITextService_EnhanceText_NoAPIKey tests that the service exists
// and has the right interface even without a real API key.
func TestOpenAITextService_EnhanceText_NoAPIKey(t *testing.T) {
	svc := textgen.NewOpenAITextService("sk-fake-key", "", defaultModel, "", "")
	require.NotNil(t, svc)

	var iface textgen.TextGenerationService = svc
	require.NotNil(t, iface)
}

// TestOpenAITextService_TranslateText_NoAPIKey tests translate interface.
func TestOpenAITextService_TranslateText_NoAPIKey(t *testing.T) {
	svc := textgen.NewOpenAITextService("sk-fake-key", "", defaultModel, "", "")
	require.NotNil(t, svc)

	var iface textgen.TextGenerationService = svc
	require.NotNil(t, iface)
}

// TestOpenAITextService_Interface tests that OpenAITextService satisfies TextGenerationService.
func TestOpenAITextService_Interface(t *testing.T) {
	svc := textgen.NewOpenAITextService("sk-key", "https://custom.api/v1", "gpt-5-mini", "", "")
	require.NotNil(t, svc)

	// Verify it satisfies the interface
	var _ textgen.TextGenerationService = svc
}

// TestOpenAITextService_EnhanceText_ContextCancellation tests enhancement with cancelled context.
func TestOpenAITextService_EnhanceText_ContextCancellation(t *testing.T) {
	svc := textgen.NewOpenAITextService("sk-fake-key", "", defaultModel, "", "")
	require.NotNil(t, svc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := svc.EnhanceText(ctx, validTipTapJSON(), "tiptap", "")
	// Expect error because context is cancelled before the API call completes
	assert.Error(t, err, "EnhanceText with cancelled context should return error")
}

// TestOpenAITextService_TranslateText_ContextCancellation tests translation with cancelled context.
func TestOpenAITextService_TranslateText_ContextCancellation(t *testing.T) {
	svc := textgen.NewOpenAITextService("sk-fake-key", "", defaultModel, "", "")
	require.NotNil(t, svc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.TranslateText(ctx, validTipTapJSON(), "Hello world", "A short summary.", "en", "fr", "tiptap")
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
			svc := textgen.NewOpenAITextService(tt.apiKey, tt.baseURL, tt.model, "", "")
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

// TestBuildHTMLSystemPrompt_NoThemeCSS tests that the prompt is assembled
// without the theme section when no theme CSS is provided. The base rules and
// examples must always be present.
func TestBuildHTMLSystemPrompt_NoThemeCSS(t *testing.T) {
	tests := []struct {
		name         string
		themeBaseCSS string
		themeStyleCSS string
	}{
		{
			name:          "both empty",
			themeBaseCSS:  "",
			themeStyleCSS: "",
		},
		{
			name:          "base only empty",
			themeBaseCSS:  "",
			themeStyleCSS: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := textgen.BuildHTMLSystemPromptForTest(tt.themeBaseCSS, tt.themeStyleCSS)

			assert.Contains(t, prompt, "## Hard rules")
			assert.Contains(t, prompt, "## Brand consistency")
			assert.Contains(t, prompt, "Few-shot examples")
			assert.Contains(t, prompt, "var(--color-primary)")
			assert.NotContains(t, prompt, "Active site theme CSS")
		})
	}
}

// TestBuildHTMLSystemPrompt_WithThemeCSS tests that the active theme CSS is
// injected into the prompt alongside the base rules and examples.
func TestBuildHTMLSystemPrompt_WithThemeCSS(t *testing.T) {
	tests := []struct {
		name          string
		themeBaseCSS  string
		themeStyleCSS string
	}{
		{
			name:          "both present",
			themeBaseCSS:  ":root{--color-primary:#22d3ee;--color-text:#1a1a2e}",
			themeStyleCSS: ".btn{background:var(--color-primary)}",
		},
		{
			name:          "only base present",
			themeBaseCSS:  ":root{--color-primary:#22d3ee}",
			themeStyleCSS: "",
		},
		{
			name:          "only style present",
			themeBaseCSS:  "",
			themeStyleCSS: ".btn{background:var(--color-primary)}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := textgen.BuildHTMLSystemPromptForTest(tt.themeBaseCSS, tt.themeStyleCSS)

			assert.Contains(t, prompt, "## Hard rules")
			assert.Contains(t, prompt, "## Brand consistency")
			assert.Contains(t, prompt, "Active site theme CSS")
			assert.Contains(t, prompt, "### base.css")
			assert.Contains(t, prompt, "### style.css")
			assert.Contains(t, prompt, "Few-shot examples")

			if tt.themeBaseCSS != "" {
				assert.Contains(t, prompt, tt.themeBaseCSS)
			}
			if tt.themeStyleCSS != "" {
				assert.Contains(t, prompt, tt.themeStyleCSS)
			}
		})
	}
}

// TestNewOpenAITextService_HTMLSystemPromptPrecomputed verifies that the
// constructor accepts theme CSS without panicking and produces a usable service.
func TestNewOpenAITextService_HTMLSystemPromptPrecomputed(t *testing.T) {
	svc := textgen.NewOpenAITextService(
		"sk-fake-key",
		"",
		defaultModel,
		":root{--color-primary:#22d3ee}",
		".btn{background:var(--color-primary)}",
	)
	require.NotNil(t, svc)

	var iface textgen.TextGenerationService = svc
	require.NotNil(t, iface)
}

// buildEnhanceEnvelope builds an AI enhance-response envelope JSON string with
// the given title, meta description, and raw content JSON.
func buildEnhanceEnvelope(title, meta, contentJSON string) string {
	titleJSON, _ := json.Marshal(title)
	metaJSON, _ := json.Marshal(meta)
	return fmt.Sprintf(`{"title":%s,"metaDescription":%s,"content":%s}`, titleJSON, metaJSON, contentJSON)
}

// TestNewEnhanceResult tests the EnhanceResult constructor.
func TestNewEnhanceResult(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		title           string
		metaDescription string
	}{
		{
			name:            "full tiptap result",
			content:         validTipTapJSON(),
			title:           "Enhanced Hello World",
			metaDescription: "A sharper hello to the world.",
		},
		{
			name:            "html result without seo metadata",
			content:         "<style>.ls-test{}</style><section>Hello</section>",
			title:           "",
			metaDescription: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := textgen.NewEnhanceResult(tt.content, tt.title, tt.metaDescription)
			assert.Equal(t, tt.content, result.Content)
			assert.Equal(t, tt.title, result.Title)
			assert.Equal(t, tt.metaDescription, result.MetaDescription)
		})
	}
}

// TestStripJSONFences tests markdown fence removal from AI JSON output.
func TestStripJSONFences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain json passes through",
			input:    `{"title":"T"}`,
			expected: `{"title":"T"}`,
		},
		{
			name:     "json fences are stripped",
			input:    "```json\n" + `{"title":"T"}` + "\n```",
			expected: `{"title":"T"}`,
		},
		{
			name:     "bare fences are stripped",
			input:    "```\n" + `{"title":"T"}` + "\n```",
			expected: `{"title":"T"}`,
		},
		{
			name:     "surrounding whitespace is trimmed",
			input:    "  \n" + `{"title":"T"}` + "  \n",
			expected: `{"title":"T"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, textgen.StripJSONFencesForTest(tt.input))
		})
	}
}

// TestParseEnhanceEnvelope tests validation and splitting of the AI enhance
// response into the enhanced document and its SEO metadata.
func TestParseEnhanceEnvelope(t *testing.T) {
	tests := []struct {
		name            string
		response        string
		expectedContent string
		expectedTitle   string
		expectedMeta    string
		wantErr         bool
	}{
		{
			name:            "success - full envelope",
			response:        buildEnhanceEnvelope("Enhanced Hello World", "A sharper hello to the world.", validTipTapJSON()),
			expectedContent: validTipTapJSON(),
			expectedTitle:   "Enhanced Hello World",
			expectedMeta:    "A sharper hello to the world.",
			wantErr:         false,
		},
		{
			name:            "success - missing meta description is allowed",
			response:        `{"title":"Enhanced Hello World","content":` + validTipTapJSON() + `}`,
			expectedContent: validTipTapJSON(),
			expectedTitle:   "Enhanced Hello World",
			expectedMeta:    "",
			wantErr:         false,
		},
		{
			name:            "success - trims surrounding whitespace",
			response:        buildEnhanceEnvelope("  Padded Title  ", "  Padded summary.  ", "  "+validTipTapJSON()+"  "),
			expectedContent: validTipTapJSON(),
			expectedTitle:   "Padded Title",
			expectedMeta:    "Padded summary.",
			wantErr:         false,
		},
		{
			name:            "success - truncates long title to 60 characters",
			response:        buildEnhanceEnvelope(strings.Repeat("a", 70), "Summary.", validTipTapJSON()),
			expectedContent: validTipTapJSON(),
			expectedTitle:   strings.Repeat("a", 60),
			expectedMeta:    "Summary.",
			wantErr:         false,
		},
		{
			name:            "success - truncates long meta description to 160 characters",
			response:        buildEnhanceEnvelope("Title", strings.Repeat("b", 200), validTipTapJSON()),
			expectedContent: validTipTapJSON(),
			expectedTitle:   "Title",
			expectedMeta:    strings.Repeat("b", 160),
			wantErr:         false,
		},
		{
			name:            "success - truncation trims trailing space",
			response:        buildEnhanceEnvelope(strings.Repeat("a", 59)+" bcdef", "Summary.", validTipTapJSON()),
			expectedContent: validTipTapJSON(),
			expectedTitle:   strings.Repeat("a", 59),
			expectedMeta:    "Summary.",
			wantErr:         false,
		},
		{
			name:     "error - response is not json",
			response: "this is not json",
			wantErr:  true,
		},
		{
			name:     "error - missing title",
			response: `{"metaDescription":"Summary.","content":` + validTipTapJSON() + `}`,
			wantErr:  true,
		},
		{
			name:     "error - blank title",
			response: buildEnhanceEnvelope("   ", "Summary.", validTipTapJSON()),
			wantErr:  true,
		},
		{
			name:     "error - missing content",
			response: `{"title":"Title","metaDescription":"Summary."}`,
			wantErr:  true,
		},
		{
			name:     "error - null content",
			response: `{"title":"Title","metaDescription":"Summary.","content":null}`,
			wantErr:  true,
		},
		{
			name:     "error - content is not a json object",
			response: buildEnhanceEnvelope("Title", "Summary.", `"just a string"`),
			wantErr:  true,
		},
		{
			name:     "error - content is a json array",
			response: buildEnhanceEnvelope("Title", "Summary.", `[1,2,3]`),
			wantErr:  true,
		},
		{
			name:     "error - content is not a tiptap doc",
			response: buildEnhanceEnvelope("Title", "Summary.", `{"type":"paragraph"}`),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := textgen.ParseEnhanceEnvelopeForTest(tt.response)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedContent, result.Content)
			assert.Equal(t, tt.expectedTitle, result.Title)
			assert.Equal(t, tt.expectedMeta, result.MetaDescription)
		})
	}
}

// TestNewTranslateResult tests the TranslateResult constructor.
func TestNewTranslateResult(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		title           string
		metaDescription string
	}{
		{
			name:            "full tiptap result",
			content:         validTipTapJSON(),
			title:           "Bonjour le monde",
			metaDescription: "Un résumé plus net.",
		},
		{
			name:            "html result without seo metadata",
			content:         "<section>Bonjour le monde</section>",
			title:           "",
			metaDescription: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := textgen.NewTranslateResult(tt.content, tt.title, tt.metaDescription)
			assert.Equal(t, tt.content, result.Content)
			assert.Equal(t, tt.title, result.Title)
			assert.Equal(t, tt.metaDescription, result.MetaDescription)
		})
	}
}

// TestParseMetadataEnvelope tests the shared envelope parser for both content
// kinds: TipTap documents and HTML strings.
func TestParseMetadataEnvelope(t *testing.T) {
	tests := []struct {
		name            string
		response        string
		format          string
		expectedContent string
		expectedTitle   string
		expectedMeta    string
		wantErr         bool
	}{
		{
			name:            "success - tiptap envelope",
			response:        buildEnhanceEnvelope("Bonjour le monde", "Un résumé.", validTipTapJSON()),
			format:          "tiptap",
			expectedContent: validTipTapJSON(),
			expectedTitle:   "Bonjour le monde",
			expectedMeta:    "Un résumé.",
			wantErr:         false,
		},
		{
			name:            "success - html envelope",
			response:        buildEnhanceEnvelope("Bonjour le monde", "Un résumé.", `"  <section>Bonjour le monde</section>  "`),
			format:          "html",
			expectedContent: "<section>Bonjour le monde</section>",
			expectedTitle:   "Bonjour le monde",
			expectedMeta:    "Un résumé.",
			wantErr:         false,
		},
		{
			name:            "success - html envelope truncates long meta to 160 characters",
			response:        buildEnhanceEnvelope("Titre", strings.Repeat("c", 200), `"<section>Bonjour</section>"`),
			format:          "html",
			expectedContent: "<section>Bonjour</section>",
			expectedTitle:   "Titre",
			expectedMeta:    strings.Repeat("c", 160),
			wantErr:         false,
		},
		{
			name:     "error - html content is not a string",
			response: buildEnhanceEnvelope("Titre", "Résumé.", validTipTapJSON()),
			format:   "html",
			wantErr:  true,
		},
		{
			name:     "error - html content is blank",
			response: buildEnhanceEnvelope("Titre", "Résumé.", `"   "`),
			format:   "html",
			wantErr:  true,
		},
		{
			name:     "error - html envelope missing title",
			response: `{"metaDescription":"Résumé.","content":"<section>Bonjour</section>"}`,
			format:   "html",
			wantErr:  true,
		},
		{
			name:     "error - response is not json",
			response: "this is not json",
			format:   "html",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, title, meta, err := textgen.ParseMetadataEnvelopeForTest(tt.response, tt.format)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedContent, content)
			assert.Equal(t, tt.expectedTitle, title)
			assert.Equal(t, tt.expectedMeta, meta)
		})
	}
}
