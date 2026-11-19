package plugingen_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/plugingen"
	"github.com/stretchr/testify/assert"
)

func TestModule(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "success - module metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := plugingen.New()
			assert.Equal(t, "plugingen", m.Name())
			assert.Equal(t, "0.1.0", m.Version())
			assert.Nil(t, m.Requires())
			assert.NotNil(t, m.Handler())
		})
	}
}
