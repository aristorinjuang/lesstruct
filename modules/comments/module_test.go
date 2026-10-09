package comments_test

import (
	"io/fs"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/comments"
	commentapi "github.com/aristorinjuang/lesstruct/modules/comments/api"
	commentagent "github.com/aristorinjuang/lesstruct/modules/comments/api/agent"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	"github.com/aristorinjuang/lesstruct/modules/comments/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, comments.Name, module.Name())
	assert.Equal(t, comments.Version, module.Version())
	assert.Equal(t, []string{"users"}, module.Requires())
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
		assert.Contains(t, names, "000001_comments.up.sql")
		assert.Contains(t, names, "000001_comments.down.sql")
	}
}

func TestModule_AdminManifest(t *testing.T) {
	module := testutil.NewModule()

	manifest := module.AdminManifest()
	require.NoError(t, manifest.Validate())
	require.NotEmpty(t, manifest.MenuEntries)
}

func TestModule_ImplementsMigrator(t *testing.T) {
	var _ kernel.Migrator = testutil.NewModule()
}

type stubRoles struct {
	comments map[string]bool
}

func (s *stubRoles) CanComment(name string) bool {
	return s.comments[name]
}

func TestModule_Handlers(t *testing.T) {
	tests := []struct {
		name string
		opts []comments.Option
	}{
		{
			name: "success - bare module builds handlers",
			opts: nil,
		},
		{
			name: "success - full options build gated handlers",
			opts: []comments.Option{
				comments.WithService(commentdomain.NewService(nil)),
				comments.WithContentResolver(nil),
				comments.WithAgentContentLookup(nil),
				comments.WithRoles(&stubRoles{comments: map[string]bool{"Commentator": true}}),
				comments.WithAgentRoles(&stubRoles{comments: map[string]bool{}}),
				comments.WithLogger(nil),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := comments.New(tt.opts...)

			require.NotNil(t, module.BrowserHandler())
			require.NotNil(t, module.AgentHandler())
		})
	}
}

func TestModule_Service(t *testing.T) {
	service := commentdomain.NewService(nil)
	module := comments.New(comments.WithService(service))

	assert.Equal(t, service, module.Service())
	assert.Nil(t, testutil.NewModule().Service())
}

var (
	_ commentapi.RoleCapabilities   = (*stubRoles)(nil)
	_ commentagent.RoleCapabilities = (*stubRoles)(nil)
)
