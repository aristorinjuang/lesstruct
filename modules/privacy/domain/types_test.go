package privacy_test

import (
	"image"
	"testing"

	privacy "github.com/aristorinjuang/lesstruct/modules/privacy/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDetection(t *testing.T) {
	box := image.Rect(1, 2, 10, 20)
	got := privacy.NewDetection(privacy.ConceptFace, box)
	assert.Equal(t, privacy.ConceptFace, got.Concept)
	assert.Equal(t, box, got.Box)
}

func TestParseConcepts(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected []string
	}{
		{
			name:     "empty returns defaults",
			raw:      "",
			expected: []string{privacy.ConceptFace, privacy.ConceptLicensePlate, privacy.ConceptNudity},
		},
		{
			name:     "blank returns defaults",
			raw:      "   ",
			expected: []string{privacy.ConceptFace, privacy.ConceptLicensePlate, privacy.ConceptNudity},
		},
		{
			name:     "single concept",
			raw:      "face",
			expected: []string{"face"},
		},
		{
			name:     "trims and lowercases with dedup",
			raw:      " Face, FACE , license plate,, nudity ",
			expected: []string{"face", "license plate", "nudity"},
		},
		{
			name:     "only separators returns defaults",
			raw:      ",,,",
			expected: []string{privacy.ConceptFace, privacy.ConceptLicensePlate, privacy.ConceptNudity},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := privacy.ParseConcepts(tt.raw)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestWithoutFaces(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "drops face",
			input:    []string{"face", "license plate", "nudity"},
			expected: []string{"license plate", "nudity"},
		},
		{
			name:     "case-insensitive drop",
			input:    []string{"Face", "nudity"},
			expected: []string{"nudity"},
		},
		{
			name:     "only face yields empty",
			input:    []string{"face"},
			expected: []string{},
		},
		{
			name:     "no face untouched",
			input:    []string{"license plate"},
			expected: []string{"license plate"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := privacy.WithoutFaces(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestDefaultConceptsImmutable(t *testing.T) {
	got := privacy.ParseConcepts("")
	require.Equal(t, privacy.DefaultConcepts, got)
	got[0] = "mutated"
	assert.NotEqual(t, "mutated", privacy.DefaultConcepts[0])
}
