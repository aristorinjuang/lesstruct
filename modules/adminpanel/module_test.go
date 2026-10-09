package adminpanel_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/adminpanel"
	"github.com/aristorinjuang/lesstruct/modules/adminpanel/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, adminpanel.Name, module.Name())
	assert.Equal(t, adminpanel.Version, module.Version())
	assert.Equal(t, []string{"auth"}, module.Requires())
	assert.Empty(t, module.Optional())
	require.NoError(t, module.Init(testutil.Kernel()))
	require.NoError(t, module.Shutdown())
}

func TestModule_AdminManifest(t *testing.T) {
	module := testutil.NewModule()

	manifest := module.AdminManifest()
	require.NoError(t, manifest.Validate())
	assert.Empty(t, manifest.MenuEntries)
}

func TestModule_SPAHandler(t *testing.T) {
	tests := []struct {
		name string
		opts []adminpanel.Option
	}{
		{
			name: "success - bare module builds handler",
			opts: nil,
		},
		{
			name: "success - full options build configured handler",
			opts: []adminpanel.Option{
				adminpanel.WithDevMode(true),
				adminpanel.WithAdminDevURL("http://localhost:5173"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := adminpanel.New(tt.opts...)

			require.NotNil(t, module.SPAHandler())
		})
	}
}

func TestModule_SPAHandler_ServesProductionBuild(t *testing.T) {
	module := testutil.NewModule()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	module.SPAHandler().ServeAdmin(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Lesstruct")
}
