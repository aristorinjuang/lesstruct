package middleware

import (
	"net/http"

	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/capability"
)

// Capability names a permission the kernel resolver decides. It aliases the
// kernel vocabulary (kernel/capability) so existing references keep working;
// new code should import kernel/capability directly.
type Capability = capability.Capability

const (
	CapabilityAdmin          = capability.CapabilityAdmin
	CapabilityMedia          = capability.CapabilityMedia
	CapabilityComments       = capability.CapabilityComments
	CapabilityPublishContent = capability.CapabilityPublishContent
	CapabilityManageContent  = capability.CapabilityManageContent
)

// ManageContentCapability builds the per-type manage capability.
func ManageContentCapability(postType string) string {
	return capability.ManageContent(postType)
}

// RoleCapabilities is the narrow interface the resolver depends on for role
// lookups. *role.Service satisfies it. Defined here — not in the domain — so
// the middleware owns its own seam and the domain stays HTTP-agnostic
// (mirrors the APIKeyVerifier pattern).
type RoleCapabilities interface {
	IsAdmin(name string) bool
	CanPublish(name string) bool
	CanMedia(name string) bool
	CanComment(name string) bool
	CanManageType(name, postTypeSlug string) bool
}

// NewCapabilityResolver builds the kernel resolver over the role registry. A
// principal is granted a capability when ANY of its roles grants it. Empty
// principals, unknown roles, and unknown capabilities all deny (fail closed).
// A nil registry yields a deny-all resolver.
func NewCapabilityResolver(roles RoleCapabilities) kernel.CapabilityResolver {
	if roles == nil {
		return func(kernel.Principal, string) bool {
			return false
		}
	}
	return func(principal kernel.Principal, capabilityName string) bool {
		name, arg := capability.Split(capabilityName)
		for _, role := range principal.Roles {
			if role == "" {
				continue
			}
			switch name {
			case capability.CapabilityAdmin:
				if roles.IsAdmin(role) {
					return true
				}
			case capability.CapabilityMedia:
				if roles.CanMedia(role) {
					return true
				}
			case capability.CapabilityComments:
				if roles.CanComment(role) {
					return true
				}
			case capability.CapabilityPublishContent:
				if roles.CanPublish(role) {
					return true
				}
			case capability.CapabilityManageContent:
				if arg != "" && roles.CanManageType(role, arg) {
					return true
				}
			}
		}
		return false
	}
}

// RequireCapability enforces a capability on an already-authenticated request.
// Mount it after auth middleware: a missing principal, a nil resolver, or a
// denial all answer 403 INSUFFICIENT_PERMISSIONS, matching the RequireRole
// message so protected surfaces keep one denial shape.
func RequireCapability(
	resolve kernel.CapabilityResolver,
	capability string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := Principal(r)
			if !ok || resolve == nil || !resolve(principal, capability) {
				response.Error(w, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "You do not have permission to access this resource", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
