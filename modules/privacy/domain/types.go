// Package privacy is the privacy redaction domain: SAM-agnostic detection
// vocabulary, the Segmenter port, and the sentinel for fail-closed scans.
package privacy

import (
	"errors"
	"image"
	"strings"
)

const (
	// ConceptFace marks human faces. Profile pictures exempt it; every
	// other surface blurs it.
	ConceptFace = "face"
	// ConceptLicensePlate marks vehicle plates. Blurred everywhere. The
	// phrase is a plain noun phrase because the Model API matches
	// concrete nouns best.
	ConceptLicensePlate = "license plate"
	// ConceptNudity marks nudity. Blurred everywhere.
	ConceptNudity = "nudity"
)

var (
	// ErrScanFailed is returned when the segmentation backend fails. Callers
	// fail closed: reject the upload and store nothing.
	ErrScanFailed = errors.New("privacy scan failed")

	// DefaultConcepts is the scan vocabulary used when AI_SAM_CONCEPTS is
	// empty or unset.
	DefaultConcepts = []string{ConceptFace, ConceptLicensePlate, ConceptNudity}
)

// Detection is one sensitive region in pixel coordinates. Box is inclusive
// of the minimum corner and exclusive of the maximum, matching
// image.Rectangle.
type Detection struct {
	Concept string
	Box     image.Rectangle
}

// NewDetection builds a Detection from a concept and pixel bounds.
func NewDetection(concept string, box image.Rectangle) Detection {
	return Detection{
		Concept: concept,
		Box:     box,
	}
}

// ParseConcepts splits a comma-separated concept list into normalized tokens.
// Empty entries are dropped; tokens are lowercased and trimmed. An empty
// input yields a copy of DefaultConcepts so the scanner stays opt-out only
// via the missing API key, never via an empty list.
func ParseConcepts(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		out := make([]string, len(DefaultConcepts))
		copy(out, DefaultConcepts)
		return out
	}
	seen := make(map[string]bool)
	out := make([]string, 0, len(DefaultConcepts))
	for part := range strings.SplitSeq(raw, ",") {
		token := strings.ToLower(strings.TrimSpace(part))
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		out = append(out, token)
	}
	if len(out) == 0 {
		cp := make([]string, len(DefaultConcepts))
		copy(cp, DefaultConcepts)
		return cp
	}
	return out
}

// WithoutFaces drops the face concept so profile pictures keep faces
// visible while plates and nudity still blur.
func WithoutFaces(concepts []string) []string {
	out := make([]string, 0, len(concepts))
	for _, c := range concepts {
		if strings.ToLower(strings.TrimSpace(c)) == ConceptFace {
			continue
		}
		out = append(out, c)
	}
	return out
}
