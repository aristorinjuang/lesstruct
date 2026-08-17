package role_test

import (
	"errors"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/role"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService_SeedsBuiltInRoles(t *testing.T) {
	s := role.NewService()

	tests := []struct {
		name string
		role string
		check func(*role.Service) bool
	}{
		{
			name: "admin is assignable and is the admin superuser",
			role: "Admin",
			check: func(s *role.Service) bool {
				return s.IsAssignable("Admin") && s.IsAdmin("Admin")
			},
		},
		{
			name: "contributor is assignable and manages every type",
			role: "Contributor",
			check: func(s *role.Service) bool {
				return s.IsAssignable("Contributor") && !s.IsAdmin("Contributor") &&
					s.CanManageType("Contributor", "post") && s.CanManageType("Contributor", "article")
			},
		},
		{
			name: "contributor can publish, use media, and comment",
			role: "Contributor",
			check: func(s *role.Service) bool {
				return s.CanPublish("Contributor") && s.CanMedia("Contributor") && s.CanComment("Contributor")
			},
		},
		{
			name: "commentator is assignable but manages no content types",
			role: "Commentator",
			check: func(s *role.Service) bool {
				return s.IsAssignable("Commentator") && !s.CanManageType("Commentator", "post") &&
					!s.CanManageType("Commentator", "article") && !s.CanPublish("Commentator")
			},
		},
		{
			name: "commentator keeps media and comment capabilities",
			role: "Commentator",
			check: func(s *role.Service) bool {
				return s.CanMedia("Commentator") && s.CanComment("Commentator")
			},
		},
		{
			name: "admin short-circuits every capability",
			role: "Admin",
			check: func(s *role.Service) bool {
				return s.CanPublish("Admin") && s.CanMedia("Admin") && s.CanComment("Admin") &&
					s.CanManageType("Admin", "anything")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.check(s))
		})
	}
}

func TestService_Register(t *testing.T) {
	s := role.NewService()

	tests := []struct {
		name    string
		toAdd   role.Role
		wantErr error
	}{
		{
			name: "success - new custom role",
			toAdd: role.Role{
				Name:      "Journalist",
				PostTypes: []string{"article"},
				Publish:   true,
				Media:     true,
				Comments:  true,
			},
			wantErr: nil,
		},
		{
			name:    "error - empty name",
			toAdd:   role.Role{Name: "   "},
			wantErr: role.ErrInvalidRoleName,
		},
		{
			name: "error - duplicate new role",
			toAdd: role.Role{
				Name: "Journalist",
			},
			wantErr: role.ErrDuplicateRole,
		},
		{
			name:    "error - admin is reserved",
			toAdd:   role.Role{Name: "Admin"},
			wantErr: role.ErrAdminRoleReserved,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.Register(tt.toAdd)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestService_Register_OverridesBuiltIn(t *testing.T) {
	tests := []struct {
		name          string
		override      role.Role
		wantAssign    bool
		wantAllTypes  bool
		wantPublish   bool
		manageArticle bool
		managePost    bool
	}{
		{
			name: "contributor override narrows post types and disables publish",
			override: role.Role{
				Name:      "Contributor",
				PostTypes: []string{"article"},
				Publish:   false,
			},
			wantAssign:    true,
			wantAllTypes:  false,
			wantPublish:   false,
			manageArticle: true,
			managePost:    false,
		},
		{
			name: "contributor override without post_types keeps manage-all",
			override: role.Role{
				Name:    "Contributor",
				Publish: false,
			},
			wantAssign:    true,
			wantAllTypes:  true,
			wantPublish:   false,
			manageArticle: true,
			managePost:    true,
		},
		{
			name: "commentator override grants post types",
			override: role.Role{
				Name:      "Commentator",
				PostTypes: []string{"article"},
			},
			wantAssign:    true,
			wantAllTypes:  false,
			wantPublish:   false,
			manageArticle: true,
			managePost:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := role.NewService()
			require.NoError(t, s.Register(tt.override))

			assert.Equal(t, tt.wantAssign, s.IsAssignable(tt.override.Name))
			got, err := s.Get(tt.override.Name)
			require.NoError(t, err)
			assert.Equal(t, tt.wantAllTypes, got.AllTypes)
			assert.Equal(t, tt.wantPublish, s.CanPublish(tt.override.Name))
			assert.Equal(t, tt.manageArticle, s.CanManageType(tt.override.Name, "article"))
			assert.Equal(t, tt.managePost, s.CanManageType(tt.override.Name, "post"))
		})
	}
}

func TestService_Get(t *testing.T) {
	s := role.NewService()
	require.NoError(t, s.Register(role.Role{Name: "Journalist", PostTypes: []string{"article"}}))

	tests := []struct {
		name    string
		role    string
		wantErr error
	}{
		{name: "success - built-in", role: "Admin", wantErr: nil},
		{name: "success - custom", role: "Journalist", wantErr: nil},
		{name: "error - unknown", role: "Ghost", wantErr: role.ErrRoleNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Get(tt.role)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.role, got.Name)
		})
	}
}

