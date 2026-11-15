package themegen_test

import (
	"context"
	"testing"

	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator(t *testing.T) {
	tests := []struct {
		name      string
		theme     themegendomain.Theme
		wantValid bool
	}{
		{
			name: "success - valid css",
			theme: themegendomain.NewTheme("midnight-serif", themegendomain.EditionBlog, map[string]string{
				"static/base.css":  ":root{--color-bg:#fff}",
				"static/style.css": ".btn{color:red}",
			}, ""),
			wantValid: true,
		},
		{
			name: "error - unbalanced braces",
			theme: themegendomain.NewTheme("midnight-serif", themegendomain.EditionBlog, map[string]string{
				"static/base.css": ":root{--color-bg:#fff",
			}, ""),
			wantValid: false,
		},
		{
			name: "success - external url warns only",
			theme: themegendomain.NewTheme("midnight-serif", themegendomain.EditionBlog, map[string]string{
				"static/base.css":  ":root{}",
				"static/style.css": ".a{background:url(https://evil.example/x.png)}",
			}, ""),
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := themegendomain.NewValidator()
			report := validator.Validate(context.Background(), tt.theme)
			assert.Equal(t, tt.wantValid, report.Valid)
			if !tt.wantValid {
				require.NotEmpty(t, report.Findings)
			}
		})
	}
}
