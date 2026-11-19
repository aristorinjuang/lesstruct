package plugingen_test

import (
	"context"
	"errors"
	"testing"

	plugingen "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeGeneration struct {
	project   plugingen.Project
	err       error
	repair    plugingen.Project
	repairErr error
	calls     int
}

func (f *fakeGeneration) GenerateProject(_ context.Context, _ plugingen.Brief, _ plugingen.PluginSources) (plugingen.Project, error) {
	return plugingen.Project{}, errors.New("unused")
}

func (f *fakeGeneration) GenerateProject2(_ context.Context, _ plugingen.Brief, _ plugingen.PluginSources) (plugingen.Project, error) {
	if f.err != nil {
		return plugingen.Project{}, f.err
	}
	return f.project, nil
}

func (f *fakeGeneration) RepairProject(_ context.Context, _ plugingen.Brief, _ plugingen.PluginSources, _ plugingen.Project, _ []string) (plugingen.Project, error) {
	f.calls++
	if f.repairErr != nil {
		return plugingen.Project{}, f.repairErr
	}
	if f.repair.Name != "" {
		return f.repair, nil
	}
	return f.project, nil
}

// adapter to satisfy the interface without name clash
type genAdapter struct {
	inner *fakeGeneration
}

func (g *genAdapter) GenerateProject(ctx context.Context, brief plugingen.Brief, sources plugingen.PluginSources) (plugingen.Project, error) {
	return g.inner.GenerateProject2(ctx, brief, sources)
}

func (g *genAdapter) RepairProject(ctx context.Context, brief plugingen.Brief, sources plugingen.PluginSources, project plugingen.Project, problems []string) (plugingen.Project, error) {
	return g.inner.RepairProject(ctx, brief, sources, project, problems)
}

type fakeStore struct {
	projects   map[string]plugingen.Project
	installErr error
	listErr    error
}

func newFakeStore() *fakeStore {
	return &fakeStore{projects: map[string]plugingen.Project{}}
}

func (f *fakeStore) Install(_ context.Context, project plugingen.Project) error {
	if f.installErr != nil {
		return f.installErr
	}
	if _, ok := f.projects[project.Name]; ok {
		return plugingen.ErrProjectExists
	}
	f.projects[project.Name] = project
	return nil
}

func (f *fakeStore) List(_ context.Context) ([]plugingen.InstalledProject, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return []plugingen.InstalledProject{}, nil
}

func (f *fakeStore) Get(_ context.Context, name string) (plugingen.Project, error) {
	if p, ok := f.projects[name]; ok {
		return p, nil
	}
	return plugingen.Project{}, plugingen.ErrProjectNotFound
}

func (f *fakeStore) Delete(_ context.Context, name string) error {
	delete(f.projects, name)
	return nil
}

func (f *fakeStore) Zip(_ context.Context, name string) ([]byte, error) {
	if _, ok := f.projects[name]; !ok {
		return nil, plugingen.ErrProjectNotFound
	}
	return []byte("zip"), nil
}

func validProject() plugingen.Project {
	return plugingen.NewProject(
		"my-plugin",
		"go",
		[]string{"before_save"},
		[]string{"log_info"},
		map[string]string{
			"main.go": "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}",
			"go.mod":  "module example",
		},
		"rationale",
	)
}

func TestServiceGenerate(t *testing.T) {
	tests := []struct {
		name       string
		brief      plugingen.Brief
		setupGen   func() *fakeGeneration
		setupStore func() *fakeStore
		wantErr    bool
	}{
		{
			name:  "success - valid generation",
			brief: plugingen.NewBrief("uppercase titles", "go", []string{"before_save"}, []string{"log_info"}),
			setupGen: func() *fakeGeneration {
				return &fakeGeneration{project: validProject()}
			},
			setupStore: func() *fakeStore {
				return newFakeStore()
			},
			wantErr: false,
		},
		{
			name:  "error - invalid brief",
			brief: plugingen.NewBrief("", "go", []string{"before_save"}, nil),
			setupGen: func() *fakeGeneration {
				return &fakeGeneration{project: validProject()}
			},
			setupStore: func() *fakeStore {
				return newFakeStore()
			},
			wantErr: true,
		},
		{
			name:  "error - generation unavailable",
			brief: plugingen.NewBrief("desc", "go", []string{"before_save"}, nil),
			setupGen: func() *fakeGeneration {
				return nil
			},
			setupStore: func() *fakeStore {
				return newFakeStore()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := tt.setupStore()
			var gen plugingen.PluginGenerationService
			if fg := tt.setupGen(); fg != nil {
				gen = &genAdapter{inner: fg}
			}
			svc := plugingen.NewService(gen, plugingen.NewValidator(), store, nil)
			project, report, err := svc.Generate(context.Background(), tt.brief)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.True(t, report.Valid)
			assert.Equal(t, "my-plugin", project.Name)
		})
	}
}

func TestServiceListValidateGetDeleteZip(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - store operations",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			gen := &fakeGeneration{project: validProject()}
			svc := plugingen.NewService(&genAdapter{inner: gen}, plugingen.NewValidator(), store, nil)
			brief := plugingen.NewBrief("desc", "go", []string{"before_save"}, nil)
			project, _, err := svc.Generate(context.Background(), brief)
			require.NoError(t, err)
			_ = project
			list, err := svc.List(context.Background())
			require.NoError(t, err)
			assert.NotNil(t, list)
			report := svc.Validate(context.Background(), validProject())
			assert.True(t, report.Valid)
			got, err := svc.Get(context.Background(), "my-plugin")
			require.NoError(t, err)
			assert.Equal(t, "my-plugin", got.Name)
			zipped, err := svc.Zip(context.Background(), "my-plugin")
			require.NoError(t, err)
			assert.NotEmpty(t, zipped)
			require.NoError(t, svc.Delete(context.Background(), "my-plugin"))
			_, err = svc.Get(context.Background(), "my-plugin")
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, false)
		})
	}
}

func TestServiceGenerateRepair(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "error - still failing after repair",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bad := plugingen.NewProject(
				"bad-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{
					"main.go": "package main\nfunc main() {}",
					"go.mod":  "module x",
				},
				"",
			)
			badRepair := plugingen.NewProject(
				"Also Bad!",
				"",
				[]string{"before_save"},
				nil,
				map[string]string{
					"main.go": "package main\nfunc main() {}",
					"go.mod":  "module x",
				},
				"",
			)
			gen := &fakeGeneration{project: bad, repair: badRepair}
			store := newFakeStore()
			svc := plugingen.NewService(&genAdapter{inner: gen}, plugingen.NewValidator(), store, nil)
			brief := plugingen.NewBrief("a an of fix plugin hook save now", "go", []string{"before_save"}, nil)
			_, _, err := svc.Generate(context.Background(), brief)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