func TestService_GetAll(t *testing.T) {
	s := role.NewService()
	require.NoError(t, s.Register(role.Role{Name: "Journalist"}))

	all := s.GetAll()
	names := make(map[string]bool)
	for _, r := range all {
		names[r.Name] = true
	}
	assert.Len(t, all, 4)
	assert.True(t, names["Admin"])
	assert.True(t, names["Contributor"])
	assert.True(t, names["Commentator"])
	assert.True(t, names["Journalist"])
}

func TestService_UnknownRoleFailsClosed(t *testing.T) {
	s := role.NewService()

	assert.False(t, s.IsAssignable("Ghost"))
	assert.False(t, s.IsAdmin("Ghost"))
	assert.False(t, s.CanManageType("Ghost", "post"))
	assert.False(t, s.CanPublish("Ghost"))
	assert.False(t, s.CanMedia("Ghost"))
	assert.False(t, s.CanComment("Ghost"))

	got, err := s.Get("Ghost")
	require.Error(t, err)
	assert.ErrorIs(t, err, role.ErrRoleNotFound)
	assert.Empty(t, got.Name)
}

func TestService_ManageablePostTypes(t *testing.T) {
	allSlugs := []string{"post", "page", "media", "comment", "article", "event"}

	tests := []struct {
		name string
		role string
		seed func(*role.Service)
		want []string
	}{
		{
			name: "admin manages every registered slug",
			role: "Admin",
			want: allSlugs,
		},
		{
			name: "contributor manages every registered slug",
			role: "Contributor",
			want: allSlugs,
		},
		{
			name: "custom role manages only its allowlist",
			role: "Journalist",
			seed: func(s *role.Service) {
				_ = s.Register(role.Role{Name: "Journalist", PostTypes: []string{"article", "event"}})
			},
			want: []string{"article", "event"},
		},
		{
			name: "custom role allowlist drops slugs that are not registered",
			role: "Journalist",
			seed: func(s *role.Service) {
				_ = s.Register(role.Role{Name: "Journalist", PostTypes: []string{"article", "missing"}})
			},
			want: []string{"article"},
		},
		{
			name: "commentator manages no types",
			role: "Commentator",
			want: nil,
		},
		{
			name: "unknown role manages nothing",
			role: "Ghost",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := role.NewService()
			if tt.seed != nil {
				tt.seed(s)
			}
			got := s.ManageablePostTypes(tt.role, allSlugs)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		wantErr bool
	}{
		{name: "success - valid", in: "Journalist", wantErr: false},
		{name: "error - empty", in: "", wantErr: true},
		{name: "error - whitespace", in: "   ", wantErr: true},
		{name: "error - too long", in: string(make([]byte, 201)), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := role.ValidateName(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, role.ErrInvalidRoleName))
				return
			}
			require.NoError(t, err)
		})
	}
}