package speech_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/speech"
	"github.com/aristorinjuang/lesstruct/modules/speech/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, speech.Name, module.Name())
	assert.Equal(t, speech.Version, module.Version())
	assert.Empty(t, module.Requires())
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

func TestModule_Handlers(t *testing.T) {
	tests := []struct {
		name string
		opts []speech.Option
	}{
		{
			name: "success - bare module builds handler",
			opts: nil,
		},
		{
			name: "success - full options build configured handler",
			opts: []speech.Option{
				speech.WithTranscriber(nil),
				speech.WithEnhancer(nil),
				speech.WithLogger(nil),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := speech.New(tt.opts...)

			require.NotNil(t, module.Handler())
			assert.Nil(t, module.Service())
		})
	}
}
