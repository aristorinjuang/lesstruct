package posttype_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	kernelposttype "github.com/aristorinjuang/lesstruct/kernel/posttype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubProvider struct {
	name  string
	types []kernel.PostTypeDef
}

func (s *stubProvider) Name() string {
	return s.name
}

func (s *stubProvider) Version() string {
	return "test"
}

func (s *stubProvider) Requires() []string {
	return nil
}

func (s *stubProvider) Optional() []string {
	return nil
}

func (s *stubProvider) Init(_ kernel.Kernel) error {
	return nil
}

func (s *stubProvider) Shutdown() error {
	return nil
}

func (s *stubProvider) PostTypes() []kernel.PostTypeDef {
	return s.types
}

func newStubProvider(
	name string,
	types []kernel.PostTypeDef,
) *stubProvider {
	return &stubProvider{
		name:  name,
		types: types,
	}
}

type plainModule struct {
	name string
}

func (m *plainModule) Name() string {
	return m.name
}

func (m *plainModule) Version() string {
	return "test"
}

func (m *plainModule) Requires() []string {
	return nil
}

func (m *plainModule) Optional() []string {
	return nil
}

func (m *plainModule) Init(_ kernel.Kernel) error {
	return nil
}

func (m *plainModule) Shutdown() error {
	return nil
}

func TestRegistryRegisterGet(t *testing.T) {
	registry := kernelposttype.NewRegistry()
	require.NoError(t, registry.Register(kernel.PostTypeDef{Slug: "post", Name: "Post", Protected: true}))

	def, ok := registry.GetBySlug("post")
	require.True(t, ok)
	assert.Equal(t, "Post", def.Name)
	assert.True(t, registry.IsProtected("post"))
	assert.False(t, registry.IsProtected("page"))

	_, ok = registry.GetBySlug("missing")
	assert.False(t, ok)
}

func TestRegistryRegisterErrors(t *testing.T) {
	tests := []struct {
		name    string
		defs    []kernel.PostTypeDef
		wantErr bool
		errIs   error
	}{
		{
			name:    "success - distinct slugs",
			defs:    []kernel.PostTypeDef{{Slug: "post"}, {Slug: "page"}},
			wantErr: false,
		},
		{
			name:    "error - duplicate slug",
			defs:    []kernel.PostTypeDef{{Slug: "post"}, {Slug: "post"}},
			wantErr: true,
			errIs:   kernelposttype.ErrDuplicatePostType,
		},
		{
			name:    "error - empty slug",
			defs:    []kernel.PostTypeDef{{Slug: ""}},
			wantErr: true,
			errIs:   kernelposttype.ErrEmptyPostTypeSlug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := kernelposttype.NewRegistry()
			var err error
			for _, def := range tt.defs {
				err = registry.Register(def)
				if err != nil {
					break
				}
			}
			if tt.wantErr {
				require.Error(t, err)
				if tt.errIs != nil {
					assert.ErrorIs(t, err, tt.errIs)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRegistryAllSlugs(t *testing.T) {
	registry := kernelposttype.NewRegistry()
	require.NoError(t, registry.Register(kernel.PostTypeDef{Slug: "page", Name: "Page"}))
	require.NoError(t, registry.Register(kernel.PostTypeDef{Slug: "post", Name: "Post", Protected: true}))

	assert.Equal(t, []string{"page", "post"}, registry.Slugs())

	all := registry.All()
	require.Len(t, all, 2)
	assert.Equal(t, "page", all[0].Slug)
	assert.Equal(t, "post", all[1].Slug)
}

func TestRegistryCollectFrom(t *testing.T) {
	tests := []struct {
		name      string
		providers []kernel.Module
		expected  []string
		wantErr   bool
	}{
		{
			name: "success - collects across providers in name order",
			providers: []kernel.Module{
				newStubProvider("b-types", []kernel.PostTypeDef{{Slug: "page"}}),
				newStubProvider("a-types", []kernel.PostTypeDef{{Slug: "post", Protected: true}}),
			},
			expected: []string{"page", "post"},
			wantErr:  false,
		},
		{
			name: "error - duplicate across providers aborts",
			providers: []kernel.Module{
				newStubProvider("a-types", []kernel.PostTypeDef{{Slug: "post"}}),
				newStubProvider("b-types", []kernel.PostTypeDef{{Slug: "post"}}),
			},
			wantErr: true,
		},
		{
			name:      "success - non providers ignored",
			providers: []kernel.Module{&plainModule{name: "plain"}},
			expected:  nil,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			moduleRegistry := kernel.NewRegistry()
			for _, provider := range tt.providers {
				require.NoError(t, moduleRegistry.Register(provider))
			}
			registry := kernelposttype.NewRegistry()
			err := registry.CollectFrom(moduleRegistry)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, kernelposttype.ErrDuplicatePostType)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, registry.Slugs())
		})
	}
}

func TestRegistryCollectFromNil(t *testing.T) {
	registry := kernelposttype.NewRegistry()
	err := registry.CollectFrom(nil)
	require.Error(t, err)
}
