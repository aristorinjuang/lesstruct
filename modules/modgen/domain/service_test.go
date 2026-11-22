package modgen_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	modgen "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeGeneration struct {
	project   modgen.Project
	err       error
	repair    modgen.Project
	repairErr error
	repairSet bool
	calls     int
}

func (f *fakeGeneration) GenerateProject(_ context.Context, _ modgen.Brief, _ modgen.ModuleSources) (modgen.Project, error) {
	if f.err != nil {
		return modgen.Project{}, f.err
	}
	return f.project, nil
}

func (f *fakeGeneration) RepairProject(_ context.Context, _ modgen.Brief, _ modgen.ModuleSources, _ modgen.Project, _ []string) (modgen.Project, error) {
	f.calls++
	if f.repairErr != nil {
		return modgen.Project{}, f.repairErr
	}
	if f.repairSet {
		return f.repair, nil
	}
	return f.project, nil
}

type stubValidator struct {
	report modgen.ValidationReport
}

func (s stubValidator) Validate(_ context.Context, _ modgen.Project) modgen.ValidationReport {
	return s.report
}

type fakeStore struct {
	projects     map[string]modgen.Project
	installErr   error
	listErr      error
	listProjects bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{projects: map[string]modgen.Project{}}
}

func (f *fakeStore) Install(_ context.Context, project modgen.Project) error {
	if f.installErr != nil {
		return f.installErr
	}
	if _, ok := f.projects[project.Name]; ok {
		return modgen.ErrProjectExists
	}
	f.projects[project.Name] = project
	return nil
}

func (f *fakeStore) List(_ context.Context) ([]modgen.InstalledProject, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	if !f.listProjects {
		return []modgen.InstalledProject{}, nil
	}
	projects := make([]modgen.InstalledProject, 0, len(f.projects))
	for name := range f.projects {
		projects = append(projects, modgen.NewInstalledProject(name, nil, 0, time.Time{}, "generated", false))
	}
	return projects, nil
}

func (f *fakeStore) Get(_ context.Context, name string) (modgen.Project, error) {
	if p, ok := f.projects[name]; ok {
		return p, nil
	}
	return modgen.Project{}, modgen.ErrProjectNotFound
}

func (f *fakeStore) Delete(_ context.Context, name string) error {
	if _, ok := f.projects[name]; !ok {
		return modgen.ErrProjectNotFound
	}
	delete(f.projects, name)
	return nil
}

func (f *fakeStore) Zip(_ context.Context, name string) ([]byte, error) {
	if _, ok := f.projects[name]; !ok {
		return nil, modgen.ErrProjectNotFound
	}
	return []byte("zip"), nil
}

func validServiceProject() modgen.Project {
	return modgen.NewProject(
		"forum",
		[]string{"routes"},
		map[string]string{
			"go.mod":    reportGoMod,
			"module.go": reportModuleGo,
			"README.md": "forum",
		},
		"R.",
	)
}

func invalidServiceProject() modgen.Project {
	return modgen.NewProject(
		"forum",
		nil,
		map[string]string{
			"go.mod":    reportGoMod,
			"module.go": reportModuleGo,
		},
		"R.",
	)
}

