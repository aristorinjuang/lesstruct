// Package speech is the voice-dictation module's domain layer: the
// speech-to-text port, its result value types, and the WAV validator that
// keeps unbilled garbage audio from reaching the provider. Provider adapters
// (Meta Model API, OpenAI-compatible transcription) live in the module's
// infrastructure package and implement the port.
package speech

import (
	"context"
	"encoding/binary"
	"errors"
	"strings"
)

const (
	// MaxAudioBytes bounds a dictation upload: five minutes of 16 kHz mono
	// 16-bit PCM (≈9.6 MB with the WAV header) fit, anything larger is
	// rejected before it reaches the provider.
	MaxAudioBytes = 10 << 20

	// minWAVSize is the smallest byte length a WAV file can have: the 12-byte
	// RIFF header plus one 8-byte chunk header plus a 16-byte fmt payload.
	minWAVSize = 36

	// supportedSampleRate16K and supportedSampleRate24K are the sample rates
	// both bundled providers accept: Meta's engine-native 24 kHz and the
	// 16 kHz rate the admin recorder captures at.
	supportedSampleRate16K = 16000
	supportedSampleRate24K = 24000
)

var (
	// ErrInvalidAudio reports audio that is not a mono 16-bit PCM WAV file
	// at a supported sample rate.
	ErrInvalidAudio = errors.New("audio must be a mono 16-bit PCM WAV file")

	// ErrAudioTooLarge reports an upload beyond MaxAudioBytes.
	ErrAudioTooLarge = errors.New("audio exceeds the maximum dictation length")

	// ErrTranscriptionFailed reports a provider-side transcription failure.
	ErrTranscriptionFailed = errors.New("speech transcription failed")

	// ErrEmptyTranscript reports a provider response that carries no text.
	ErrEmptyTranscript = errors.New("speech transcription returned no text")
)

// Turn is a single detected speech segment. Speaker is populated in
// diarization-style responses and empty for single-turn dictation.
type Turn struct {
	TurnID     int
	StartMs    int64
	EndMs      int64
	Transcript string
	Speaker    string
}

// NewTurn assembles a speech turn from its boundaries and text.
func NewTurn(
	turnID int,
	startMs, endMs int64,
	transcript, speaker string,
) Turn {
	return Turn{
		TurnID:     turnID,
		StartMs:    startMs,
		EndMs:      endMs,
		Transcript: transcript,
		Speaker:    speaker,
	}
}

// Transcription is the outcome of transcribing dictated audio: the full text
// plus the detected turns behind it.
type Transcription struct {
	Text  string
	Turns []Turn
}

// NewTranscription assembles a transcription outcome from its text and turns.
func NewTranscription(text string, turns []Turn) Transcription {
	return Transcription{
		Text:  text,
		Turns: turns,
	}
}

// ValidateWAV rejects anything that is not a mono 16-bit PCM WAV file at a
// supported sample rate, or that exceeds MaxAudioBytes. It parses the RIFF
// envelope and the fmt chunk instead of trusting extensions or MIME types.
func ValidateWAV(audio []byte) error {
	if len(audio) > MaxAudioBytes {
		return ErrAudioTooLarge
	}
	if len(audio) < minWAVSize {
		return ErrInvalidAudio
	}
	if string(audio[0:4]) != "RIFF" || string(audio[8:12]) != "WAVE" {
		return ErrInvalidAudio
	}
	foundFmt := false
	foundData := false
	for offset := 12; offset+8 <= len(audio); {
		chunkID := string(audio[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(audio[offset+4 : offset+8]))
		bodyStart := offset + 8
		if bodyStart+chunkSize > len(audio) {
			return ErrInvalidAudio
		}
		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return ErrInvalidAudio
			}
			format := binary.LittleEndian.Uint16(audio[bodyStart : bodyStart+2])
			channels := binary.LittleEndian.Uint16(audio[bodyStart+2 : bodyStart+4])
			sampleRate := binary.LittleEndian.Uint32(audio[bodyStart+4 : bodyStart+8])
			bits := binary.LittleEndian.Uint16(audio[bodyStart+14 : bodyStart+16])
			if format != 1 || channels != 1 || bits != 16 {
				return ErrInvalidAudio
			}
			if sampleRate != supportedSampleRate16K && sampleRate != supportedSampleRate24K {
				return ErrInvalidAudio
			}
			foundFmt = true
		case "data":
			foundData = true
		}
		offset = bodyStart + chunkSize
		// Chunks are word-aligned: skip the pad byte after an odd-sized body.
		if chunkSize%2 == 1 {
			offset++
		}
		if foundFmt && foundData {
			return nil
		}
	}
	if !foundFmt || !foundData {
		return ErrInvalidAudio
	}
	return nil
}

// NormalizeLanguage trims a language hint for the provider request. An empty
// hint means auto-detect and stays empty; overlong values are dropped to
// empty rather than forwarded.
func NormalizeLanguage(language string) string {
	trimmed := strings.TrimSpace(language)
	if len(trimmed) > 32 {
		return ""
	}
	return trimmed
}

// SpeechToTextService defines the interface for speech-to-text providers.
type SpeechToTextService interface {
	Transcribe(
		ctx context.Context,
		audio []byte,
		language string,
	) (Transcription, error)
}
