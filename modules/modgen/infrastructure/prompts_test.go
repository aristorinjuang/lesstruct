package infrastructure_test

import (
	"strings"
	"testing"

	modgendomain "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
	"github.com/aristorinjuang/lesstruct/modules/modgen/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSystemPrompt(t *testing.T) {
	tests := []struct {
		name    string
		sources modgendomain.ModuleSources
		want    []string
		wantErr bool
	}{
		{
			name:    "success - distilled contract only",
			sources: modgendomain.NewModuleSources("", "", "", ""),
			want:    []string{"Tier 1", "kernel.Module", "Output ONLY a JSON object"},
			wantErr: false,
		},
		{
			name:    "success - full sources",
			sources: modgendomain.NewModuleSources("contract", "grammar", "examples", "v1.6.0"),
			want:    []string{"contract", "grammar", "examples", "v1.6.0"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := infrastructure.BuildSystemPrompt(tt.sources)
			require.Equal(t, tt.wantErr, false)
			for _, want := range tt.want {
				assert.Contains(t, prompt, want)
			}
		})
	}
}

func TestBuildUserPrompt(t *testing.T) {
	tests := []struct {
		name    string
		brief   modgendomain.Brief
		want    []string
		wantErr bool
	}{
		{
			name:    "success - surfaces listed",
			brief:   modgenBrief("A forum module", []string{"routes", "migrations"}),
			want:    []string{"routes, migrations", "A forum module"},
			wantErr: false,
		},
		{
			name:    "success - surfaces inferred",
			brief:   modgenBrief("Something", nil),
			want:    []string{"infer from the brief", "Something"},
			wantErr: false,
		},
		{
			name:    "success - long brief truncated",
			brief:   modgenBrief(strings.Repeat("a", 6000), nil),
			want:    []string{"Brief:"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := infrastructure.BuildUserPrompt(tt.brief)
			require.Equal(t, tt.wantErr, false)
			for _, want := range tt.want {
				assert.Contains(t, prompt, want)
			}
		})
	}
}

func TestBuildRepairPrompt(t *testing.T) {
	tests := []struct {
		name    string
		brief   modgendomain.Brief
		files   map[string]string
		want    []string
		wantErr bool
	}{
		{
			name:    "success - repair prompt",
			brief:   modgenBrief("A forum module", []string{"routes"}),
			files:   map[string]string{"module.go": "package forum"},
			want:    []string{"failed validation", "module.go", "package forum"},
			wantErr: false,
		},
		{
			name:    "success - no surfaces",
			brief:   modgenBrief("Something", nil),
			files:   map[string]string{"module.go": strings.Repeat("x", 20000)},
			want:    []string{"failed validation"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := infrastructure.BuildRepairPrompt(tt.brief, tt.files, []string{"module-contract: fix it"})
			require.Equal(t, tt.wantErr, false)
			for _, want := range tt.want {
				assert.Contains(t, prompt, want)
			}
		})
	}
}

func TestBuildRepairPromptBoundsTotal(t *testing.T) {
	tests := []struct {
		name     string
		fileSize int
		wantErr  bool
	}{
		{
			name:     "success - total prompt stays bounded",
			fileSize: 20000,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := make(map[string]string, 64)
			for i := range 64 {
				files["domain/f"+string(rune('a'+i%26))+string(rune('0'+i%10))+".go"] = strings.Repeat("x", tt.fileSize)
			}
			prompt := infrastructure.BuildRepairPrompt(modgenBrief("A forum module", nil), files, []string{"fix it"})
			require.Equal(t, tt.wantErr, false)
			assert.Less(t, len([]rune(prompt)), 300000)
			assert.Contains(t, prompt, "prompt budget reached")
		})
	}
}

func modgenBrief(description string, surfaces []string) modgendomain.Brief {
	return modgendomain.NewBrief(description, surfaces)
}