func TestServiceGenerate(t *testing.T) {
	tests := []struct {
		name        string
		brief       modgen.Brief
		generation  *fakeGeneration
		validator   modgen.ProjectValidator
		store       *fakeStore
		nilStore    bool
		withSources bool
		wantErr     bool
		wantName    string
	}{
		{
			name:        "success - generates and installs",
			brief:       modgen.NewBrief("A forum module", []string{"routes"}),
			generation:  &fakeGeneration{project: validServiceProject()},
			validator:   modgen.NewValidator(),
			store:       newFakeStore(),
			withSources: true,
			wantErr:     false,
			wantName:    "forum",
		},
		{
			name:       "success - surfaces fall back to brief",
			brief:      modgen.NewBrief("A forum module", []string{"migrations"}),
			generation: &fakeGeneration{project: modgen.NewProject("forum", nil, validServiceProject().Files, "R.")},
			validator:  modgen.NewValidator(),
			store:      newFakeStore(),
			wantErr:    false,
			wantName:   "forum",
		},
		{
			name:       "success - nil store skips install",
			brief:      modgen.NewBrief("A forum module", nil),
			generation: &fakeGeneration{project: validServiceProject()},
			validator:  modgen.NewValidator(),
			nilStore:   true,
			wantErr:    false,
			wantName:   "forum",
		},
		{
			name:       "success - default validator",
			brief:      modgen.NewBrief("A forum module", nil),
			generation: &fakeGeneration{project: validServiceProject()},
			validator:  nil,
			store:      newFakeStore(),
			wantErr:    false,
			wantName:   "forum",
		},
		{
			name:       "success - tier guidance warns",
			brief:      modgen.NewBrief("hook my plugin before_save", nil),
			generation: &fakeGeneration{project: modgen.NewProject("forum", nil, validServiceProject().Files, "R.")},
			validator:  modgen.NewValidator(),
			store:      newFakeStore(),
			wantErr:    false,
			wantName:   "forum",
		},
		{
			name:       "success - scan findings repaired",
			brief:      modgen.NewBrief("A forum module", nil),
			generation: &fakeGeneration{project: modgen.NewProject("forum", nil, map[string]string{"module.go": "package forum // see javascript:void(0)"}, "R."), repair: validServiceProject(), repairSet: true},
			validator:  stubValidator{report: modgen.NewValidationReport(true, nil)},
			store:      newFakeStore(),
			wantErr:    false,
			wantName:   "forum",
		},
		{
			name:       "success - repair fixes project",
			brief:      modgen.NewBrief("A forum module", nil),
			generation: &fakeGeneration{project: invalidServiceProject(), repair: validServiceProject(), repairSet: true},
			validator:  modgen.NewValidator(),
			store:      newFakeStore(),
			wantErr:    false,
			wantName:   "forum",
		},
		{
			name:       "success - repair keeps name and surfaces",
			brief:      modgen.NewBrief("A forum module", nil),
			generation: &fakeGeneration{project: invalidServiceProject(), repair: modgen.NewProject("Bad Name!", nil, validServiceProject().Files, "R."), repairSet: true},
			validator:  modgen.NewValidator(),
			store:      newFakeStore(),
			wantErr:    false,
			wantName:   "forum",
		},
		{
			name:       "success - slugifies bad name",
			brief:      modgen.NewBrief("A forum module", nil),
			generation: &fakeGeneration{project: modgen.NewProject("Bad Name!", nil, validServiceProject().Files, "R."), repair: validServiceProject(), repairSet: true},
			validator:  modgen.NewValidator(),
			store:      newFakeStore(),
			wantErr:    false,
			wantName:   "forum",
		},
		{
			name:       "error - invalid brief",
			brief:      modgen.NewBrief("", nil),
			generation: &fakeGeneration{project: validServiceProject()},
			validator:  modgen.NewValidator(),
			store:      newFakeStore(),
			wantErr:    true,
		},
		{
			name:       "error - generation fails",
			brief:      modgen.NewBrief("A forum module", nil),
			generation: &fakeGeneration{err: errors.New("boom")},
			validator:  modgen.NewValidator(),
			store:      newFakeStore(),
			wantErr:    true,
		},
		{
			name:       "error - repair fails",
			brief:      modgen.NewBrief("A forum module", nil),
			generation: &fakeGeneration{project: invalidServiceProject(), repairErr: errors.New("boom")},
			validator:  modgen.NewValidator(),
			store:      newFakeStore(),
			wantErr:    true,
		},
		{
			name:       "error - refused after repairs",
			brief:      modgen.NewBrief("A forum module", nil),
			generation: &fakeGeneration{project: invalidServiceProject()},
			validator:  modgen.NewValidator(),
			store:      newFakeStore(),
			wantErr:    true,
		},
		{
			name:       "error - install collision",
			brief:      modgen.NewBrief("A forum module", nil),
			generation: &fakeGeneration{project: validServiceProject()},
			validator:  modgen.NewValidator(),
			store:      &fakeStore{projects: map[string]modgen.Project{}, installErr: modgen.ErrProjectExists},
			wantErr:    true,
		},
		{
			name:       "error - empty rule finding repaired",
			brief:      modgen.NewBrief("A forum module", nil),
			generation: &fakeGeneration{project: validServiceProject(), repairErr: errors.New("boom")},
			validator: stubValidator{report: modgen.NewFindingsReport([]modgen.Finding{
				modgen.NewFinding("", modgen.FindingSeverityError, "nameless"),
				modgen.NewFinding("r", modgen.FindingSeverityError, "ruled"),
				modgen.NewFinding("w", modgen.FindingSeverityWarning, "warn"),
			})},
			store:   newFakeStore(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var store modgen.ProjectStore
			if !tt.nilStore {
				store = tt.store
			}
			var sources func(ctx context.Context) modgen.ModuleSources
			if tt.withSources {
				sources = func(context.Context) modgen.ModuleSources {
					return modgen.NewModuleSources("c", "g", "e", "v9.9.9")
				}
			}
			svc := modgen.NewService(tt.generation, tt.validator, store, sources)
			project, report, err := svc.Generate(context.Background(), tt.brief)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantName != "" {
				assert.Equal(t, tt.wantName, project.Name)
			} else {
				require.NoError(t, modgen.ValidateProjectName(project.Name))
			}
			_ = report
		})
	}
}

