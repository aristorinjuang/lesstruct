package plugingen_test

import (
	"testing"
	"time"

	plugingen "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBrief(t *testing.T) {
	tests := []struct {
		name         string
		description  string
		language     string
		hooks        []string
		capabilities []string
		wantErr      bool
	}{
		{
			name:         "success - valid brief",
			description:  "uppercase titles on save",
			language:     "go",
			hooks:        []string{"before_save"},
			capabilities: []string{"log_info"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brief := plugingen.NewBrief(tt.description, tt.language, tt.hooks, tt.capabilities)
			require.Equal(t, tt.description, brief.Description)
			assert.Equal(t, tt.language, brief.Language)
		})
	}
}

func TestNewProject(t *testing.T) {
	tests := []struct {
		name     string
		input    plugingen.Project
		expected plugingen.Project
		wantErr  bool
	}{
		{
			name: "success - valid project",
			input: plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				[]string{"log_info"},
				map[string]string{
					"main.go": "package main\nfunc main() {}",
					"go.mod":  "module example",
				},
				"rationale",
			),
			expected: plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				[]string{"log_info"},
				map[string]string{
					"main.go": "package main\nfunc main() {}",
					"go.mod":  "module example",
				},
				"rationale",
			),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestNewInstalledProject(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name     string
		input    plugingen.InstalledProject
		expected plugingen.InstalledProject
		wantErr  bool
	}{
		{
			name: "success - valid projection",
			input: plugingen.NewInstalledProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				now,
				"generated",
			),
			expected: plugingen.NewInstalledProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				now,
				"generated",
			),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestNewPluginSources(t *testing.T) {
	tests := []struct {
		name     string
		contract string
		examples string
		wantErr  bool
	}{
		{
			name:     "success - sources",
			contract: "contract",
			examples: "examples",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources := plugingen.NewPluginSources(tt.contract, tt.examples)
			assert.Equal(t, tt.contract, sources.Contract)
			assert.Equal(t, tt.examples, sources.Examples)
		})
	}
}

func TestNewProjectManifest(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name     string
		input    plugingen.ProjectManifest
		expected plugingen.ProjectManifest
		wantErr  bool
	}{
		{
			name: "success - manifest",
			input: plugingen.NewProjectManifest(
				"my-plugin",
				"go",
				[]string{"before_save"},
				[]string{"log_info"},
				[]string{"https://api.example.com/*"},
				[]string{"read:content"},
				"plugingen@0.1.0",
				now,
				"generated",
			),
			expected: plugingen.NewProjectManifest(
				"my-plugin",
				"go",
				[]string{"before_save"},
				[]string{"log_info"},
				[]string{"https://api.example.com/*"},
				[]string{"read:content"},
				"plugingen@0.1.0",
				now,
				"generated",
			),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestNewFinding(t *testing.T) {
	tests := []struct {
		name     string
		input    plugingen.Finding
		expected plugingen.Finding
		wantErr  bool
	}{
		{
			name:     "success - finding",
			input:    plugingen.NewFinding("rule", "error", "message"),
			expected: plugingen.NewFinding("rule", "error", "message"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestNewValidationReport(t *testing.T) {
	tests := []struct {
		name     string
		input    plugingen.ValidationReport
		expected plugingen.ValidationReport
		wantErr  bool
	}{
		{
			name:     "success - report",
			input:    plugingen.NewValidationReport(true, nil),
			expected: plugingen.NewValidationReport(true, nil),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestNewFindingsReport(t *testing.T) {
	tests := []struct {
		name     string
		findings []plugingen.Finding
		valid    bool
		wantErr  bool
	}{
		{
			name:     "success - empty valid",
			findings: nil,
			valid:    true,
			wantErr:  false,
		},
		{
			name: "success - error invalid",
			findings: []plugingen.Finding{
				plugingen.NewFinding("r", "error", "m"),
			},
			valid:   false,
			wantErr: false,
		},
		{
			name: "success - warning valid",
			findings: []plugingen.Finding{
				plugingen.NewFinding("r", "warning", "m"),
			},
			valid:   true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := plugingen.NewFindingsReport(tt.findings)
			assert.Equal(t, tt.valid, report.Valid)
		})
	}
}
