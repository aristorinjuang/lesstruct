package api_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/speech/api"
	apimocks "github.com/aristorinjuang/lesstruct/modules/speech/api/mocks"
	speechdomain "github.com/aristorinjuang/lesstruct/modules/speech/domain"
	textgendomain "github.com/aristorinjuang/lesstruct/modules/textgen/domain"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newTestWAVBytes builds a minimal mono 16-bit PCM WAV the domain validator
// accepts, so handler tests exercise the wire with realistic uploads.
func newTestWAVBytes(sampleRate uint32) []byte {
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], 196)
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], 1)
	binary.LittleEndian.PutUint32(header[24:28], sampleRate)
	binary.LittleEndian.PutUint32(header[28:32], sampleRate*2)
	binary.LittleEndian.PutUint16(header[32:34], 2)
	binary.LittleEndian.PutUint16(header[34:36], 16)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], 160)
	return append(header, make([]byte, 160)...)
}

// buildTranscribeBody builds the multipart dictation upload: the WAV audio
// part plus the optional language field.
func buildTranscribeBody(t *testing.T, audio []byte, includeAudio bool, language string) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if includeAudio {
		audioPart, err := writer.CreateFormFile("audio", "audio.wav")
		require.NoError(t, err)
		_, err = audioPart.Write(audio)
		require.NoError(t, err)
	}
	if language != "" {
		require.NoError(t, writer.WriteField("language", language))
	}
	require.NoError(t, writer.Close())
	return body, writer.FormDataContentType()
}

func validTiptapDoc() string {
	return `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello world"}]}]}`
}

