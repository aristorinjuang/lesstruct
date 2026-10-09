package modules_test

import (
	"fmt"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/modules"
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubModule is a kernel.Module with programmable edges for prune tests.
type stubModule struct {
	name     string
	requires []string
}

func (m *stubModule) Name() string               { return m.name }
func (m *stubModule) Version() string            { return "test" }
func (m *stubModule) Requires() []string         { return m.requires }
func (m *stubModule) Optional() []string         { return nil }
func (m *stubModule) Init(_ kernel.Kernel) error { return nil }
func (m *stubModule) Shutdown() error            { return nil }

func testCandidates() []kernel.Module {
	return []kernel.Module{
		&stubModule{name: "users"},
		&stubModule{name: "auth", requires: []string{"users"}},
		&stubModule{name: "comments", requires: []string{"users"}},
		&stubModule{name: "adminpanel", requires: []string{"auth"}},
		&stubModule{name: "media"},
	}
}

func TestApplyDisabled(t *testing.T) {
	tests := []struct {
		name       string
		candidates []kernel.Module
		disabled   []string
		wantKept   []string
		wantWarns  []string
		wantErr    bool
	}{
		{
			name:       "success - empty disabled registers everything",
			candidates: testCandidates(),
			disabled:   nil,
			wantKept:   []string{"adminpanel", "auth", "comments", "media", "users"},
		},
		{
			name:       "success - direct disable warns and skips",
			candidates: testCandidates(),
			disabled:   []string{"comments"},
			wantKept:   []string{"adminpanel", "auth", "media", "users"},
			wantWarns: []string{
				`module "comments" disabled by [modules] config — skipping registration`,
			},
		},
		{
			name:       "success - cascade drops dependents with cause",
			candidates: testCandidates(),
			disabled:   []string{"users"},
			wantKept:   []string{"media"},
			wantWarns: []string{
				`module "users" disabled by [modules] config — skipping registration`,
				`module "auth" disabled: requires disabled module "users" — skipping registration`,
				`module "comments" disabled: requires disabled module "users" — skipping registration`,
				`module "adminpanel" disabled: requires disabled module "auth" — skipping registration`,
			},
		},
		{
			name:       "success - unknown names warn and register everything",
			candidates: testCandidates(),
			disabled:   []string{"no-such-module"},
			wantKept:   []string{"adminpanel", "auth", "comments", "media", "users"},
			wantWarns: []string{
				`unknown module "no-such-module" in [modules] disabled — ignoring`,
			},
		},
		{
			name: "success - absent but enabled dep registers for fail-fast",
			candidates: []kernel.Module{
				&stubModule{name: "needy", requires: []string{"ghost"}},
			},
			disabled: []string{"other"},
			wantKept: []string{"needy"},
			wantWarns: []string{
				`unknown module "other" in [modules] disabled — ignoring`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := kernel.NewHost(nil).Registry()
			var warns []string
			warnf := func(format string, args ...any) {
				warns = append(warns, fmt.Sprintf(format, args...))
			}

			kept, err := modules.ApplyDisabled(registry, tt.candidates, tt.disabled, warnf)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantKept, registry.Names())
			for _, name := range tt.wantKept {
				assert.True(t, kept[name], "kept set must contain %q", name)
			}
			assert.Equal(t, tt.wantWarns, warns)
		})
	}
}

func TestApplyDisabled_AbsentDepFailsFastAtInit(t *testing.T) {
	registry := kernel.NewHost(nil).Registry()
	kept, err := modules.ApplyDisabled(
		registry,
		[]kernel.Module{&stubModule{name: "needy", requires: []string{"ghost"}}},
		nil,
		func(string, ...any) {},
	)
	require.NoError(t, err, "prune keeps absent-but-enabled deps")
	assert.True(t, kept["needy"])

	err = registry.InitAll(kernel.NewHost(nil))
	require.Error(t, err, "InitAll must fail fast on the missing dependency")
	assert.ErrorIs(t, err, kernel.ErrMissingDependency)
}

func TestApplyDisabled_DuplicateRegisterFails(t *testing.T) {
	registry := kernel.NewHost(nil).Registry()
	_, err := modules.ApplyDisabled(
		registry,
		[]kernel.Module{
			&stubModule{name: "media"},
			&stubModule{name: "media"},
		},
		nil,
		func(string, ...any) {},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, kernel.ErrDuplicateModule)
}

func TestEffectiveCommentsEnabled(t *testing.T) {
	tests := []struct {
		name           string
		legacyEnabled  bool
		modulesDisable []string
		kept           map[string]bool
		expected       bool
	}{
		{
			name:           "success - legacy on nothing disabled",
			legacyEnabled:  true,
			modulesDisable: nil,
			kept:           map[string]bool{"comments": true},
			expected:       true,
		},
		{
			name:           "success - legacy off stays off",
			legacyEnabled:  false,
			modulesDisable: nil,
			kept:           map[string]bool{"comments": true},
			expected:       false,
		},
		{
			name:           "success - module disable equals legacy toggle",
			legacyEnabled:  true,
			modulesDisable: []string{"comments"},
			kept:           map[string]bool{},
			expected:       false,
		},
		{
			name:           "success - cascade drop unmounts",
			legacyEnabled:  true,
			modulesDisable: []string{"users"},
			kept:           map[string]bool{"media": true},
			expected:       false,
		},
		{
			name:           "success - unrelated disable keeps comments",
			legacyEnabled:  true,
			modulesDisable: []string{"textgen", "nope"},
			kept:           map[string]bool{"comments": true, "users": true},
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, modules.EffectiveCommentsEnabled(tt.legacyEnabled, tt.modulesDisable, tt.kept))
		})
	}
}