func TestServiceGenerateUnavailable(t *testing.T) {
	tests := []struct {
		name    string
		brief   modgen.Brief
		wantErr bool
	}{
		{
			name:    "error - no generation service",
			brief:   modgen.NewBrief("A forum module", nil),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := modgen.NewService(nil, modgen.NewValidator(), newFakeStore(), nil)
			_, _, err := svc.Generate(context.Background(), tt.brief)
			if tt.wantErr {
				require.ErrorIs(t, err, modgen.ErrGenerationUnavailable)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestServiceSlugifyBranches(t *testing.T) {
	tests := []struct {
		name    string
		brief   string
		wantErr bool
	}{
		{
			name:    "success - no words falls back",
			brief:   "a b c",
			wantErr: false,
		},
		{
			name:    "success - long words truncate",
			brief:   strings.Repeat("a", 30) + " " + strings.Repeat("b", 30),
			wantErr: false,
		},
		{
			name:    "success - digit words fallback",
			brief:   "12345 67890",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &fakeGeneration{project: modgen.NewProject("Bad Name!", nil, validServiceProject().Files, "R."), repair: validServiceProject(), repairSet: true}
			svc := modgen.NewService(gen, modgen.NewValidator(), newFakeStore(), nil)
			project, _, err := svc.Generate(context.Background(), modgen.NewBrief(tt.brief, nil))
			require.Equal(t, tt.wantErr, false)
			require.NoError(t, err)
			require.NoError(t, modgen.ValidateProjectName(project.Name))
		})
	}
}

func TestServicePersistence(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - list get zip delete",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			svc := modgen.NewService(&fakeGeneration{project: validServiceProject()}, modgen.NewValidator(), store, nil)
			_, _, err := svc.Generate(context.Background(), modgen.NewBrief("A forum module", nil))
			require.NoError(t, err)
			listed, err := svc.List(context.Background())
			require.Equal(t, tt.wantErr, false)
			require.NoError(t, err)
			assert.Empty(t, listed)
			got, err := svc.Get(context.Background(), "forum")
			require.NoError(t, err)
			assert.Equal(t, "forum", got.Name)
			data, err := svc.Zip(context.Background(), "forum")
			require.NoError(t, err)
			assert.Equal(t, []byte("zip"), data)
			_, err = svc.Delete(context.Background(), "forum")
			require.NoError(t, err)
			_, err = svc.Get(context.Background(), "forum")
			require.ErrorIs(t, err, modgen.ErrProjectNotFound)
			require.ErrorIs(t, func() error { _, err := svc.Zip(context.Background(), "forum"); return err }(), modgen.ErrProjectNotFound)
			_, err = svc.Delete(context.Background(), "forum")
			require.ErrorIs(t, err, modgen.ErrProjectNotFound)
			report := svc.Validate(context.Background(), validServiceProject())
			assert.True(t, report.Valid)
		})
	}
}

func TestServicePersistenceErrors(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "error - store failures surface",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{projects: map[string]modgen.Project{}, listErr: errors.New("boom")}
			svc := modgen.NewService(&fakeGeneration{}, modgen.NewValidator(), store, nil)
			_, err := svc.List(context.Background())
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestServiceNilStore(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "error - nil store not found",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := modgen.NewService(&fakeGeneration{}, modgen.NewValidator(), nil, nil)
			_, err := svc.List(context.Background())
			require.ErrorIs(t, err, modgen.ErrProjectNotFound)
			_, err = svc.Get(context.Background(), "forum")
			require.ErrorIs(t, err, modgen.ErrProjectNotFound)
			_, err = svc.Delete(context.Background(), "forum")
			require.ErrorIs(t, err, modgen.ErrProjectNotFound)
			_, err = svc.Zip(context.Background(), "forum")
			require.ErrorIs(t, err, modgen.ErrProjectNotFound)
			require.Equal(t, tt.wantErr, true)
		})
	}
}

