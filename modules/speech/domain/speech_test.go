package speech_test

import (
	"encoding/binary"
	"testing"

	speech "github.com/aristorinjuang/lesstruct/modules/speech/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestWAV builds a minimal mono 16-bit PCM WAV with a 160-byte data
// payload. Callers mutate the returned bytes for negative cases.
func newTestWAV(sampleRate uint32) []byte {
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

func TestNewTurn(t *testing.T) {
	tests := []struct {
		name       string
		turnID     int
		startMs    int64
		endMs      int64
		transcript string
		speaker    string
	}{
		{
			name:       "success - single turn without speaker",
			turnID:     1,
			startMs:    1520,
			endMs:      4640,
			transcript: "How is the weather?",
			speaker:    "",
		},
		{
			name:       "success - diarized turn with speaker",
			turnID:     2,
			startMs:    5900,
			endMs:      8240,
			transcript: "It is raining.",
			speaker:    "B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			turn := speech.NewTurn(tt.turnID, tt.startMs, tt.endMs, tt.transcript, tt.speaker)
			assert.Equal(t, tt.turnID, turn.TurnID)
			assert.Equal(t, tt.startMs, turn.StartMs)
			assert.Equal(t, tt.endMs, turn.EndMs)
			assert.Equal(t, tt.transcript, turn.Transcript)
			assert.Equal(t, tt.speaker, turn.Speaker)
		})
	}
}

func TestNewTranscription(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		turns []speech.Turn
	}{
		{
			name:  "success - text without turns",
			text:  "How is the weather?",
			turns: nil,
		},
		{
			name: "success - text with turns",
			text: "How is the weather? It is raining.",
			turns: []speech.Turn{
				speech.NewTurn(1, 1520, 4640, "How is the weather?", "A"),
				speech.NewTurn(2, 5900, 8240, "It is raining.", "B"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := speech.NewTranscription(tt.text, tt.turns)
			assert.Equal(t, tt.text, result.Text)
			assert.Equal(t, tt.turns, result.Turns)
		})
	}
}

func TestValidateWAV(t *testing.T) {
	stereo := newTestWAV(16000)
	binary.LittleEndian.PutUint16(stereo[22:24], 2)

	eightBit := newTestWAV(16000)
	binary.LittleEndian.PutUint16(eightBit[34:36], 8)

	wrongRate := newTestWAV(44100)

	badMagic := newTestWAV(16000)
	copy(badMagic[0:4], "RIFX")

	noData := newTestWAV(16000)[:36]

	tests := []struct {
		name    string
		audio   []byte
		wantErr bool
		errIs   error
	}{
		{
			name:    "success - 16 kHz mono PCM",
			audio:   newTestWAV(16000),
			wantErr: false,
		},
		{
			name:    "success - 24 kHz mono PCM",
			audio:   newTestWAV(24000),
			wantErr: false,
		},
		{
			name:    "error - empty input",
			audio:   nil,
			wantErr: true,
			errIs:   speech.ErrInvalidAudio,
		},
		{
			name:    "error - truncated header",
			audio:   newTestWAV(16000)[:20],
			wantErr: true,
			errIs:   speech.ErrInvalidAudio,
		},
		{
			name:    "error - bad RIFF magic",
			audio:   badMagic,
			wantErr: true,
			errIs:   speech.ErrInvalidAudio,
		},
		{
			name:    "error - stereo is rejected",
			audio:   stereo,
			wantErr: true,
			errIs:   speech.ErrInvalidAudio,
		},
		{
			name:    "error - 8-bit is rejected",
			audio:   eightBit,
			wantErr: true,
			errIs:   speech.ErrInvalidAudio,
		},
		{
			name:    "error - unsupported sample rate is rejected",
			audio:   wrongRate,
			wantErr: true,
			errIs:   speech.ErrInvalidAudio,
		},
		{
			name:    "error - missing data chunk is rejected",
			audio:   noData,
			wantErr: true,
			errIs:   speech.ErrInvalidAudio,
		},
		{
			name:    "error - oversize audio is rejected",
			audio:   make([]byte, speech.MaxAudioBytes+1),
			wantErr: true,
			errIs:   speech.ErrAudioTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := speech.ValidateWAV(tt.audio)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.errIs)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNormalizeLanguage(t *testing.T) {
	tests := []struct {
		name     string
		language string
		expected string
	}{
		{
			name:     "success - empty stays empty for auto-detect",
			language: "",
			expected: "",
		},
		{
			name:     "success - whitespace is trimmed",
			language: "  en  ",
			expected: "en",
		},
		{
			name:     "success - overlong hint is dropped",
			language: "this language hint is far too long to forward",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, speech.NormalizeLanguage(tt.language))
		})
	}
}
