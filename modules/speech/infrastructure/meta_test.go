package infrastructure_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	speechdomain "github.com/aristorinjuang/lesstruct/modules/speech/domain"
	speechinfra "github.com/aristorinjuang/lesstruct/modules/speech/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestWAVBytes builds a minimal mono 16-bit PCM WAV the domain validator
// accepts, so adapter tests exercise the wire with realistic uploads.
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

func TestNewMetaSpeechService(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		baseURL string
		model   string
	}{
		{
			name:    "success - meta model api",
			apiKey:  "meta-key",
			baseURL: "https://api.meta.ai",
			model:   "muse-voice-transcribe-1.0",
		},
		{
			name:    "success - empty base url falls back to default",
			apiKey:  "meta-key",
			baseURL: "",
			model:   "muse-voice-transcribe-1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := speechinfra.NewMetaSpeechService(tt.apiKey, tt.baseURL, tt.model, nil)
			require.NotNil(t, svc)

			var iface speechdomain.SpeechToTextService = svc
			assert.NotNil(t, iface)
		})
	}
}

func TestMetaSpeechService_Transcribe(t *testing.T) {
	const successResponse = `{"sessionId":"test-session","transcript":"How is the weather? It is raining.","audioDurationMs":8240,"turns":[{"turnId":1,"startMs":1520,"endMs":4640,"transcript":"How is the weather?","speaker":"A"},{"turnId":2,"startMs":5900,"endMs":8240,"transcript":"It is raining.","speaker":"B"}]}`

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
			name:         "success - transcript with turns",
			statusCode:   http.StatusOK,
			responseBody: successResponse,
			language:     "",
			wantErr:      false,
			validateResult: func(t *testing.T, result speechdomain.Transcription) {
				assert.Equal(t, "How is the weather? It is raining.", result.Text)
				require.Len(t, result.Turns, 2)
				assert.Equal(t, 1, result.Turns[0].TurnID)
				assert.Equal(t, "How is the weather?", result.Turns[0].Transcript)
				assert.Equal(t, "A", result.Turns[0].Speaker)
				assert.Equal(t, 2, result.Turns[1].TurnID)
				assert.Equal(t, "B", result.Turns[1].Speaker)
			},
		},
		{
			name:         "error - provider rejection maps to transcription failure",
			statusCode:   http.StatusBadRequest,
			responseBody: `{"error":"unsupported audio"}`,
			wantErr:      true,
			errIs:        speechdomain.ErrTranscriptionFailed,
		},
		{
			name:         "error - empty transcript maps to empty transcript",
			statusCode:   http.StatusOK,
			responseBody: `{"sessionId":"test-session","transcript":"  ","turns":[]}`,
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
				assert.Equal(t, "/v1/asr/transcribe", r.URL.Path)
				assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

				require.NoError(t, r.ParseMultipartForm(1<<20))
				settingsJSON := r.FormValue("request")
				require.NotEmpty(t, settingsJSON)
				var settings map[string]any
				require.NoError(t, json.Unmarshal([]byte(settingsJSON), &settings))
				assert.Equal(t, "muse-voice-transcribe-1.0", settings["model"])
				assert.Equal(t, "PUSH_TO_TALK", settings["mode"])
				assert.Equal(t, "WAV", settings["audioEncoding"])

				audioFile, _, err := r.FormFile("audio")
				require.NoError(t, err)
				uploaded, err := io.ReadAll(audioFile)
				require.NoError(t, err)
				require.NoError(t, speechdomain.ValidateWAV(uploaded))

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			svc := speechinfra.NewMetaSpeechService("test-key", server.URL, "muse-voice-transcribe-1.0", server.Client())

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

func TestMetaSpeechService_Transcribe_ForwardsLanguageBias(t *testing.T) {
	tests := []struct {
		name             string
		language         string
		expectBias       bool
		expectedLanguage string
	}{
		{
			name:             "success - hint becomes single-element bias",
			language:         "English",
			expectBias:       true,
			expectedLanguage: "English",
		},
		{
			name:       "success - empty hint omits bias for auto-detect",
			language:   "",
			expectBias: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, r.ParseMultipartForm(1<<20))
				var settings map[string]any
				require.NoError(t, json.Unmarshal([]byte(r.FormValue("request")), &settings))
				bias, present := settings["languageBias"]
				assert.Equal(t, tt.expectBias, present)
				if tt.expectBias {
					biasList, ok := bias.([]any)
					require.True(t, ok)
					require.Len(t, biasList, 1)
					assert.Equal(t, tt.expectedLanguage, biasList[0])
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"transcript":"Hello world."}`))
			}))
			defer server.Close()

			svc := speechinfra.NewMetaSpeechService("test-key", server.URL, "muse-voice-transcribe-1.0", server.Client())

			result, err := svc.Transcribe(context.Background(), newTestWAVBytes(24000), tt.language)
			require.NoError(t, err)
			assert.Equal(t, "Hello world.", result.Text)
		})
	}
}
