package themegen_test

import (
	"context"
	"testing"
	"time"

	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func now() time.Time {
	return time.Now().UTC()
}

type stubGeneration struct {
	theme themegendomain.Theme
	err   error
}

func (s stubGeneration) GenerateTheme(_ context.Context, _ themegendomain.Brief, edition themegendomain.Edition, _ themegendomain.ThemeSources) (themegendomain.Theme, error) {
	if s.err != nil {
		return themegendomain.Theme{}, s.err
	}
	theme := s.theme
	if theme.Edition == "" {
		theme.Edition = edition
	}
	return theme, nil
}

func (s stubGeneration) RepairTheme(_ context.Context, _ themegendomain.Brief, _ themegendomain.ThemeSources, theme themegendomain.Theme, _ []string) (themegendomain.Theme, error) {
	return theme, nil
}

type stubStore struct {
	themes map[string]themegendomain.Theme
}

func newStubStore() *stubStore {
	return &stubStore{themes: map[string]themegendomain.Theme{}}
}

func (s *stubStore) Install(_ context.Context, theme themegendomain.Theme) error {
	if _, ok := s.themes[theme.Name]; ok {
		return themegendomain.ErrThemeExists
	}
	s.themes[theme.Name] = theme
	return nil
}

func (s *stubStore) List(_ context.Context) ([]themegendomain.InstalledTheme, error) {
	result := make([]themegendomain.InstalledTheme, 0, len(s.themes))
	for _, theme := range s.themes {
		result = append(result, themegendomain.NewInstalledTheme(theme.Name, theme.Edition, now(), false, "generated"))
	}
	return result, nil
}

func (s *stubStore) Get(_ context.Context, name string) (themegendomain.Theme, error) {
	theme, ok := s.themes[name]
	if !ok {
		return themegendomain.Theme{}, themegendomain.ErrThemeNotFound
	}
	return theme, nil
}

func (s *stubStore) Delete(_ context.Context, name string) error {
	if _, ok := s.themes[name]; !ok {
		return themegendomain.ErrThemeNotFound
	}
	delete(s.themes, name)
	return nil
}

func (s *stubStore) Zip(_ context.Context, name string) ([]byte, error) {
	if _, ok := s.themes[name]; !ok {
		return nil, themegendomain.ErrThemeNotFound
	}
	return []byte("zip"), nil
}

func TestServiceGenerate(t *testing.T) {
	validTheme := themegendomain.NewTheme("midnight-serif", themegendomain.EditionBlog, map[string]string{
		"static/base.css": ":root{}",
	}, "")

	tests := []struct {
		name       string
		brief      themegendomain.Brief
		generation themegendomain.ThemeGenerationService
		wantErr    bool
	}{
		{
			name:       "success - generates and installs",
			brief:      themegendomain.NewBrief("A dark editorial theme", "dark", false),
			generation: stubGeneration{theme: validTheme},
			wantErr:    false,
		},
		{
			name:       "error - invalid brief",
			brief:      themegendomain.NewBrief("  ", "", false),
			generation: stubGeneration{theme: validTheme},
			wantErr:    true,
		},
		{
			name:       "error - generation unavailable",
			brief:      themegendomain.NewBrief("A theme", "", false),
			generation: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := themegendomain.NewService(tt.generation, nil, newStubStore(), nil, themegendomain.EditionBlog)
			theme, report, err := svc.Generate(context.Background(), tt.brief, themegendomain.EditionBlog)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "midnight-serif", theme.Name)
			assert.True(t, report.Valid)
		})
	}
}

// scriptedGeneration scripts generation and repair outcomes so the repair
// loop can be exercised without a model.
type scriptedGeneration struct {
	generated   themegendomain.Theme
	generateErr error
	repairs     []themegendomain.Theme
	repairErr   error
	repairCalls int
	problems    [][]string
}

func (s *scriptedGeneration) GenerateTheme(_ context.Context, _ themegendomain.Brief, edition themegendomain.Edition, _ themegendomain.ThemeSources) (themegendomain.Theme, error) {
	if s.generateErr != nil {
		return themegendomain.Theme{}, s.generateErr
	}
	theme := s.generated
	if theme.Edition == "" {
		theme.Edition = edition
	}
	return theme, nil
}

