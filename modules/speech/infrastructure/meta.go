// Package infrastructure adapts the speech module's domain port onto
// speech-to-text provider APIs. Transport and provider response parsing live
// here; audio validation stays in the domain's WAV validator.
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
	// defaultMetaBaseURL is the Meta Model API host. The composition root
	// normally supplies the configured base URL; the adapter falls back to
	// this when it receives an empty one.
	defaultMetaBaseURL = "https://api.meta.ai"

	// metaTranscribePath is the Meta Model API file-transcription endpoint.
	metaTranscribePath = "/v1/asr/transcribe"

	// metaAudioEncoding tells Muse the upload is a RIFF/WAVE container. The
	// admin recorder and the domain WAV validator guarantee that shape.
	metaAudioEncoding = "WAV"

	// metaPushToTalk selects single-turn transcription: the user delimits the
	// turn with the record/stop button, matching dictation.
	metaPushToTalk = "PUSH_TO_TALK"

	// maxMetaErrorSnippet bounds the provider error body kept in error
	// messages.
	maxMetaErrorSnippet = 4096
)

// metaTranscribeRequest is the JSON settings part of the multipart upload.
type metaTranscribeRequest struct {
	Model         string   `json:"model"`
	Mode          string   `json:"mode"`
	AudioEncoding string   `json:"audioEncoding"`
	LanguageBias  []string `json:"languageBias,omitempty"`
}

// metaTurn mirrors one entry of the response turns array. Word-level
// timestamps are unavailable; turn boundaries arrive in milliseconds.
type metaTurn struct {
	TurnID     int    `json:"turnId"`
	StartMs    int64  `json:"startMs"`
	EndMs      int64  `json:"endMs"`
	Transcript string `json:"transcript"`
	Speaker    string `json:"speaker"`
}

// metaTranscribeResponse mirrors the Meta transcribe JSON response.
type metaTranscribeResponse struct {
	Transcript string     `json:"transcript"`
	Turns      []metaTurn `json:"turns"`
}

// buildMetaRequestBody assembles the multipart upload: the JSON settings
// part plus the WAV audio part.
func buildMetaRequestBody(
	audio []byte,
	model, language string,
) (body *bytes.Buffer, contentType string, err error) {
	request := metaTranscribeRequest{
		Model:         model,
		Mode:          metaPushToTalk,
		AudioEncoding: metaAudioEncoding,
	}
	if language != "" {
		request.LanguageBias = []string{language}
	}
	settings, err := json.Marshal(request)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encode transcribe settings: %w", err)
	}

	body = &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	settingsPart, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="request"`},
		"Content-Type":        {"application/json"},
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to build transcribe settings part: %w", err)
	}
	if _, err := settingsPart.Write(settings); err != nil {
		return nil, "", fmt.Errorf("failed to write transcribe settings part: %w", err)
	}

	audioPart, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="audio"; filename="audio.wav"`},
		"Content-Type":        {"audio/wav"},
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to build transcribe audio part: %w", err)
	}
	if _, err := audioPart.Write(audio); err != nil {
		return nil, "", fmt.Errorf("failed to write transcribe audio part: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to close transcribe request body: %w", err)
	}
	return body, writer.FormDataContentType(), nil
}

// parseMetaResponse decodes the provider response into a Transcription. An
// empty transcript text fails with ErrEmptyTranscript.
func parseMetaResponse(payload []byte) (speechdomain.Transcription, error) {
	var response metaTranscribeResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return speechdomain.Transcription{}, fmt.Errorf(
			"failed to decode transcribe response: %w: %w",
			err,
			speechdomain.ErrTranscriptionFailed,
		)
	}
	text := strings.TrimSpace(response.Transcript)
	if text == "" {
		return speechdomain.Transcription{}, fmt.Errorf(
			"transcribe response carries no text: %w",
			speechdomain.ErrEmptyTranscript,
		)
	}
	turns := make([]speechdomain.Turn, 0, len(response.Turns))
	for _, turn := range response.Turns {
		turns = append(turns, speechdomain.NewTurn(
			turn.TurnID,
			turn.StartMs,
			turn.EndMs,
			strings.TrimSpace(turn.Transcript),
			strings.TrimSpace(turn.Speaker),
		))
	}
	return speechdomain.NewTranscription(text, turns), nil
}

// MetaSpeechService transcribes audio with Meta Model API's Muse Voice
// Transcribe model over the multipart file endpoint.
type MetaSpeechService struct {
	client  *http.Client
	apiKey  string
	baseURL string
	model   string
}

var _ speechdomain.SpeechToTextService = (*MetaSpeechService)(nil)

func (s *MetaSpeechService) resolveClient() *http.Client {
	if s.client != nil {
		return s.client
	}
	// No client timeout: the caller's context carries the per-request
	// deadline, so a slow transcription is bounded by the handler instead.
	return &http.Client{}
}

// Transcribe uploads WAV audio to Meta Model API and returns the transcript
// with its detected turns. Language is an optional bias hint; empty means
// auto-detect.
func (s *MetaSpeechService) Transcribe(
	ctx context.Context,
	audio []byte,
	language string,
) (speechdomain.Transcription, error) {
	baseURL := s.baseURL
	if baseURL == "" {
		baseURL = defaultMetaBaseURL
	}
	endpoint := strings.TrimSuffix(baseURL, "/") + metaTranscribePath

	body, contentType, err := buildMetaRequestBody(audio, s.model, speechdomain.NormalizeLanguage(language))
	if err != nil {
		return speechdomain.Transcription{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return speechdomain.Transcription{}, fmt.Errorf(
			"failed to build transcribe request: %w: %w",
			err,
			speechdomain.ErrTranscriptionFailed,
		)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := s.resolveClient().Do(req)
	if err != nil {
		return speechdomain.Transcription{}, fmt.Errorf(
			"transcribe request failed: %w: %w",
			err,
			speechdomain.ErrTranscriptionFailed,
		)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return speechdomain.Transcription{}, fmt.Errorf(
			"failed to read transcribe response: %w: %w",
			err,
			speechdomain.ErrTranscriptionFailed,
		)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(payload))
		if len(snippet) > maxMetaErrorSnippet {
			snippet = snippet[:maxMetaErrorSnippet]
		}
		return speechdomain.Transcription{}, fmt.Errorf(
			"transcribe API error (status %d): %s: %w",
			resp.StatusCode,
			snippet,
			speechdomain.ErrTranscriptionFailed,
		)
	}

	return parseMetaResponse(payload)
}

// NewMetaSpeechService creates a Muse Voice Transcribe client. baseURL is
// optional — pass "" for the Meta Model API default. A nil httpClient gets a
// deadline-free client; the caller's context bounds the request.
func NewMetaSpeechService(apiKey, baseURL, model string, httpClient *http.Client) *MetaSpeechService {
	return &MetaSpeechService{
		client:  httpClient,
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
	}
}
