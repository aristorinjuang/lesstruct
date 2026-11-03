package privacy_test

import (
	"testing"

	privacymodule "github.com/aristorinjuang/lesstruct/modules/privacy"
	privacydomain "github.com/aristorinjuang/lesstruct/modules/privacy/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleDefaults(t *testing.T) {
	m := privacymodule.New()
	assert.Equal(t, "privacy", m.Name())
	assert.Equal(t, "0.1.0", m.Version())
	assert.Nil(t, m.Requires())
	assert.Nil(t, m.Optional())
	require.NoError(t, m.Init(nil))
	require.NoError(t, m.Shutdown())
	require.NoError(t, m.AdminManifest().Validate())
	require.NotNil(t, m.Service())
}

func TestModuleWithService(t *testing.T) {
	svc := privacydomain.NewService(nil, nil)
	m := privacymodule.New(
		privacymodule.WithService(svc),
		privacymodule.WithLogger(nil),
	)
	assert.Equal(t, svc, m.Service())
}

func TestModuleWithServiceNil(t *testing.T) {
	m := privacymodule.New(privacymodule.WithService(nil))
	require.NotNil(t, m.Service())
}
