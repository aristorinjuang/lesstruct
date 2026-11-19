package plugingen_test

import (
	"context"
	"errors"
	"testing"

	plugingen "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeBuilder struct {
	result plugingen.BuildResult
	err    error
	got    plugingen.Project
	calls  int
}

func (f *fakeBuilder) Build(_ context.Context, project plugingen.Project) (plugingen.BuildResult, error) {
	f.calls++
	f.got = project
	if f.err != nil {
		return plugingen.BuildResult{}, f.err
	}
	return f.result, nil
}

func TestServiceBuild(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		setup       func(store *fakeStore) plugingen.Builder
		wantResult  plugingen.BuildResult
		wantErr     bool
		wantErrIs   error
	}{
		{
			name:        "success - installed without restart",
			projectName: "my-plugin",
			setup: func(store *fakeStore) plugingen.Builder {
				store.projects["my-plugin"] = validProject()
				return &fakeBuilder{result: plugingen.NewBuildResult(true, false)}
			},
			wantResult: plugingen.NewBuildResult(true, false),
			wantErr:    false,
		},
		{
			name:        "success - installed with restart",
			projectName: "my-plugin",
			setup: func(store *fakeStore) plugingen.Builder {
				store.projects["my-plugin"] = validProject()
				return &fakeBuilder{result: plugingen.NewBuildResult(true, true)}
			},
			wantResult: plugingen.NewBuildResult(true, true),
			wantErr:    false,
		},
		{
			name:        "error - invalid name",
			projectName: "Bad!",
			setup: func(_ *fakeStore) plugingen.Builder {
				return &fakeBuilder{result: plugingen.NewBuildResult(true, false)}
			},
			wantErr:   true,
			wantErrIs: plugingen.ErrProjectNotFound,
		},
		{
			name:        "error - empty name",
			projectName: "   ",
			setup: func(_ *fakeStore) plugingen.Builder {
				return &fakeBuilder{result: plugingen.NewBuildResult(true, false)}
			},
			wantErr:   true,
			wantErrIs: plugingen.ErrProjectNotFound,
		},
		{
			name:        "error - builder unavailable",
			projectName: "my-plugin",
			setup: func(store *fakeStore) plugingen.Builder {
				store.projects["my-plugin"] = validProject()
				return nil
			},
			wantErr:   true,
			wantErrIs: plugingen.ErrBuildUnavailable,
		},
		{
			name:        "error - missing project",
			projectName: "my-plugin",
			setup: func(_ *fakeStore) plugingen.Builder {
				return &fakeBuilder{result: plugingen.NewBuildResult(true, false)}
			},
			wantErr:   true,
			wantErrIs: plugingen.ErrProjectNotFound,
		},
		{
			name:        "error - builder fails",
			projectName: "my-plugin",
			setup: func(store *fakeStore) plugingen.Builder {
				store.projects["my-plugin"] = validProject()
				return &fakeBuilder{err: errors.New("tinygo boom")}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			builder := tt.setup(store)
			svc := plugingen.NewServiceWithBuilder(nil, plugingen.NewValidator(), store, builder, nil)
			result, err := svc.Build(context.Background(), tt.projectName)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrIs != nil {
					assert.ErrorIs(t, err, tt.wantErrIs)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantResult, result)
		})
	}
}

func TestServiceBuildNoStore(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "error - nil store reads as not found",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := plugingen.NewServiceWithBuilder(nil, plugingen.NewValidator(), nil, &fakeBuilder{}, nil)
			_, err := svc.Build(context.Background(), "my-plugin")
			if tt.wantErr {
				require.ErrorIs(t, err, plugingen.ErrProjectNotFound)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestServiceBuildPassesStoredGrants(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - builder receives stored grants",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stored := validProject()
			stored.HTTPPatterns = []string{"https://api.example.com/*"}
			stored.DBPermissions = []string{"read:content"}
			store := newFakeStore()
			store.projects["my-plugin"] = stored
			builder := &fakeBuilder{result: plugingen.NewBuildResult(true, false)}
			svc := plugingen.NewServiceWithBuilder(nil, plugingen.NewValidator(), store, builder, nil)
			_, err := svc.Build(context.Background(), "my-plugin")
			require.NoError(t, err)
			assert.Equal(t, stored.HTTPPatterns, builder.got.HTTPPatterns)
			assert.Equal(t, stored.DBPermissions, builder.got.DBPermissions)
			require.Equal(t, false, tt.wantErr)
		})
	}
}

func TestNewBuildResult(t *testing.T) {
	tests := []struct {
		name     string
		input    plugingen.BuildResult
		expected plugingen.BuildResult
		wantErr  bool
	}{
		{
			name:     "success - build result",
			input:    plugingen.NewBuildResult(true, true),
			expected: plugingen.NewBuildResult(true, true),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}
