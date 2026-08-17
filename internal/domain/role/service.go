package role

import (
	"slices"
	"sync"
)

// defaultRoles returns the built-in roles. Admin is the reserved superuser
// (IsAdmin short-circuits every capability check to true). Contributor manages
// every registered post type and may publish; Commentator manages no content
// types but keeps the media and comment capabilities that any authenticated
// user has today. Capabilities can be narrowed or widened per site by declaring
// a same-name [[role]] entry in config.toml (except Admin).
func defaultRoles() []Role {
	return []Role{
		{
			Name:     "Admin",
			AllTypes: true,
			Publish:  true,
			Media:    true,
			Comments: true,
			IsAdmin:  true,
		},
		{
			Name:     "Contributor",
			AllTypes: true,
			Publish:  true,
			Media:    true,
			Comments: true,
		},
		{
			Name:     "Commentator",
			Publish:  false,
			Media:    true,
			Comments: true,
		},
	}
}

// Service is an in-memory registry of user roles. It is seeded with the built-in
// roles at construction and extended by [[role]] config entries at startup (same
// lifecycle as the post type registry).
type Service struct {
	registry map[string]Role
	defaults map[string]bool
	mu       sync.RWMutex
}

// Register adds or overrides a role. A name matching a built-in role (except
// Admin, which is reserved) overrides that built-in's capabilities; otherwise
// the entry is validated as a new role. Returns ErrDuplicateRole for a name that
// is neither a built-in nor already registered twice, and ErrAdminRoleReserved
// when a config entry names "Admin".
func (s *Service) Register(r Role) error {
	if err := ValidateName(r.Name); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if r.Name == "Admin" {
		return ErrAdminRoleReserved
	}

	if s.defaults[r.Name] {
		return s.overrideBuiltInLocked(r)
	}

	if _, exists := s.registry[r.Name]; exists {
		return ErrDuplicateRole
	}

	// A new custom role with no post_types manages no content types.
	s.registry[r.Name] = r
	return nil
}

// overrideBuiltInLocked applies a config entry over a built-in role in place,
// preserving the built-in's identity and, for Contributor, its manage-all-types
// default unless the entry lists explicit post_types. The write lock must be
// held by the caller and the name must be a registered built-in.
func (s *Service) overrideBuiltInLocked(r Role) error {
	base := s.registry[r.Name]
	base.Publish = r.Publish
	base.Media = r.Media
	base.Comments = r.Comments
	if len(r.PostTypes) > 0 {
		base.PostTypes = r.PostTypes
		base.AllTypes = false
	}
	s.registry[r.Name] = base
	return nil
}

// registerUnsafe stores a role without validation. Used only during NewService
// bootstrap (single-threaded) and takes the write lock itself.
func (s *Service) registerUnsafe(r Role) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registry[r.Name] = r
}

// get returns a copy of the role with the given name.
func (s *Service) get(name string) (Role, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.registry[name]
	return r, ok
}

// Get returns the role with the given name, or ErrRoleNotFound.
func (s *Service) Get(name string) (Role, error) {
	r, ok := s.get(name)
	if !ok {
		return Role{}, ErrRoleNotFound
	}
	return r, nil
}

// GetAll returns all registered roles. The order is non-deterministic.
func (s *Service) GetAll() []Role {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Role, 0, len(s.registry))
	for _, r := range s.registry {
		result = append(result, r)
	}
	return result
}

// IsAssignable reports whether the name is a registered role an admin may assign.
func (s *Service) IsAssignable(name string) bool {
	_, ok := s.get(name)
	return ok
}

// IsAdmin reports whether the role is the reserved Admin superuser.
func (s *Service) IsAdmin(name string) bool {
	r, ok := s.get(name)
	return ok && r.IsAdmin
}

// CanManageType reports whether the role may create/edit/delete content of the
// given post type. Admin and all-types roles manage everything.
func (s *Service) CanManageType(name, postTypeSlug string) bool {
	r, ok := s.get(name)
	if !ok {
		return false
	}
	if r.IsAdmin || r.AllTypes {
		return true
	}
	return slices.Contains(r.PostTypes, postTypeSlug)
}

// ManageablePostTypes returns the post-type slugs the role may manage, resolved
// against every registered slug. allSlugs must list the currently registered
// post types (including hidden ones). Admin and all-types roles manage all of
// them; other roles only the slugs in their allowlist.
func (s *Service) ManageablePostTypes(name string, allSlugs []string) []string {
	r, ok := s.get(name)
	if !ok {
		return nil
	}
	if r.IsAdmin || r.AllTypes {
		result := make([]string, len(allSlugs))
		copy(result, allSlugs)
		return result
	}
	result := make([]string, 0, len(r.PostTypes))
	for _, slug := range r.PostTypes {
		if slices.Contains(allSlugs, slug) {
			result = append(result, slug)
		}
	}
	return result
}

// CanPublish reports whether the role may publish content directly. Admin
// always can.
func (s *Service) CanPublish(name string) bool {
	r, ok := s.get(name)
	return ok && (r.IsAdmin || r.Publish)
}

// CanMedia reports whether the role may upload/generate media. Admin always can.
func (s *Service) CanMedia(name string) bool {
	r, ok := s.get(name)
	return ok && (r.IsAdmin || r.Media)
}

// CanComment reports whether the role may post comments while authenticated.
// Admin always can.
func (s *Service) CanComment(name string) bool {
	r, ok := s.get(name)
	return ok && (r.IsAdmin || r.Comments)
}

// NewService creates a role registry seeded with the built-in roles.
func NewService() *Service {
	s := &Service{
		registry: make(map[string]Role),
		defaults: make(map[string]bool),
	}
	for _, r := range defaultRoles() {
		s.registerUnsafe(r)
		s.defaults[r.Name] = true
	}
	return s
}