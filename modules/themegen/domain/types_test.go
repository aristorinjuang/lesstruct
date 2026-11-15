package themegen_test

import (
	"testing"

	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
	"github.com/stretchr/testify/assert"
)

func TestNewThemeSources(t *testing.T) {
	tests := []struct {
		name      string
		css       string
		templates string
		expected  themegendomain.ThemeSources
	}{
		{
			name:      "success - stylesheets only",
			css:       "--- base.css ---\n:root{}",
			templates: "",
			expected: themegendomain.ThemeSources{
				CSS:       "--- base.css ---\n:root{}",
				Templates: "",
			},
		},
		{
			name:      "success - stylesheets and templates",
			css:       "--- base.css ---\n:root{}",
			templates: "--- template: index ---\n{{define \"body\"}}{{end}}",
			expected: themegendomain.ThemeSources{
				CSS:       "--- base.css ---\n:root{}",
				Templates: "--- template: index ---\n{{define \"body\"}}{{end}}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := themegendomain.NewThemeSources(tt.css, tt.templates)
			assert.Equal(t, tt.expected, result)
		})
	}
}
