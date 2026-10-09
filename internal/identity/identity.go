// Package identity bridges kernel.Principal to the v0 int-ID, single-role
// model during the Phase 2 transition. Services resolving acting identity
// parse through here so the "invalid user ID" failure stays in one place.
// Phase 3 modules own their ID shapes; new code must prefer Principal fields
// over these helpers.
package identity

import (
	"fmt"
	"strconv"

	"github.com/aristorinjuang/lesstruct/kernel"
)

// ErrInvalidUserID is returned when a principal carries a non-decimal ID.
// Handlers map it to the same 400 invalid_user_id response the pre-Phase-2
// handler-side parse produced. It aliases the kernel sentinel so module
// services (which cannot import this package) map through the same value.
var ErrInvalidUserID = kernel.ErrInvalidPrincipalID

// New builds a principal from v0 int-ID parts. Importers and tests use it to
// construct the acting identity they already hold as parts.
func New(
	userID int,
	username string,
	role string,
) kernel.Principal {
	roles := make([]string, 0, 1)
	if role != "" {
		roles = append(roles, role)
	}
	return kernel.Principal{
		ID:       strconv.Itoa(userID),
		Username: username,
		Roles:    roles,
	}
}

// IntID parses the principal's decimal ID for repository calls.
func IntID(principal kernel.Principal) (int, error) {
	id, err := strconv.Atoi(principal.ID)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidUserID, principal.ID)
	}
	return id, nil
}

// PrimaryRole returns the principal's first role, or "" when anonymous.
// v0 roles are single-valued; multi-role resolution belongs to the users
// module (Phase 3).
func PrimaryRole(principal kernel.Principal) string {
	if len(principal.Roles) == 0 {
		return ""
	}
	return principal.Roles[0]
}
