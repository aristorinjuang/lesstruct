package modgen_test

import (
	"testing"

	modgen "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFinding(t *testing.T) {
	tests := []struct {
		name     string
		rule     string
		severity string
		message  string
		wantErr  bool
	}{
		{
			name:     "success - error finding",
			rule:     "module-go-parse",
			severity: modgen.FindingSeverityError,
			message:  "bad",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finding := modgen.NewFinding(tt.rule, tt.severity, tt.message)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.rule, finding.Rule)
			assert.Equal(t, tt.severity, finding.Severity)
			assert.Equal(t, tt.message, finding.Message)
		})
	}
}

func TestNewValidationReport(t *testing.T) {
	tests := []struct {
		name     string
		valid    bool
		findings []modgen.Finding
		wantErr  bool
	}{
		{
			name:     "success - valid",
			valid:    true,
			findings: nil,
			wantErr:  false,
		},
		{
			name:     "success - invalid",
			valid:    false,
			findings: []modgen.Finding{modgen.NewFinding("r", modgen.FindingSeverityError, "m")},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := modgen.NewValidationReport(tt.valid, tt.findings)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.valid, report.Valid)
			assert.Equal(t, tt.findings, report.Findings)
		})
	}
}

func TestNewFindingsReport(t *testing.T) {
	tests := []struct {
		name     string
		findings []modgen.Finding
		expected bool
		wantErr  bool
	}{
		{
			name:     "success - empty findings valid",
			findings: nil,
			expected: true,
			wantErr:  false,
		},
		{
			name: "success - warning only valid",
			findings: []modgen.Finding{
				modgen.NewFinding("r", modgen.FindingSeverityWarning, "m"),
			},
			expected: true,
			wantErr:  false,
		},
		{
			name: "success - error invalid",
			findings: []modgen.Finding{
				modgen.NewFinding("r", modgen.FindingSeverityWarning, "m"),
				modgen.NewFinding("r", modgen.FindingSeverityError, "m"),
			},
			expected: false,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := modgen.NewFindingsReport(tt.findings)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.expected, report.Valid)
			assert.Equal(t, tt.findings, report.Findings)
		})
	}
}
