package infrastructure_test

import (
	"testing"

	textgeninfra "github.com/aristorinjuang/lesstruct/modules/textgen/infrastructure"
	"github.com/stretchr/testify/assert"
)

func TestBuildHTMLSystemPrompt_NoThemeCSS(t *testing.T) {
	tests := []struct {
		name          string
		themeBaseCSS  string
		themeStyleCSS string
	}{
		{
			name:          "both empty",
			themeBaseCSS:  "",
			themeStyleCSS: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := textgeninfra.BuildHTMLSystemPromptForTest(tt.themeBaseCSS, tt.themeStyleCSS)

			assert.Contains(t, prompt, "## Hard rules")
			assert.Contains(t, prompt, "## Brand consistency")
			assert.Contains(t, prompt, "Few-shot examples")
			assert.Contains(t, prompt, "var(--color-primary)")
			assert.NotContains(t, prompt, "Active site theme CSS")
		})
	}
}

func TestBuildHTMLSystemPrompt_WithThemeCSS(t *testing.T) {
	tests := []struct {
		name          string
		themeBaseCSS  string
		themeStyleCSS string
	}{
		{
			name:          "both present",
			themeBaseCSS:  ":root{--color-primary:#22d3ee;--color-text:#1a1a2e}",
			themeStyleCSS: ".btn{background:var(--color-primary)}",
		},
		{
			name:          "only base present",
			themeBaseCSS:  ":root{--color-primary:#22d3ee}",
			themeStyleCSS: "",
		},
		{
			name:          "only style present",
			themeBaseCSS:  "",
			themeStyleCSS: ".btn{background:var(--color-primary)}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := textgeninfra.BuildHTMLSystemPromptForTest(tt.themeBaseCSS, tt.themeStyleCSS)

			assert.Contains(t, prompt, "## Hard rules")
			assert.Contains(t, prompt, "## Brand consistency")
			assert.Contains(t, prompt, "Active site theme CSS")
			assert.Contains(t, prompt, "### base.css")
			assert.Contains(t, prompt, "### style.css")
			assert.Contains(t, prompt, "Few-shot examples")

			if tt.themeBaseCSS != "" {
				assert.Contains(t, prompt, tt.themeBaseCSS)
			}
			if tt.themeStyleCSS != "" {
				assert.Contains(t, prompt, tt.themeStyleCSS)
			}
		})
	}
}

func TestStripJSONFences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain json passes through",
			input:    `{"title":"T"}`,
			expected: `{"title":"T"}`,
		},
		{
			name:     "json fences are stripped",
			input:    "```json\n" + `{"title":"T"}` + "\n```",
			expected: `{"title":"T"}`,
		},
		{
			name:     "bare fences are stripped",
			input:    "```\n" + `{"title":"T"}` + "\n```",
			expected: `{"title":"T"}`,
		},
		{
			name:     "surrounding whitespace is trimmed",
			input:    "  \n" + `{"title":"T"}` + "  \n",
			expected: `{"title":"T"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, textgeninfra.StripJSONFencesForTest(tt.input))
		})
	}
}
