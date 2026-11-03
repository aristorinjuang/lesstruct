package infrastructure_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	speechdomain "github.com/aristorinjuang/lesstruct/modules/speech/domain"
	speechinfra "github.com/aristorinjuang/lesstruct/modules/speech/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenAISpeechService(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		baseURL string
		model   string
	}{
		{
			name:    "success - openai endpoint",
			apiKey:  "sk-key",
			baseURL: "https://api.openai.com",
			model:   "whisper-1",
		},
		{
			name:    "success - self-hosted endpoint",
			apiKey:  "ollama",
			baseURL: "http://localhost:11434/v1",
			model:   "whisper",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := speechinfra.NewOpenAISpeechService(tt.apiKey, tt.baseURL, tt.model, nil)
			require.NotNil(t, svc)

			var iface speechdomain.SpeechToTextService = svc
			assert.NotNil(t, iface)
		})
	}
}

func TestOpenAISpeechService_Transcribe(t *testing.T) {
	const successResponse = `{"text":"How is the weather?"}`

	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		language       string
		cancelContext  bool
		wantErr        bool
		errIs          error
		validateResult func(*testing.T, speechdomain.Transcription)
	}{
		{
			name:         "success - transcript without turns",
			statusCode:   http.StatusOK,
			responseBody: successResponse,
			language:     "en",
			wantErr:      false,
			validateResult: func(t *testing.T, result speechdomain.Transcription) {
				assert.Equal(t, "How is the weather?", result.Text)
				assert.Empty(t, result.Turns)
			},
		},
		{
			name:         "error - provider rejection maps to transcription failure",
			statusCode:   http.StatusUnauthorized,
			responseBody: `{"error":{"message":"invalid api key"}}`,
			wantErr:      true,
			errIs:        speechdomain.ErrTranscriptionFailed,
		},
		{
			name:         "error - empty text maps to empty transcript",
			statusCode:   http.StatusOK,
			responseBody: `{"text":"  "}`,
			wantErr:      true,
			errIs:        speechdomain.ErrEmptyTranscript,
		},
		{
			name:         "error - malformed response maps to transcription failure",
			statusCode:   http.StatusOK,
			responseBody: `not-json`,
			wantErr:      true,
			errIs:        speechdomain.ErrTranscriptionFailed,
		},
		{
			name:          "error - cancelled context fails",
			statusCode:    http.StatusOK,
			responseBody:  successResponse,
			cancelContext: true,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/v1/audio/transcriptions", r.URL.Path)
				assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

				require.NoError(t, r.ParseMultipartForm(1<<20))
				assert.Equal(t, "whisper-1", r.FormValue("model"))
				assert.Equal(t, "json", r.FormValue("response_format"))

				audioFile, _, err := r.FormFile("file")
				require.NoError(t, err)
				uploaded, err := io.ReadAll(audioFile)
				require.NoError(t, err)
				require.NoError(t, speechdomain.ValidateWAV(uploaded))

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			svc := speechinfra.NewOpenAISpeechService("test-key", server.URL, "whisper-1", server.Client())

			ctx := context.Background()
			if tt.cancelContext {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			result, err := svc.Transcribe(ctx, newTestWAVBytes(16000), tt.language)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errIs != nil {
					assert.ErrorIs(t, err, tt.errIs)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, tt.validateResult)
			tt.validateResult(t, result)
		})
	}
}

func TestOpenAISpeechService_Transcribe_OmitsEmptyLanguage(t *testing.T) {
	tests := []struct {
		name           string
		language       string
		expectLanguage bool
	}{
		{
			name:           "success - hint is forwarded",
			language:       "en",
			expectLanguage: true,
		},
		{
			name:           "success - empty hint is omitted for auto-detect",
			language:       "",
			expectLanguage: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, r.ParseMultipartForm(1<<20))
				_, present := r.MultipartForm.Value["language"]
				assert.Equal(t, tt.expectLanguage, present)
				if tt.expectLanguage {
					assert.Equal(t, "en", r.FormValue("language"))
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"text":"Hello world."}`))
			}))
			defer server.Close()

			svc := speechinfra.NewOpenAISpeechService("test-key", server.URL, "whisper-1", server.Client())

			result, err := svc.Transcribe(context.Background(), newTestWAVBytes(16000), tt.language)
			require.NoError(t, err)
			assert.Equal(t, "Hello world.", result.Text)
		})
	}
}
