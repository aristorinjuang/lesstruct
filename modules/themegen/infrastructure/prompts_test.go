package infrastructure_test

import (
	"strings"
	"testing"

	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
	"github.com/aristorinjuang/lesstruct/modules/themegen/infrastructure"
	"github.com/stretchr/testify/assert"
)

func TestBuildSystemPrompt(t *testing.T) {
	tests := []struct {
		name              string
		sources           themegendomain.ThemeSources
		templateOverrides bool
		wantContains      []string
		wantAbsent        []string
	}{
		{
			name:              "success - contract only",
			sources:           themegendomain.NewThemeSources("", ""),
			templateOverrides: false,
			wantContains:      []string{"--color-primary", `"files"`},
			wantAbsent:        []string{"Template overrides", "template sources"},
		},
		{
			name:              "success - with stylesheet sources",
			sources:           themegendomain.NewThemeSources("base content\n--- style ---\nstyle content", ""),
			templateOverrides: false,
			wantContains:      []string{"base content", "style content"},
			wantAbsent:        []string{"Template overrides"},
		},
		{
			name:              "success - template contract and sources with overrides",
			sources:           themegendomain.NewThemeSources("base content", "template page content"),
			templateOverrides: true,
			wantContains:      []string{"Template overrides", "templates/index.html", "template page content", `{{define "body"}}`, "formatDate"},
			wantAbsent:        []string{},
		},
		{
			name:              "success - template contract without sources",
			sources:           themegendomain.NewThemeSources("base content", ""),
			templateOverrides: true,
			wantContains:      []string{"Template overrides", `"templates/index.html"`},
			wantAbsent:        []string{"template sources"},
		},
		{
			name:              "success - template sources ignored without the flag",
			sources:           themegendomain.NewThemeSources("base content", "template page content"),
			templateOverrides: false,
			wantContains:      []string{"base content"},
			wantAbsent:        []string{"template page content", "Template overrides"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := infrastructure.BuildSystemPrompt(tt.sources, tt.templateOverrides)
			for _, want := range tt.wantContains {
				assert.Contains(t, got, want)
			}
			for _, absent := range tt.wantAbsent {
				assert.NotContains(t, got, absent)
			}
		})
	}
}

func TestBuildUserPrompt(t *testing.T) {
	tests := []struct {
		name         string
		description  string
		style        string
		wantContains []string
	}{
		{
			name:         "success - dark preset",
			description:  "A dark editorial theme",
			style:        "dark",
			wantContains: []string{"dark", "A dark editorial theme"},
		},
		{
			name:         "success - auto preset",
			description:  "A clean blog theme",
			style:        "",
			wantContains: []string{"auto", "A clean blog theme"},
		},
		{
			name:         "success - truncates long brief",
			description:  strings.Repeat("a", 6000),
			style:        "",
			wantContains: []string{"auto"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := infrastructure.BuildUserPrompt(tt.description, tt.style)
			for _, want := range tt.wantContains {
				assert.Contains(t, got, want)
			}
		})
	}
}

func TestBuildRepairPrompt(t *testing.T) {
	tests := []struct {
		name         string
		brief        themegendomain.Brief
		files        map[string]string
		problems     []string
		wantContains []string
	}{
		{
			name:         "success - lists problems and files",
			brief:        themegendomain.NewBrief("A dark theme", "", false),
			files:        map[string]string{"static/base.css": ":root{--color-bg:#fff}"},
			problems:     []string{"css-parse: static/base.css has unbalanced braces"},
			wantContains: []string{"css-parse", "static/base.css", "--color-bg"},
		},
		{
			name:         "success - carries the style preset into the repair",
			brief:        themegendomain.NewBrief("A dark theme", "dark", false),
			files:        map[string]string{"static/base.css": ":root{--color-bg:#fff}"},
			problems:     []string{"css-parse: static/base.css has unbalanced braces"},
			wantContains: []string{"Style preset: dark", "css-parse"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := infrastructure.BuildRepairPrompt(tt.brief, tt.files, tt.problems)
			for _, want := range tt.wantContains {
				assert.Contains(t, got, want)
			}
		})
	}
}
