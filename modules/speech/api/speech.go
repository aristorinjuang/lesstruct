package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/aristorinjuang/lesstruct/kernel"
	speechdomain "github.com/aristorinjuang/lesstruct/modules/speech/domain"
	textgendomain "github.com/aristorinjuang/lesstruct/modules/textgen/domain"
)

const (
	// maxSpeechFormMemory is the multipart memory budget before overflow
	// spills to temp files. The audio part always spills; only small text
	// fields stay in memory.
	maxSpeechFormMemory = 1 << 20

	// maxTranscriptRunes caps the transcript forwarded to the text model,
	// mirroring the text generation surface's prompt ceiling.
	maxTranscriptRunes = 50000

	// transcribeTimeout bounds the provider transcription call. Five minutes
	// of audio transcribe well inside it; the frontend waits longer.
	transcribeTimeout = 3 * time.Minute

	// enhanceTimeout bounds the transcript-enhancement call, mirroring the
	// text generation surface's deadline.
	enhanceTimeout = 2 * time.Minute
)

// SpeechTranscriber is the speech-to-text port the handler transcribes
// uploads with. Provider adapters implement it in infrastructure.
type SpeechTranscriber interface {
	Transcribe(
		ctx context.Context,
		audio []byte,
		language string,
	) (speechdomain.Transcription, error)
}

// TranscriptionEnhancer turns a raw transcript into a polished TipTap
// document. The text generation service satisfies it structurally once the
// dictation prompt ships.
type TranscriptionEnhancer interface {
	EnhanceTranscription(
		ctx context.Context,
		transcript string,
	) (textgendomain.EnhanceResult, error)
}

// speechTranscribeResponse is the JSON body for the Transcribe endpoint:
// the raw transcript plus the enhanced TipTap content and SEO metadata.
type speechTranscribeResponse struct {
	Transcript      string `json:"transcript"`
	Content         string `json:"content"`
	Title           string `json:"title,omitempty"`
	MetaDescription string `json:"metaDescription,omitempty"`
}

// discardLogger drops log lines. It backs the nil-logger fallback in
// NewSpeechHandler so the handler is safe to construct in any context.
type discardLogger struct{}

func (discardLogger) Info(string, ...any) {}

func (discardLogger) Error(string, ...any) {}

// SpeechHandler handles voice-dictation transcription requests.
type SpeechHandler struct {
	transcriber SpeechTranscriber
	enhancer    TranscriptionEnhancer
	logger      kernel.Logger
}

// Transcribe handles the "Dictate with AI" endpoint: it transcribes the
// uploaded WAV, enhances the transcript into TipTap content, and returns
// both plus SEO metadata.
func (h *SpeechHandler) Transcribe(w http.ResponseWriter, r *http.Request) {
	if _, ok := kernel.PrincipalFromContext(r.Context()); !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	if err := r.ParseMultipartForm(maxSpeechFormMemory); err != nil {
		h.logger.Error("Failed to parse transcribe form: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}
	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
	}

	file, _, err := r.FormFile("audio")
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_audio", "Audio file is required", nil)
		return
	}
	defer func() { _ = file.Close() }()

	audio, err := io.ReadAll(io.LimitReader(file, speechdomain.MaxAudioBytes+1))
	if err != nil {
		h.logger.Error("Failed to read transcribe audio: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_audio", "Audio file is required", nil)
		return
	}
	if len(audio) > speechdomain.MaxAudioBytes {
		sendErrorResponse(w, http.StatusRequestEntityTooLarge, "audio_too_large", "Audio exceeds the maximum dictation length", nil)
		return
	}
	if err := speechdomain.ValidateWAV(audio); err != nil {
		if errors.Is(err, speechdomain.ErrAudioTooLarge) {
			sendErrorResponse(w, http.StatusRequestEntityTooLarge, "audio_too_large", "Audio exceeds the maximum dictation length", nil)
			return
		}
		sendErrorResponse(w, http.StatusBadRequest, "invalid_audio", "Audio must be a mono 16-bit PCM WAV file", nil)
		return
	}

	language := speechdomain.NormalizeLanguage(r.FormValue("language"))

	transcribeCtx, transcribeCancel := context.WithTimeout(context.WithoutCancel(r.Context()), transcribeTimeout)
	defer transcribeCancel()

	transcription, err := h.transcriber.Transcribe(transcribeCtx, audio, language)
	if err != nil {
		h.logger.Error("Failed to transcribe audio: %v", err)
		if errors.Is(err, speechdomain.ErrEmptyTranscript) {
			sendErrorResponse(w, http.StatusBadGateway, "transcription_failed", "No speech detected in the audio", nil)
			return
		}
		sendErrorResponse(w, http.StatusBadGateway, "transcription_failed", "Failed to transcribe audio", nil)
		return
	}

	transcript := transcription.Text
	if utf8.RuneCountInString(transcript) > maxTranscriptRunes {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content", "Transcript is too long", nil)
		return
	}

	enhanceCtx, enhanceCancel := context.WithTimeout(context.WithoutCancel(r.Context()), enhanceTimeout)
	defer enhanceCancel()

	enhanced, err := h.enhancer.EnhanceTranscription(enhanceCtx, transcript)
	if err != nil {
		h.logger.Error("Failed to enhance transcription: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "generation_failed", "Failed to enhance transcription", nil)
		return
	}

	sendSuccessResponse(w, http.StatusOK, &speechTranscribeResponse{
		Transcript:      transcript,
		Content:         enhanced.Content,
		Title:           enhanced.Title,
		MetaDescription: enhanced.MetaDescription,
	})
}

// NewSpeechHandler creates a new SpeechHandler. A nil logger degrades to a
// discard sink so the handler is safe to construct in any context.
func NewSpeechHandler(
	transcriber SpeechTranscriber,
	enhancer TranscriptionEnhancer,
	logger kernel.Logger,
) *SpeechHandler {
	if logger == nil {
		logger = discardLogger{}
	}
	return &SpeechHandler{
		transcriber: transcriber,
		enhancer:    enhancer,
		logger:      logger,
	}
}
