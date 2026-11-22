package modgen_test

import (
	"testing"
	"time"

	modgen "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModuleSources(t *testing.T) {
	tests := []struct {
		name          string
		contract      string
		grammar       string
		examples      string
		kernelVersion string
		wantErr       bool
	}{
		{
			name:          "success - full sources",
			contract:      "contract",
			grammar:       "grammar",
			examples:      "examples",
			kernelVersion: "v1.6.0",
			wantErr:       false,
		},
		{
			name:    "success - empty sources",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources := modgen.NewModuleSources(tt.contract, tt.grammar, tt.examples, tt.kernelVersion)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.contract, sources.Contract)
			assert.Equal(t, tt.grammar, sources.Grammar)
			assert.Equal(t, tt.examples, sources.Examples)
			assert.Equal(t, tt.kernelVersion, sources.KernelVersion)
		})
	}
}

func TestNewBrief(t *testing.T) {
	tests := []struct {
		name        string
		description string
		surfaces    []string
		wantErr     bool
	}{
		{
			name:        "success - with surfaces",
			description: "A forum module",
			surfaces:    []string{"routes", "migrations"},
			wantErr:     false,
		},
		{
			name:        "success - empty surfaces",
			description: "Something",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brief := modgen.NewBrief(tt.description, tt.surfaces)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.description, brief.Description)
			assert.Equal(t, tt.surfaces, brief.Surfaces)
		})
	}
}

func TestNewProject(t *testing.T) {
	tests := []struct {
		name      string
		modName   string
		surfaces  []string
		files     map[string]string
		rationale string
		wantErr   bool
	}{
		{
			name:      "success - full project",
			modName:   "forum",
			surfaces:  []string{"routes"},
			files:     map[string]string{"module.go": "package forum"},
			rationale: "R.",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := modgen.NewProject(tt.modName, tt.surfaces, tt.files, tt.rationale)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.modName, project.Name)
			assert.Equal(t, tt.surfaces, project.Surfaces)
			assert.Equal(t, tt.files, project.Files)
			assert.Equal(t, tt.rationale, project.Rationale)
		})
	}
}

func TestNewInstalledProject(t *testing.T) {
	tests := []struct {
		name      string
		modName   string
		surfaces  []string
		fileCount int
		source    string
		wantErr   bool
	}{
		{
			name:      "success - projection",
			modName:   "forum",
			surfaces:  []string{"routes"},
			fileCount: 9,
			source:    "generated",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created := time.Now().UTC()
			project := modgen.NewInstalledProject(tt.modName, tt.surfaces, tt.fileCount, created, tt.source, false)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.modName, project.Name)
			assert.Equal(t, tt.surfaces, project.Surfaces)
			assert.Equal(t, tt.fileCount, project.FileCount)
			assert.Equal(t, created, project.CreatedAt)
			assert.Equal(t, tt.source, project.Source)
			assert.False(t, project.Applied)
		})
	}
}

func TestNewApplyStep(t *testing.T) {
	tests := []struct {
		name     string
		stepName string
		passed   bool
		skipped  bool
		output   string
		wantErr  bool
	}{
		{
			name:     "success - passed step",
			stepName: "tests",
			passed:   true,
			skipped:  false,
			output:   "",
			wantErr:  false,
		},
		{
			name:     "success - skipped step",
			stepName: "isolated",
			passed:   false,
			skipped:  true,
			output:   "skipped",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := modgen.NewApplyStep(tt.stepName, tt.passed, tt.skipped, tt.output)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.stepName, step.Name)
			assert.Equal(t, tt.passed, step.Passed)
			assert.Equal(t, tt.skipped, step.Skipped)
			assert.Equal(t, tt.output, step.Output)
		})
	}
}

func TestNewApplyResult(t *testing.T) {
	tests := []struct {
		name     string
		applied  bool
		verified bool
		restart  bool
		wantErr  bool
	}{
		{
			name:     "success - verified apply",
			applied:  true,
			verified: true,
			restart:  true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := []modgen.ApplyStep{modgen.NewApplyStep("tests", true, false, "")}
			result := modgen.NewApplyResult(tt.applied, tt.verified, steps, []string{"forum"}, tt.restart)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.applied, result.Applied)
			assert.Equal(t, tt.verified, result.Verified)
			assert.Equal(t, steps, result.Steps)
			assert.Equal(t, []string{"forum"}, result.AppliedModules)
			assert.Equal(t, tt.restart, result.RequiresRestart)
		})
	}
}

func TestNewProjectManifest(t *testing.T) {
	tests := []struct {
		name          string
		modName       string
		kernelVersion string
		generator     string
		source        string
		wantErr       bool
	}{
		{
			name:          "success - manifest",
			modName:       "forum",
			kernelVersion: "v1.6.0",
			generator:     "modgen@0.1.0",
			source:        "generated",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created := time.Now().UTC()
			manifest := modgen.NewProjectManifest(tt.modName, []string{"routes"}, tt.kernelVersion, tt.generator, created, tt.source)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.modName, manifest.Name)
			assert.Equal(t, []string{"routes"}, manifest.Surfaces)
			assert.Equal(t, tt.kernelVersion, manifest.KernelVersion)
			assert.Equal(t, tt.generator, manifest.Generator)
			assert.Equal(t, created, manifest.CreatedAt)
			assert.Equal(t, tt.source, manifest.Source)
		})
	}
}

func TestNewUnapplyResult(t *testing.T) {
	tests := []struct {
		name             string
		removed          bool
		verified         bool
		moduleDirRemoved bool
		wantErr          bool
	}{
		{
			name:             "success - full removal verified",
			removed:          true,
			verified:         true,
			moduleDirRemoved: true,
			wantErr:          false,
		},
		{
			name:             "success - registry-only cleanup",
			removed:          true,
			verified:         true,
			moduleDirRemoved: false,
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := []modgen.ApplyStep{modgen.NewApplyStep("build", true, false, "")}
			result := modgen.NewUnapplyResult(tt.removed, tt.verified, steps, []string{"wiki"}, tt.moduleDirRemoved)
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.removed, result.Removed)
			assert.Equal(t, tt.verified, result.Verified)
			assert.Equal(t, steps, result.Steps)
			assert.Equal(t, []string{"wiki"}, result.AppliedModules)
			assert.Equal(t, tt.moduleDirRemoved, result.ModuleDirRemoved)
		})
	}
}