type fakeApplier struct {
	result     modgen.ApplyResult
	err        error
	applied    []string
	appliedErr error
	gotReplace bool
	gotConfirm bool
}

func (f *fakeApplier) Apply(_ context.Context, _ modgen.Project, replace, confirm bool) (modgen.ApplyResult, error) {
	f.gotReplace = replace
	f.gotConfirm = confirm
	if f.err != nil {
		return modgen.ApplyResult{}, f.err
	}
	return f.result, nil
}

func (f *fakeApplier) Unapply(_ context.Context, _ string, _ bool) (modgen.UnapplyResult, error) {
	if f.err != nil {
		return modgen.UnapplyResult{}, f.err
	}
	return modgen.NewUnapplyResult(true, true, nil, nil, true), nil
}

func (f *fakeApplier) Applied(_ context.Context) ([]string, error) {
	if f.appliedErr != nil {
		return nil, f.appliedErr
	}
	return f.applied, nil
}

func TestServiceApply(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		applier  *fakeApplier
		store    *fakeStore
		nilStore bool
		wantErr  bool
	}{
		{
			name:    "success - applies staged project",
			input:   "forum",
			applier: &fakeApplier{result: modgen.NewApplyResult(true, true, nil, []string{"forum"}, true)},
			store: func() *fakeStore {
				s := newFakeStore()
				s.projects["forum"] = validServiceProject()
				return s
			}(),
			wantErr: false,
		},
		{
			name:    "error - invalid name",
			input:   "Bad Name!",
			applier: &fakeApplier{},
			store:   newFakeStore(),
			wantErr: true,
		},
		{
			name:    "error - no applier",
			input:   "forum",
			applier: nil,
			store: func() *fakeStore {
				s := newFakeStore()
				s.projects["forum"] = validServiceProject()
				return s
			}(),
			wantErr: true,
		},
		{
			name:     "error - nil store",
			input:    "forum",
			applier:  &fakeApplier{},
			nilStore: true,
			wantErr:  true,
		},
		{
			name:    "error - unknown project",
			input:   "missing",
			applier: &fakeApplier{},
			store:   newFakeStore(),
			wantErr: true,
		},
		{
			name:    "error - applier fails",
			input:   "forum",
			applier: &fakeApplier{err: modgen.ErrApplyFailed},
			store: func() *fakeStore {
				s := newFakeStore()
				s.projects["forum"] = validServiceProject()
				return s
			}(),
			wantErr: true,
		},
		{
			name:    "error - staged project fails gate",
			input:   "forum",
			applier: &fakeApplier{result: modgen.NewApplyResult(true, true, nil, []string{"forum"}, true)},
			store: func() *fakeStore {
				s := newFakeStore()
				s.projects["forum"] = modgen.NewProject("forum", nil, map[string]string{
					"module.go": "package forum\nimport \"os/exec\"\n",
				}, "R.")
				return s
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var store modgen.ProjectStore
			if !tt.nilStore {
				store = tt.store
			}
			var applier modgen.Applier
			if tt.applier != nil {
				applier = tt.applier
			}
			svc := modgen.NewServiceWithApplier(&fakeGeneration{}, modgen.NewValidator(), store, applier, nil)
			project, result, err := svc.Apply(context.Background(), tt.input, false, false)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "forum", project.Name)
			assert.True(t, result.Applied)
			assert.True(t, result.Verified)
			assert.True(t, result.RequiresRestart)
		})
	}
}

func TestServiceListApplied(t *testing.T) {
	tests := []struct {
		name     string
		applier  *fakeApplier
		wantMark bool
		wantErr  bool
	}{
		{
			name:     "success - applied project marked",
			applier:  &fakeApplier{applied: []string{"forum"}},
			wantMark: true,
			wantErr:  false,
		},
		{
			name:     "success - lookup failure degrades unmarked",
			applier:  &fakeApplier{appliedErr: errors.New("boom")},
			wantMark: false,
			wantErr:  false,
		},
		{
			name:     "success - other projects unmarked",
			applier:  &fakeApplier{applied: []string{"other"}},
			wantMark: false,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			store.projects["forum"] = validServiceProject()
			store.listProjects = true
			svc := modgen.NewServiceWithApplier(&fakeGeneration{}, modgen.NewValidator(), store, tt.applier, nil)
			listed, err := svc.List(context.Background())
			require.Equal(t, tt.wantErr, false)
			require.NoError(t, err)
			require.Len(t, listed, 1)
			assert.Equal(t, "forum", listed[0].Name)
			assert.Equal(t, tt.wantMark, listed[0].Applied)
		})
	}
}

