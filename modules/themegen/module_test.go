package themegen_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/themegen"
	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
	"github.com/stretchr/testify/assert"
)

// stubReloader records Reload calls for module wiring tests.
type stubReloader struct {
	calls int
	err   error
}

func (s *stubReloader) Reload(context.Context) error {
	s.calls++
	return s.err
}

// stubSnapshotter renders a fixed document for module wiring tests.
type stubSnapshotter struct {
	doc string
	err error
}

func (s stubSnapshotter) SnapshotTheme(name string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.doc, nil
}

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
			m := themegen.New(
				themegen.WithEdition(themegendomain.EditionBlog),
			)
			assert.Equal(t, "themegen", m.Name())
			assert.Equal(t, "0.1.0", m.Version())
			assert.Nil(t, m.Requires())
			assert.NotNil(t, m.Handler())
		})
	}
}

func TestModuleWithPreviewKey(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "success - preview key option builds handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := themegen.New(
				themegen.WithEdition(themegendomain.EditionBlog),
				themegen.WithPreviewKey([]byte("preview-test-key-32-bytes-long!!"), false),
			)
			assert.NotNil(t, m.Handler())
		})
	}
}
func TestModuleWithReloader(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "success - reloader option builds handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := themegen.New(
				themegen.WithEdition(themegendomain.EditionBlog),
				themegen.WithReloader(&stubReloader{}),
			)
			assert.NotNil(t, m.Handler())
		})
	}
}

func TestModuleWithSnapshotRenderer(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "success - snapshot renderer option builds handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := themegen.New(
				themegen.WithEdition(themegendomain.EditionBlog),
				themegen.WithSnapshotRenderer(stubSnapshotter{doc: "<html></html>"}),
			)
			assert.NotNil(t, m.Handler())
		})
	}
}