func (s *scriptedGeneration) RepairTheme(_ context.Context, _ themegendomain.Brief, _ themegendomain.ThemeSources, theme themegendomain.Theme, problems []string) (themegendomain.Theme, error) {
	s.repairCalls++
	s.problems = append(s.problems, problems)
	if s.repairErr != nil {
		return themegendomain.Theme{}, s.repairErr
	}
	if len(s.repairs) == 0 {
		return theme, nil
	}
	next := s.repairs[0]
	s.repairs = s.repairs[1:]
	return next, nil
}

func TestServiceGenerateRepair(t *testing.T) {
	brokenTheme := func(name string) themegendomain.Theme {
		return themegendomain.NewTheme(name, themegendomain.EditionBlog, map[string]string{
			"static/base.css": ":root{",
		}, "")
	}
	validTheme := func(name string) themegendomain.Theme {
		return themegendomain.NewTheme(name, themegendomain.EditionBlog, map[string]string{
			"static/base.css": ":root{}",
		}, "")
	}
	scriptTheme := func(name string) themegendomain.Theme {
		return themegendomain.NewTheme(name, themegendomain.EditionBlog, map[string]string{
			"static/base.css":      ":root{}",
			"templates/index.html": `{{define "body"}}<main><script src="https://evil.example/x.js"></script></main>{{end}}`,
		}, "")
	}

	tests := []struct {
		name            string
		scripted        *scriptedGeneration
		wantErr         error
		wantThemeName   string
		wantValid       bool
		wantRepairCalls int
		wantInstalled   int
		wantProblem     string
	}{
		{
			name: "success - repairs after one attempt",
			scripted: &scriptedGeneration{
				generated: brokenTheme("broken-theme"),
				repairs:   []themegendomain.Theme{validTheme("repaired-theme")},
			},
			wantErr:         nil,
			wantThemeName:   "repaired-theme",
			wantValid:       true,
			wantRepairCalls: 1,
			wantInstalled:   1,
			wantProblem:     "css-parse",
		},
		{
			name: "success - repairs on the second attempt",
			scripted: &scriptedGeneration{
				generated: brokenTheme("broken-theme"),
				repairs: []themegendomain.Theme{
					brokenTheme("still-broken"),
					validTheme("repaired-theme"),
				},
			},
			wantErr:         nil,
			wantThemeName:   "repaired-theme",
			wantValid:       true,
			wantRepairCalls: 2,
			wantInstalled:   1,
			wantProblem:     "css-parse",
		},
		{
			name:            "error - never repairs",
			scripted:        &scriptedGeneration{generated: brokenTheme("broken-theme")},
			wantErr:         themegendomain.ErrGenerationFailed,
			wantValid:       false,
			wantRepairCalls: 2,
			wantInstalled:   0,
			wantProblem:     "css-parse",
		},
		{
			name: "error - repair transport failure",
			scripted: &scriptedGeneration{
				generated: brokenTheme("broken-theme"),
				repairErr: assert.AnError,
			},
			wantErr:         assert.AnError,
			wantValid:       false,
			wantRepairCalls: 1,
			wantInstalled:   0,
			wantProblem:     "css-parse",
		},
		{
			name: "success - repairs smuggled script content",
			scripted: &scriptedGeneration{
				generated: scriptTheme("broken-theme"),
				repairs:   []themegendomain.Theme{validTheme("repaired-theme")},
			},
			wantErr:         nil,
			wantThemeName:   "repaired-theme",
			wantValid:       true,
			wantRepairCalls: 1,
			wantInstalled:   1,
			wantProblem:     "template-script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newStubStore()
			svc := themegendomain.NewService(tt.scripted, nil, store, nil, themegendomain.EditionBlog)
			brief := themegendomain.NewBrief("A dark editorial theme", "dark", false)
			theme, report, err := svc.Generate(context.Background(), brief, themegendomain.EditionBlog)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantThemeName, theme.Name)
			}
			assert.Equal(t, tt.wantValid, report.Valid)
			assert.Equal(t, tt.wantRepairCalls, tt.scripted.repairCalls)
			assert.Len(t, store.themes, tt.wantInstalled)
			for _, problems := range tt.scripted.problems {
				require.NotEmpty(t, problems)
				assert.Contains(t, problems[0], tt.wantProblem)
			}
		})
	}
}
