package kernel

import (
	"context"
	"errors"
	"slices"
)

// Principal is the minimal identity every auth module injects into requests.
//
// The kernel owns this shape — never a concrete user type — so identity stays a
// module concern (users/auth) while route protection stays declarative.
type Principal struct {
	ID       string
	Username string
	Roles    []string
}

// HasRole reports whether the principal carries the given role.
func (p Principal) HasRole(role string) bool {
	return slices.Contains(p.Roles, role)
}

// IsAnonymous reports whether no identity module populated the principal.
func (p Principal) IsAnonymous() bool {
	return p.ID == "" && p.Username == ""
}

// ContextWithPrincipal stores the principal in the context.
func ContextWithPrincipal(
	ctx context.Context,
	principal Principal,
) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

// PrincipalFromContext reads the principal back. ok is false on anonymous
// requests, where no auth module ran.
func PrincipalFromContext(ctx context.Context) (principal Principal, ok bool) {
	principal, ok = ctx.Value(principalKey{}).(Principal)
	return principal, ok
}

// CapabilityResolver maps a principal plus a capability name (for example
// "content.publish") to an allow/deny decision.
//
// Implemented by the users module. The kernel never resolves roles itself.
type CapabilityResolver func(principal Principal, capability string) bool

// ErrInvalidPrincipalID marks a principal whose ID is not a decimal user ID.
var ErrInvalidPrincipalID = errors.New("kernel: invalid principal ID")
