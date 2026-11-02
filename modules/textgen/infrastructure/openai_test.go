package infrastructure_test

import (
	"context"
	"testing"

	textgendomain "github.com/aristorinjuang/lesstruct/modules/textgen/domain"
	textgeninfra "github.com/aristorinjuang/lesstruct/modules/textgen/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const defaultModel = "gpt-5-mini"

func TestNewOpenAITextService(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := textgeninfra.NewOpenAITextService(tt.apiKey, tt.baseURL, tt.model, "", "")
			require.NotNil(t, svc)
		})
	}
}

func TestOpenAITextService_ImplementsTextGenerationService(t *testing.T) {
	svc := textgeninfra.NewOpenAITextService("sk-fake-key", "", defaultModel, "", "")

	var iface textgendomain.TextGenerationService = svc
	assert.NotNil(t, iface)
}

func TestOpenAITextService_EnhanceText_ContextCancellation(t *testing.T) {
	svc := textgeninfra.NewOpenAITextService(
		"sk-fake-key",
		"",
		defaultModel,
		":root{--color-primary:#22d3ee}",
		".btn{background:var(--color-primary)}",
	)
	require.NotNil(t, svc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.EnhanceText(ctx, `{"type":"doc"}`, textgendomain.FormatTiptap, "")
	assert.Error(t, err, "EnhanceText with cancelled context should return error")
}

func TestOpenAITextService_TranslateText_ContextCancellation(t *testing.T) {
	svc := textgeninfra.NewOpenAITextService("sk-fake-key", "", defaultModel, "", "")
	require.NotNil(t, svc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.TranslateText(
		ctx,
		`{"type":"doc"}`,
		"Hello world",
		"A short summary.",
		"en",
		"fr",
		textgendomain.FormatTiptap,
	)
	assert.Error(t, err, "TranslateText with cancelled context should return error")
}

func TestOpenAITextService_EnhanceTranscription_ContextCancellation(t *testing.T) {
	svc := textgeninfra.NewOpenAITextService("sk-fake-key", "", defaultModel, "", "")
	require.NotNil(t, svc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.EnhanceTranscription(ctx, "heading two hello world")
	assert.Error(t, err, "EnhanceTranscription with cancelled context should return error")
}
