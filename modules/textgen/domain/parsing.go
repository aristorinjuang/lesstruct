package textgen

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// maxEnhanceTitleRunes bounds the AI-generated SEO title. The title
	// doubles as the social share (OG) title, so it shares the 60-character
	// limit.
	maxEnhanceTitleRunes = 60
	// maxEnhanceMetaDescriptionRunes bounds the AI-generated meta description.
	maxEnhanceMetaDescriptionRunes = 160
)

// enhanceEnvelope mirrors the JSON envelope the AI returns when enhancing
// rich-text content: the enhanced document plus SEO metadata.
type enhanceEnvelope struct {
	Title           string          `json:"title"`
	MetaDescription string          `json:"metaDescription"`
	Content         json.RawMessage `json:"content"`
}

// truncateRunes shortens s to at most maxRunes Unicode characters, trimming
// trailing whitespace left behind by the cut.
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return strings.TrimSpace(string(runes[:maxRunes]))
}

// ParseMetadataEnvelope validates an AI response envelope carrying generated
// content plus SEO metadata and splits it into its parts. The title is
// required; the meta description is optional and may be empty.
// For FormatHTML, content must be a non-empty string; otherwise it must be a
// valid TipTap document object.
func ParseMetadataEnvelope(responseText, format string) (content, title, metaDescription string, err error) {
	var envelope enhanceEnvelope
	if err := json.Unmarshal([]byte(responseText), &envelope); err != nil {
		return "", "", "", fmt.Errorf("AI response is not valid JSON: %w", err)
	}

	title = truncateRunes(strings.TrimSpace(envelope.Title), maxEnhanceTitleRunes)
	if title == "" {
		return "", "", "", fmt.Errorf("AI response is missing the SEO title")
	}

	if len(envelope.Content) == 0 {
		return "", "", "", fmt.Errorf("AI response is missing the generated content")
	}
	if format == FormatHTML {
		var html string
		if err := json.Unmarshal(envelope.Content, &html); err != nil {
			return "", "", "", fmt.Errorf("AI response content is not valid HTML text: %w", err)
		}
		content = strings.TrimSpace(html)
		if content == "" {
			return "", "", "", fmt.Errorf("AI response is missing the generated content")
		}
	} else {
		var contentDoc map[string]any
		if err := json.Unmarshal(envelope.Content, &contentDoc); err != nil {
			return "", "", "", fmt.Errorf("AI response content is not valid JSON: %w", err)
		}
		if docType, ok := contentDoc["type"].(string); !ok || docType != "doc" {
			return "", "", "", fmt.Errorf("AI response is not a valid TipTap document: missing 'doc' type")
		}
		content = strings.TrimSpace(string(envelope.Content))
	}

	metaDescription = truncateRunes(strings.TrimSpace(envelope.MetaDescription), maxEnhanceMetaDescriptionRunes)
	return content, title, metaDescription, nil
}

// ParseEnhanceEnvelope validates the AI's enhance response and splits it into
// the enhanced TipTap document and its SEO metadata. The title and the
// document are required; the meta description is optional and may be empty.
func ParseEnhanceEnvelope(responseText string) (EnhanceResult, error) {
	content, title, metaDescription, err := ParseMetadataEnvelope(responseText, FormatTiptap)
	if err != nil {
		return EnhanceResult{}, err
	}
	return NewEnhanceResult(content, title, metaDescription), nil
}
