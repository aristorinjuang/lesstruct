// Package textgen is the AI text module's domain layer: the text-generation
// port, its result value types, and the parser that validates the model's
// JSON envelope. Provider adapters (the OpenAI-compatible client) live in the
// module's infrastructure package and implement the port.
package textgen

import "context"

const (
	// FormatTiptap is the canonical rich-text format: the generated body is a
	// TipTap JSON document.
	FormatTiptap = "tiptap"
	// FormatHTML is the HTML format: the generated body is an HTML fragment.
	FormatHTML = "html"
)

// EnhanceResult is the outcome of enhancing rich-text content: the enhanced
// body document plus AI-generated SEO metadata. Title and MetaDescription
// are populated for the tiptap format only; HTML generation fills Content.
type EnhanceResult struct {
	Content         string
	Title           string
	MetaDescription string
}

// NewEnhanceResult assembles an enhance outcome from its generated parts.
func NewEnhanceResult(
	content, title, metaDescription string,
) EnhanceResult {
	return EnhanceResult{
		Content:         content,
		Title:           title,
		MetaDescription: metaDescription,
	}
}

// TranslateResult is the outcome of translating content: the translated body
// plus translated SEO metadata. Title and MetaDescription are populated for
// both the tiptap and html formats.
type TranslateResult struct {
	Content         string
	Title           string
	MetaDescription string
}

// NewTranslateResult assembles a translate outcome from its generated parts.
func NewTranslateResult(
	content, title, metaDescription string,
) TranslateResult {
	return TranslateResult{
		Content:         content,
		Title:           title,
		MetaDescription: metaDescription,
	}
}

// TextGenerationService defines the interface for AI text generation services.
type TextGenerationService interface {
	EnhanceText(
		ctx context.Context,
		content, format, mediaContext string,
	) (EnhanceResult, error)
	TranslateText(
		ctx context.Context,
		content, title, metaDescription, sourceLang, targetLang, format string,
	) (TranslateResult, error)
	EnhanceTranscription(
		ctx context.Context,
		transcript string,
	) (EnhanceResult, error)
}