func TestSpeechHandler_Transcribe(t *testing.T) {
	tests := []struct {
		name           string
		audio          []byte
		includeAudio   bool
		language       string
		contentType    string
		setupMocks     func(transcriber *apimocks.MockSpeechTranscriber, enhancer *apimocks.MockTranscriptionEnhancer)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:         "successful transcribe with enhancement",
			audio:        newTestWAVBytes(16000),
			includeAudio: true,
			language:     "en",
			setupMocks: func(transcriber *apimocks.MockSpeechTranscriber, enhancer *apimocks.MockTranscriptionEnhancer) {
				transcriber.EXPECT().Transcribe(mock.Anything, mock.Anything, "en").Return(
					speechdomain.NewTranscription("heading two hello world", []speechdomain.Turn{
						speechdomain.NewTurn(1, 0, 2000, "heading two hello world", ""),
					}),
					nil,
				)
				enhancer.EXPECT().EnhanceTranscription(mock.Anything, "heading two hello world").Return(
					textgendomain.NewEnhanceResult(validTiptapDoc(), "Hello World", "A dictated hello."),
					nil,
				)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				require.True(t, ok, "expected data field")
				require.Equal(t, "heading two hello world", data["transcript"])
				require.Equal(t, validTiptapDoc(), data["content"])
				require.Equal(t, "Hello World", data["title"])
				require.Equal(t, "A dictated hello.", data["metaDescription"])
			},
		},
		{
			name:         "error - non-multipart body is rejected",
			audio:        nil,
			includeAudio: false,
			contentType:  "application/json",
			setupMocks: func(transcriber *apimocks.MockSpeechTranscriber, enhancer *apimocks.MockTranscriptionEnhancer) {
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				errField, ok := resp["error"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, "invalid_request", errField["code"])
			},
		},
		{
			name:         "error - missing audio part",
			audio:        nil,
			includeAudio: false,
			setupMocks: func(transcriber *apimocks.MockSpeechTranscriber, enhancer *apimocks.MockTranscriptionEnhancer) {
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				errField, ok := resp["error"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, "invalid_audio", errField["code"])
			},
		},
		{
			name:         "error - non-wav audio is rejected",
			audio:        []byte("not a wav file at all"),
			includeAudio: true,
			setupMocks: func(transcriber *apimocks.MockSpeechTranscriber, enhancer *apimocks.MockTranscriptionEnhancer) {
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				errField, ok := resp["error"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, "invalid_audio", errField["code"])
			},
		},
		{
			name:         "error - transcription failure maps to bad gateway",
			audio:        newTestWAVBytes(16000),
			includeAudio: true,
			setupMocks: func(transcriber *apimocks.MockSpeechTranscriber, enhancer *apimocks.MockTranscriptionEnhancer) {
				transcriber.EXPECT().Transcribe(mock.Anything, mock.Anything, "").Return(
					speechdomain.Transcription{},
					errors.New("provider exploded"),
				)
			},
			expectedStatus: http.StatusBadGateway,
			validateResp: func(t *testing.T, resp map[string]any) {
				errField, ok := resp["error"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, "transcription_failed", errField["code"])
			},
		},
		{
			name:         "error - empty transcript reports no speech",
			audio:        newTestWAVBytes(16000),
			includeAudio: true,
			setupMocks: func(transcriber *apimocks.MockSpeechTranscriber, enhancer *apimocks.MockTranscriptionEnhancer) {
				transcriber.EXPECT().Transcribe(mock.Anything, mock.Anything, "").Return(
					speechdomain.Transcription{},
					speechdomain.ErrEmptyTranscript,
				)
			},
			expectedStatus: http.StatusBadGateway,
			validateResp: func(t *testing.T, resp map[string]any) {
				errField, ok := resp["error"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, "transcription_failed", errField["code"])
				require.Equal(t, "No speech detected in the audio", errField["message"])
			},
		},
		{
			name:         "error - enhancement failure maps to generation failed",
			audio:        newTestWAVBytes(16000),
			includeAudio: true,
			setupMocks: func(transcriber *apimocks.MockSpeechTranscriber, enhancer *apimocks.MockTranscriptionEnhancer) {
				transcriber.EXPECT().Transcribe(mock.Anything, mock.Anything, "").Return(
					speechdomain.NewTranscription("hello world", nil),
					nil,
				)
				enhancer.EXPECT().EnhanceTranscription(mock.Anything, "hello world").Return(
					textgendomain.EnhanceResult{},
					errors.New("model exploded"),
				)
			},
			expectedStatus: http.StatusInternalServerError,
			validateResp: func(t *testing.T, resp map[string]any) {
				errField, ok := resp["error"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, "generation_failed", errField["code"])
			},
		},
		{
			name:         "error - overlong transcript is rejected",
			audio:        newTestWAVBytes(16000),
			includeAudio: true,
			setupMocks: func(transcriber *apimocks.MockSpeechTranscriber, enhancer *apimocks.MockTranscriptionEnhancer) {
				transcriber.EXPECT().Transcribe(mock.Anything, mock.Anything, "").Return(
					speechdomain.NewTranscription(strings.Repeat("word ", 20000), nil),
					nil,
				)
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				errField, ok := resp["error"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, "invalid_content", errField["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transcriber := apimocks.NewMockSpeechTranscriber(t)
			enhancer := apimocks.NewMockTranscriptionEnhancer(t)
			tt.setupMocks(transcriber, enhancer)

			handler := api.NewSpeechHandler(transcriber, enhancer, nil)

			body, contentType := buildTranscribeBody(t, tt.audio, tt.includeAudio, tt.language)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/speech/transcribe", body)
			req.Header.Set("Content-Type", contentType)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			ctx := kernel.ContextWithPrincipal(req.Context(), kernel.Principal{ID: "1", Username: "testuser"})
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.Transcribe(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			var resp map[string]any
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}

func TestSpeechHandler_Transcribe_Unauthorized(t *testing.T) {
	transcriber := apimocks.NewMockSpeechTranscriber(t)
	enhancer := apimocks.NewMockTranscriptionEnhancer(t)

	handler := api.NewSpeechHandler(transcriber, enhancer, nil)

	body, contentType := buildTranscribeBody(t, newTestWAVBytes(16000), true, "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/speech/transcribe", body)
	req.Header.Set("Content-Type", contentType)

	// No principal in context.

	w := httptest.NewRecorder()
	handler.Transcribe(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]any
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	errField, ok := resp["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "unauthorized", errField["code"])
}

func TestSpeechHandler_Transcribe_OversizeAudio(t *testing.T) {
	transcriber := apimocks.NewMockSpeechTranscriber(t)
	enhancer := apimocks.NewMockTranscriptionEnhancer(t)

	handler := api.NewSpeechHandler(transcriber, enhancer, nil)

	oversize := make([]byte, speechdomain.MaxAudioBytes+1)
	copy(oversize, newTestWAVBytes(16000))
	body, contentType := buildTranscribeBody(t, oversize, true, "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/speech/transcribe", body)
	req.Header.Set("Content-Type", contentType)

	ctx := kernel.ContextWithPrincipal(req.Context(), kernel.Principal{ID: "1", Username: "testuser"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Transcribe(w, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	var resp map[string]any
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	errField, ok := resp["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "audio_too_large", errField["code"])
}
