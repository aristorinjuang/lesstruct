package infrastructure_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
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

// captureCompletions serves canned chat completions while recording every
// system prompt the client sends.
func captureCompletions(t *testing.T, systems *[]string, mu *sync.Mutex) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		mu.Lock()
		for _, message := range payload.Messages {
			if message.Role == "system" {
				*systems = append(*systems, message.Content)
			}
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"<p>hi</p>"},"finish_reason":"stop"}],"usage":{}}`))
	}))
}

func TestOpenAITextServiceRefreshThemeStyles(t *testing.T) {
	tests := []struct {
		name         string
		initialCSS   string
		refreshedCSS string
		wantInPrompt string
	}{
		{
			name:         "success - refreshed styles reach the prompt",
			initialCSS:   ":root{--color-primary:#111111}",
			refreshedCSS: ":root{--color-primary:#222222}",
			wantInPrompt: "#222222",
		},
		{
			name:         "success - clearing styles falls back to contract",
			initialCSS:   ":root{--color-primary:#111111}",
			refreshedCSS: "",
			wantInPrompt: "--color-primary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var systems []string
			server := captureCompletions(t, &systems, &mu)
			defer server.Close()
			svc := textgeninfra.NewOpenAITextService("sk-fake-key", server.URL, defaultModel, tt.initialCSS, "")
			require.NotNil(t, svc)

			_, err := svc.EnhanceText(context.Background(), "make a hero", textgendomain.FormatHTML, "")
			require.NoError(t, err)

			svc.RefreshThemeStyles(tt.refreshedCSS, "")
			_, err = svc.EnhanceText(context.Background(), "make a hero", textgendomain.FormatHTML, "")
			require.NoError(t, err)

			mu.Lock()
			defer mu.Unlock()
			require.Len(t, systems, 2)
			assert.Contains(t, systems[1], tt.wantInPrompt)
			assert.NotEqual(t, systems[0], systems[1])
		})
	}
}

func TestOpenAITextServiceRefreshThemeStylesConcurrent(t *testing.T) {
	var mu sync.Mutex
	var systems []string
	server := captureCompletions(t, &systems, &mu)
	defer server.Close()
	svc := textgeninfra.NewOpenAITextService("sk-fake-key", server.URL, defaultModel, "", "")
	require.NotNil(t, svc)

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for range 25 {
				if i%2 == 0 {
					_, _ = svc.EnhanceText(context.Background(), "make a hero", textgendomain.FormatHTML, "")
				} else {
					svc.RefreshThemeStyles(":root{--color-primary:#333333}", "")
				}
			}
		}(i)
	}
	wg.Wait()
}
