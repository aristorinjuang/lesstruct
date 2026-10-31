package textgen_test

import (
	"testing"

	textgen "github.com/aristorinjuang/lesstruct/modules/textgen/domain"
	"github.com/stretchr/testify/assert"
)

func TestNewEnhanceResult(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		title           string
		metaDescription string
	}{
		{
			name:            "full tiptap result",
			content:         `{"type":"doc"}`,
			title:           "Enhanced Hello World",
			metaDescription: "A sharper hello to the world.",
		},
		{
			name:            "html result without seo metadata",
			content:         "<style>.ls-test{}</style><section>Hello</section>",
			title:           "",
			metaDescription: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := textgen.NewEnhanceResult(tt.content, tt.title, tt.metaDescription)
			assert.Equal(t, tt.content, result.Content)
			assert.Equal(t, tt.title, result.Title)
			assert.Equal(t, tt.metaDescription, result.MetaDescription)
		})
	}
}

func TestNewTranslateResult(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		title           string
		metaDescription string
	}{
		{
			name:            "full tiptap result",
			content:         `{"type":"doc"}`,
			title:           "Bonjour le monde",
			metaDescription: "Un résumé plus net.",
		},
		{
			name:            "html result without seo metadata",
			content:         "<section>Bonjour le monde</section>",
			title:           "",
			metaDescription: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := textgen.NewTranslateResult(tt.content, tt.title, tt.metaDescription)
			assert.Equal(t, tt.content, result.Content)
			assert.Equal(t, tt.title, result.Title)
			assert.Equal(t, tt.metaDescription, result.MetaDescription)
		})
	}
}