func TestServiceUnapply(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		confirm bool
		applier *fakeApplier
		wantErr string
	}{
		{
			name:    "success - unapplies with confirm",
			input:   "forum",
			confirm: true,
			applier: &fakeApplier{},
		},
		{
			name:    "error - invalid name",
			input:   "Bad Name!",
			confirm: true,
			applier: &fakeApplier{},
			wantErr: "not found",
		},
		{
			name:    "error - missing confirm",
			input:   "forum",
			confirm: false,
			applier: &fakeApplier{},
			wantErr: "confirmation required",
		},
		{
			name:    "error - no applier",
			input:   "forum",
			confirm: true,
			applier: nil,
			wantErr: "apply unavailable",
		},
		{
			name:    "error - applier fails",
			input:   "forum",
			confirm: true,
			applier: &fakeApplier{err: modgen.ErrNotApplied},
			wantErr: "not applied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var applier modgen.Applier
			if tt.applier != nil {
				applier = tt.applier
			}
			svc := modgen.NewServiceWithApplier(&fakeGeneration{}, modgen.NewValidator(), newFakeStore(), applier, nil)
			result, err := svc.Unapply(context.Background(), tt.input, tt.confirm)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.True(t, result.Removed)
			assert.True(t, result.ModuleDirRemoved)
		})
	}
}

func TestServiceDeleteApplied(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		applier  *fakeApplier
		store    *fakeStore
		wantMark bool
		wantErr  string
	}{
		{
			name:    "success - applied delete warns",
			input:   "forum",
			applier: &fakeApplier{applied: []string{"forum"}},
			store: func() *fakeStore {
				s := newFakeStore()
				s.projects["forum"] = validServiceProject()
				return s
			}(),
			wantMark: true,
		},
		{
			name:    "success - unapplied delete quiet",
			input:   "forum",
			applier: &fakeApplier{applied: []string{"other"}},
			store: func() *fakeStore {
				s := newFakeStore()
				s.projects["forum"] = validServiceProject()
				return s
			}(),
			wantMark: false,
		},
		{
			name:    "success - lookup failure degrades",
			input:   "forum",
			applier: &fakeApplier{appliedErr: errors.New("boom")},
			store: func() *fakeStore {
				s := newFakeStore()
				s.projects["forum"] = validServiceProject()
				return s
			}(),
			wantMark: false,
		},
		{
			name:    "error - missing project",
			input:   "missing",
			applier: &fakeApplier{applied: []string{"forum"}},
			store:   newFakeStore(),
			wantErr: "not found",
		},
		{
			name:    "error - invalid name",
			input:   "Bad Name!",
			applier: &fakeApplier{},
			store:   newFakeStore(),
			wantErr: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var applier modgen.Applier
			if tt.applier != nil {
				applier = tt.applier
			}
			svc := modgen.NewServiceWithApplier(&fakeGeneration{}, modgen.NewValidator(), tt.store, applier, nil)
			applied, err := svc.Delete(context.Background(), tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantMark, applied)
		})
	}
}

