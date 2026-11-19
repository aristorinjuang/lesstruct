package plugingen_test

import (
	"testing"

	plugingen "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSynthesizeManifest(t *testing.T) {
	tests := []struct {
		name     string
		input    plugingen.Project
		expected string
		wantErr  bool
	}{
		{
			name: "success - full grants",
			input: withGrants(plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{"main.go": "package main"},
				"",
			), []string{"https://api.example.com/*"}, []string{"read:content"}),
			expected: "name = \"my-plugin\"\nversion = \"0.1.0\"\nmax_memory_mb = 64\n\n[capabilities]\nhttp = [\"https://api.example.com/*\"]\ndatabase = [\"read:content\"]\n",
			wantErr:  false,
		},
		{
			name: "success - empty grants",
			input: plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{"main.go": "package main"},
				"",
			),
			expected: "name = \"my-plugin\"\nversion = \"0.1.0\"\nmax_memory_mb = 64\n\n[capabilities]\nhttp = []\ndatabase = []\n",
			wantErr:  false,
		},
		{
			name: "success - patterns trimmed",
			input: withGrants(plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{"main.go": "package main"},
				"",
			), []string{"  https://api.example.com/*  "}, []string{"  read:media  "}),
			expected: "name = \"my-plugin\"\nversion = \"0.1.0\"\nmax_memory_mb = 64\n\n[capabilities]\nhttp = [\"https://api.example.com/*\"]\ndatabase = [\"read:media\"]\n",
			wantErr:  false,
		},
		{
			name: "error - invalid name",
			input: withGrants(plugingen.NewProject(
				"Bad!",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{"main.go": "package main"},
				"",
			), nil, nil),
			wantErr: true,
		},
		{
			name: "error - invalid pattern",
			input: withGrants(plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{"main.go": "package main"},
				"",
			), []string{"https://*.example.com/x"}, nil),
			wantErr: true,
		},
		{
			name: "error - unknown permission",
			input: withGrants(plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{"main.go": "package main"},
				"",
			), nil, []string{"read:secrets"}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered, err := plugingen.SynthesizeManifest(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, rendered)
		})
	}
}
