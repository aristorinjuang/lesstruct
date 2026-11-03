package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	speechdomain "github.com/aristorinjuang/lesstruct/modules/speech/domain"
)

const (
	// openAITranscriptionsPath is the OpenAI-compatible transcription
	// endpoint (Whisper, Groq, self-hosted servers share this shape).
	openAITranscriptionsPath = "/v1/audio/transcriptions"

	// maxOpenAIErrorSnippet bounds the provider error body kept in error
	// messages.
	maxOpenAIErrorSnippet = 4096
)

// openAITranscriptionResponse mirrors the JSON transcription response. Turn
// detail stays empty: the compact response carries the text only.
type openAITranscriptionResponse struct {
	Text string `json:"text"`
}

// buildOpenAIRequestBody assembles the multipart upload: the WAV file plus
// the model and optional language fields.
func buildOpenAIRequestBody(
	audio []byte,
	model, language string,
) (body *bytes.Buffer, contentType string, err error) {
	body = &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	audioPart, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="file"; filename="audio.wav"`},
		"Content-Type":        {"audio/wav"},
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to build transcription audio part: %w", err)
	}
	if _, err := audioPart.Write(audio); err != nil {
		return nil, "", fmt.Errorf("failed to write transcription audio part: %w", err)
	}

	if err := writer.WriteField("model", model); err != nil {
		return nil, "", fmt.Errorf("failed to write transcription model field: %w", err)
	}
	if language != "" {
		if err := writer.WriteField("language", language); err != nil {
			return nil, "", fmt.Errorf("failed to write transcription language field: %w", err)
		}
	}
	if err := writer.WriteField("response_format", "json"); err != nil {
		return nil, "", fmt.Errorf("failed to write transcription format field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to close transcription request body: %w", err)
	}
	return body, writer.FormDataContentType(), nil
}

// parseOpenAIResponse decodes the transcription JSON response. An empty text
// fails with ErrEmptyTranscript.
func parseOpenAIResponse(payload []byte) (speechdomain.Transcription, error) {
	var response openAITranscriptionResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return speechdomain.Transcription{}, fmt.Errorf(
			"failed to decode transcription response: %w: %w",
			err,
			speechdomain.ErrTranscriptionFailed,
		)
	}
	text := strings.TrimSpace(response.Text)
	if text == "" {
		return speechdomain.Transcription{}, fmt.Errorf(
			"transcription response carries no text: %w",
			speechdomain.ErrEmptyTranscript,
		)
	}
	return speechdomain.NewTranscription(text, nil), nil
}

// OpenAISpeechService transcribes audio with any OpenAI-compatible
// transcription endpoint (POST /v1/audio/transcriptions).
type OpenAISpeechService struct {
	client  *http.Client
	apiKey  string
	baseURL string
	model   string
}

var _ speechdomain.SpeechToTextService = (*OpenAISpeechService)(nil)

func (s *OpenAISpeechService) resolveClient() *http.Client {
	if s.client != nil {
		return s.client
	}
	// No client timeout: the caller's context carries the per-request
	// deadline, so a slow transcription is bounded by the handler instead.
	return &http.Client{}
}

// Transcribe uploads WAV audio to the transcription endpoint and returns the
// text. Language is an optional ISO-639-1 hint; empty means auto-detect.
func (s *OpenAISpeechService) Transcribe(
	ctx context.Context,
	audio []byte,
	language string,
) (speechdomain.Transcription, error) {
	endpoint := strings.TrimSuffix(s.baseURL, "/") + openAITranscriptionsPath

	body, contentType, err := buildOpenAIRequestBody(audio, s.model, speechdomain.NormalizeLanguage(language))
	if err != nil {
		return speechdomain.Transcription{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return speechdomain.Transcription{}, fmt.Errorf(
			"failed to build transcription request: %w: %w",
			err,
			speechdomain.ErrTranscriptionFailed,
		)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := s.resolveClient().Do(req)
	if err != nil {
		return speechdomain.Transcription{}, fmt.Errorf(
			"transcription request failed: %w: %w",
			err,
			speechdomain.ErrTranscriptionFailed,
		)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return speechdomain.Transcription{}, fmt.Errorf(
			"failed to read transcription response: %w: %w",
			err,
			speechdomain.ErrTranscriptionFailed,
		)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(payload))
		if len(snippet) > maxOpenAIErrorSnippet {
			snippet = snippet[:maxOpenAIErrorSnippet]
		}
		return speechdomain.Transcription{}, fmt.Errorf(
			"transcription API error (status %d): %s: %w",
			resp.StatusCode,
			snippet,
			speechdomain.ErrTranscriptionFailed,
		)
	}

	return parseOpenAIResponse(payload)
}

// NewOpenAISpeechService creates an OpenAI-compatible transcription client.
// baseURL is required (e.g. "https://api.openai.com",
// "http://localhost:11434/v1"). A nil httpClient gets a deadline-free
// client; the caller's context bounds the request.
func NewOpenAISpeechService(apiKey, baseURL, model string, httpClient *http.Client) *OpenAISpeechService {
	return &OpenAISpeechService{
		client:  httpClient,
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
	}
}