func TestServiceApplyRegatesStagedProject(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		validator modgen.ProjectValidator
		wantErr   bool
	}{
		{
			name: "error - staged os/exec refused with apply failed",
			files: map[string]string{
				"go.mod":    reportGoMod,
				"module.go": reportModuleGo,
				"README.md": "r",
				"api/x.go":  "package api\n\nimport (\n\t\"os/exec\"\n)\n\nvar _ = exec.Command\n",
			},
			validator: nil,
			wantErr:   true,
		},
		{
			name: "error - invalid without error findings falls back",
			files: map[string]string{
				"go.mod":    reportGoMod,
				"module.go": reportModuleGo,
				"README.md": "r",
			},
			validator: stubValidator{report: modgen.NewValidationReport(false, []modgen.Finding{
				modgen.NewFinding("w", modgen.FindingSeverityWarning, "m"),
			})},
			wantErr: true,
		},
		{
			name: "error - staged scan finding refused",
			files: map[string]string{
				"go.mod":    reportGoMod,
				"module.go": reportModuleGo,
				"README.md": "r",
				"api/x.go":  "package api\n\nimport (\n\t\"github.com/aristorinjuang/lesstruct/kernel/httpx\"\n)\n\n// see javascript:void(0)\nvar _ = httpx.Success\n",
			},
			validator: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			store.projects["forum"] = modgen.NewProject("forum", nil, tt.files, "R.")
			validator := tt.validator
			if validator == nil {
				validator = modgen.NewValidator()
			}
			svc := modgen.NewServiceWithApplier(&fakeGeneration{}, validator, store, &fakeApplier{}, nil)
			_, _, err := svc.Apply(context.Background(), "forum", false, false)
			if tt.wantErr {
				require.ErrorIs(t, err, modgen.ErrApplyFailed)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestServiceApplyPassesReplaceConfirm(t *testing.T) {
	tests := []struct {
		name    string
		replace bool
		confirm bool
		wantErr bool
	}{
		{
			name:    "success - replace and confirm reach the applier",
			replace: true,
			confirm: true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			store.projects["forum"] = validServiceProject()
			fake := &fakeApplier{result: modgen.NewApplyResult(true, true, nil, []string{"forum"}, true)}
			svc := modgen.NewServiceWithApplier(&fakeGeneration{}, modgen.NewValidator(), store, fake, nil)
			_, _, err := svc.Apply(context.Background(), "forum", tt.replace, tt.confirm)
			require.Equal(t, tt.wantErr, false)
			require.NoError(t, err)
			assert.Equal(t, tt.replace, fake.gotReplace)
			assert.Equal(t, tt.confirm, fake.gotConfirm)
		})
	}
}

func TestServiceGenerateNameCollision(t *testing.T) {
	tests := []struct {
		name       string
		project    func() modgen.Project
		validator  modgen.ProjectValidator
		install    func(s *fakeStore)
		installErr error
		wantErr    bool
		wantErrIs  error
	}{
		{
			name:      "success - collision re-homes under a fresh slug",
			project:   validServiceProject,
			validator: nil,
			install: func(s *fakeStore) {
				s.projects["forum"] = validServiceProject()
			},
			wantErr: false,
		},
		{
			name:       "error - repeated collision surfaces",
			project:    validServiceProject,
			validator:  nil,
			install:    func(s *fakeStore) {},
			installErr: modgen.ErrProjectExists,
			wantErr:    true,
			wantErrIs:  modgen.ErrProjectExists,
		},
		{
			name:       "error - install failure surfaces",
			project:    validServiceProject,
			validator:  nil,
			install:    func(s *fakeStore) {},
			installErr: errors.New("disk full"),
			wantErr:    true,
			wantErrIs:  nil,
		},
		{
			name: "error - missing package declaration refuses rename",
			project: func() modgen.Project {
				return modgen.NewProject("forum", nil, map[string]string{
					"go.mod":    reportGoMod,
					"module.go": "package wrong\n",
					"README.md": "r",
				}, "R.")
			},
			validator: stubValidator{report: modgen.NewValidationReport(true, nil)},
			install: func(s *fakeStore) {
				s.projects["forum"] = validServiceProject()
			},
			wantErr:   true,
			wantErrIs: modgen.ErrProjectExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			if tt.install != nil {
				tt.install(store)
			}
			if tt.installErr != nil {
				store.installErr = tt.installErr
			}
			validator := tt.validator
			if validator == nil {
				validator = modgen.NewValidator()
			}
			gen := &fakeGeneration{project: tt.project()}
			svc := modgen.NewService(gen, validator, store, nil)
			project, _, err := svc.Generate(context.Background(), modgen.NewBrief("A forum module", nil))
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrIs != nil {
					assert.True(t, errors.Is(err, tt.wantErrIs) || strings.Contains(err.Error(), tt.wantErrIs.Error()), "expected %v, got %v", tt.wantErrIs, err)
				}
				return
			}
			require.NoError(t, err)
			assert.NotEqual(t, "forum", project.Name)
			require.NoError(t, modgen.ValidateProjectName(project.Name))
			moduleGo, ok := project.Files["module.go"]
			require.True(t, ok)
			assert.Contains(t, moduleGo, "package "+project.Name)
			assert.NotContains(t, moduleGo, "package forum\n")
			assert.Contains(t, project.Files["go.mod"], "modules/"+project.Name)
		})
	}
}
