package media_test

import (
	"io/fs"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/conformance"
	"github.com/aristorinjuang/lesstruct/modules/media"
	"github.com/aristorinjuang/lesstruct/modules/media/api"
	"github.com/aristorinjuang/lesstruct/modules/media/api/agent"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	"github.com/aristorinjuang/lesstruct/modules/media/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, media.Name, module.Name())
	assert.Equal(t, media.Version, module.Version())
	assert.Empty(t, module.Requires())
	assert.Empty(t, module.Optional())
	require.NoError(t, module.Init(testutil.Kernel()))
	require.NoError(t, module.Shutdown())
}

func TestModule_MigrationsLayout(t *testing.T) {
	module := testutil.NewModule()

	migrations := module.Migrations()
	require.NotNil(t, migrations)
	for _, dir := range []string{"sqlite", "postgresql", "mysql"} {
		entries, err := fs.ReadDir(migrations, dir)
		require.NoError(t, err, "driver dir %q must exist", dir)
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		assert.Contains(t, names, "000001_media.up.sql")
		assert.Contains(t, names, "000001_media.down.sql")
	}
}

func TestModule_AdminManifest(t *testing.T) {
	module := testutil.NewModule()

	manifest := module.AdminManifest()
	require.NoError(t, manifest.Validate())
	require.NotEmpty(t, manifest.MenuEntries)

	// The third-party conformance fixture covers the provider contract.
	conformance.CheckAdminExtension(t, module)
}

func TestModule_ImplementsMigrator(t *testing.T) {
	var _ kernel.Migrator = testutil.NewModule()
}

type stubRoles struct {
	media map[string]bool
}

func (s *stubRoles) CanMedia(name string) bool {
	return s.media[name]
}

func TestModule_Handlers(t *testing.T) {
	tests := []struct {
		name string
		opts []media.Option
	}{
		{
			name: "success - bare module builds discard handlers",
			opts: nil,
		},
		{
			name: "success - full options build gated handlers",
			opts: []media.Option{
				media.WithService(mediadomain.NewService(nil, nil, nil)),
				media.WithImageGeneration(nil),
				media.WithRoles(&stubRoles{media: map[string]bool{"Editor": true}}),
				media.WithAgentRoles(&stubRoles{media: map[string]bool{}}),
				media.WithLogger(nil),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := media.New(tt.opts...)

			require.NotNil(t, module.BrowserHandler())
			require.NotNil(t, module.AgentHandler())
		})
	}
}

func TestModule_Service(t *testing.T) {
	service := mediadomain.NewService(nil, nil, nil)
	module := media.New(media.WithService(service))

	assert.Equal(t, service, module.Service())
	assert.Nil(t, testutil.NewModule().Service())
}

var (
	_ api.RoleCapabilities   = (*stubRoles)(nil)
	_ agent.RoleCapabilities = (*stubRoles)(nil)
)
