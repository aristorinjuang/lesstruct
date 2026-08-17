package role

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	// ErrInvalidRoleName is returned when a role name fails validation
	ErrInvalidRoleName = errors.New("role name is required and must be between 1 and 200 characters")
	// ErrDuplicateRole is returned when registering a new role whose name already exists
	ErrDuplicateRole = errors.New("role with this name already exists")
	// ErrAdminRoleReserved is returned when a config entry tries to define or
	// override the Admin role — it is built-in and cannot be redefined.
	ErrAdminRoleReserved = errors.New("the Admin role is reserved and cannot be redefined")
	// ErrRoleNotFound is returned when a role name is not registered
	ErrRoleNotFound = errors.New("role not found")
)

// Role represents a user role and its content-management capabilities. The
// string stored in the users.role column is Role.Name. Custom roles are declared
// in config.toml under [[role]]; the built-in roles (Admin, Contributor,
// Commentator) are always present and can be overridden by a same-name entry
// (Admin cannot).
//
// PostTypes is the allowlist of post-type slugs the role may manage (own-content
// CRUD). An empty PostTypes on a NEW custom role means "manages no content
// types". AllTypes marks a role that manages every registered type — it is set
// for Admin and the default Contributor and is never read from config; a config
// entry that overrides Contributor without listing post_types keeps AllTypes.
type Role struct {
	Name       string   `json:"name" toml:"name"`
	PostTypes  []string `json:"postTypes,omitempty" toml:"post_types,omitempty"`
	AllTypes   bool     `json:"allTypes,omitempty" toml:"-"`
	Publish    bool     `json:"publish,omitempty" toml:"publish,omitempty"`
	Media      bool     `json:"media,omitempty" toml:"media,omitempty"`
	Comments   bool     `json:"comments,omitempty" toml:"comments,omitempty"`
	IsAdmin    bool     `json:"isAdmin,omitempty" toml:"-"`
}

// ValidateName validates the role name field
func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 200 {
		return ErrInvalidRoleName
	}
	return nil
}