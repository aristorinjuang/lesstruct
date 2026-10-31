package textgen_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	textgen "github.com/aristorinjuang/lesstruct/modules/textgen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validTipTapJSON returns a minimal valid TipTap document JSON string.
func validTipTapJSON() string {
	doc := map[string]any{
		"type": "doc",
		"content": []map[string]any{
			{
				"type": "paragraph",
				"content": []map[string]any{
					{"type": "text", "text": "Hello world"},
				},
			},
		},
	}
	b, _ := json.Marshal(doc)
	return string(b)
}

// buildEnhanceEnvelope builds an AI enhance-response envelope JSON string with
// the given title, meta description, and raw content JSON.
func buildEnhanceEnvelope(title, meta, contentJSON string) string {
	titleJSON, _ := json.Marshal(title)
	metaJSON, _ := json.Marshal(meta)
	return fmt.Sprintf(`{"title":%s,"metaDescription":%s,"content":%s}`, titleJSON, metaJSON, contentJSON)
}

func TestParseEnhanceEnvelope(t *testing.T) {
	tests := []struct {
		name            string
		response        string
		expectedContent string
		expectedTitle   string
		expectedMeta    string
		wantErr         bool
	}{
		{
			name:            "success - full envelope",
			response:        buildEnhanceEnvelope("Enhanced Hello World", "A sharper hello to the world.", validTipTapJSON()),
			expectedContent: validTipTapJSON(),
			expectedTitle:   "Enhanced Hello World",
			expectedMeta:    "A sharper hello to the world.",
			wantErr:         false,
		},
		{
			name:            "success - missing meta description is allowed",
			response:        `{"title":"Enhanced Hello World","content":` + validTipTapJSON() + `}`,
			expectedContent: validTipTapJSON(),
			expectedTitle:   "Enhanced Hello World",
			expectedMeta:    "",
			wantErr:         false,
		},
		{
			name:            "success - trims surrounding whitespace",
			response:        buildEnhanceEnvelope("  Padded Title  ", "  Padded summary.  ", "  "+validTipTapJSON()+"  "),
			expectedContent: validTipTapJSON(),
			expectedTitle:   "Padded Title",
			expectedMeta:    "Padded summary.",
			wantErr:         false,
		},
		{
			name:            "success - truncates long title to 60 characters",
			response:        buildEnhanceEnvelope(strings.Repeat("a", 70), "Summary.", validTipTapJSON()),
			expectedContent: validTipTapJSON(),
			expectedTitle:   strings.Repeat("a", 60),
			expectedMeta:    "Summary.",
			wantErr:         false,
		},
		{
			name:            "success - truncates long meta description to 160 characters",
			response:        buildEnhanceEnvelope("Title", strings.Repeat("b", 200), validTipTapJSON()),
			expectedContent: validTipTapJSON(),
			expectedTitle:   "Title",
			expectedMeta:    strings.Repeat("b", 160),
			wantErr:         false,
		},
		{
			name:            "success - truncation trims trailing space",
			response:        buildEnhanceEnvelope(strings.Repeat("a", 59)+" bcdef", "Summary.", validTipTapJSON()),
			expectedContent: validTipTapJSON(),
			expectedTitle:   strings.Repeat("a", 59),
			expectedMeta:    "Summary.",
			wantErr:         false,
		},
		{
			name:     "error - response is not json",
			response: "this is not json",
			wantErr:  true,
		},
		{
			name:     "error - missing title",
			response: `{"metaDescription":"Summary.","content":` + validTipTapJSON() + `}`,
			wantErr:  true,
		},
		{
			name:     "error - blank title",
			response: buildEnhanceEnvelope("   ", "Summary.", validTipTapJSON()),
			wantErr:  true,
		},
		{
			name:     "error - missing content",
			response: `{"title":"Title","metaDescription":"Summary."}`,
			wantErr:  true,
		},
		{
			name:     "error - null content",
			response: `{"title":"Title","metaDescription":"Summary.","content":null}`,
			wantErr:  true,
		},
		{
			name:     "error - content is not a json object",
			response: buildEnhanceEnvelope("Title", "Summary.", `"just a string"`),
			wantErr:  true,
		},
		{
			name:     "error - content is a json array",
			response: buildEnhanceEnvelope("Title", "Summary.", `[1,2,3]`),
			wantErr:  true,
		},
		{
			name:     "error - content is not a tiptap doc",
			response: buildEnhanceEnvelope("Title", "Summary.", `{"type":"paragraph"}`),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := textgen.ParseEnhanceEnvelope(tt.response)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedContent, result.Content)
			assert.Equal(t, tt.expectedTitle, result.Title)
			assert.Equal(t, tt.expectedMeta, result.MetaDescription)
		})
	}
}

func TestParseMetadataEnvelope(t *testing.T) {
	tests := []struct {
		name            string
		response        string
		format          string
		expectedContent string
		expectedTitle   string
		expectedMeta    string
		wantErr         bool
	}{
		{
			name:            "success - tiptap envelope",
			response:        buildEnhanceEnvelope("Bonjour le monde", "Un résumé.", validTipTapJSON()),
			format:          textgen.FormatTiptap,
			expectedContent: validTipTapJSON(),
			expectedTitle:   "Bonjour le monde",
			expectedMeta:    "Un résumé.",
			wantErr:         false,
		},
		{
			name:            "success - html envelope",
			response:        buildEnhanceEnvelope("Bonjour le monde", "Un résumé.", `"  <section>Bonjour le monde</section>  "`),
			format:          textgen.FormatHTML,
			expectedContent: "<section>Bonjour le monde</section>",
			expectedTitle:   "Bonjour le monde",
			expectedMeta:    "Un résumé.",
			wantErr:         false,
		},
		{
			name:            "success - html envelope truncates long meta to 160 characters",
			response:        buildEnhanceEnvelope("Titre", strings.Repeat("c", 200), `"<section>Bonjour</section>"`),
			format:          textgen.FormatHTML,
			expectedContent: "<section>Bonjour</section>",
			expectedTitle:   "Titre",
			expectedMeta:    strings.Repeat("c", 160),
			wantErr:         false,
		},
		{
			name:     "error - html content is not a string",
			response: buildEnhanceEnvelope("Titre", "Résumé.", validTipTapJSON()),
			format:   textgen.FormatHTML,
			wantErr:  true,
		},
		{
			name:     "error - html content is blank",
			response: buildEnhanceEnvelope("Titre", "Résumé.", `"   "`),
			format:   textgen.FormatHTML,
			wantErr:  true,
		},
		{
			name:     "error - html envelope missing title",
			response: `{"metaDescription":"Résumé.","content":"<section>Bonjour</section>"}`,
			format:   textgen.FormatHTML,
			wantErr:  true,
		},
		{
			name:     "error - response is not json",
			response: "this is not json",
			format:   textgen.FormatHTML,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, title, meta, err := textgen.ParseMetadataEnvelope(tt.response, tt.format)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedContent, content)
			assert.Equal(t, tt.expectedTitle, title)
			assert.Equal(t, tt.expectedMeta, meta)
		})
	}
}
