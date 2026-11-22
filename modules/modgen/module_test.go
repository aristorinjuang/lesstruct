package modgen_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/modgen"
	modgendomain "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - module metadata",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := modgen.New()
			assert.Equal(t, "modgen", m.Name())
			assert.Equal(t, "0.1.0", m.Version())
			assert.Nil(t, m.Requires())
			assert.Nil(t, m.Optional())
			assert.NotNil(t, m.Handler())
			assert.Nil(t, m.Service())
			require.NoError(t, m.Init(kernel.NewHost(nil)))
			require.NoError(t, m.Shutdown())
			require.Equal(t, tt.wantErr, false)
			require.NoError(t, m.AdminManifest().Validate())
		})
	}
}

func TestModuleWithService(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - service adopted",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := modgendomain.NewService(nil, modgendomain.NewValidator(), nil, nil)
			m := modgen.New(
				modgen.WithService(svc),
				modgen.WithLogger(nil),
				modgen.WithApplyTimeout(0),
			)
			require.Equal(t, tt.wantErr, false)
			assert.NotNil(t, m.Service())
			assert.NotNil(t, m.Handler())
		})
	}
}
