package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/capability"
	"github.com/aristorinjuang/lesstruct/kernel/customfield"
	"github.com/aristorinjuang/lesstruct/kernel/httpx"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
)

func primaryRole(principal kernel.Principal) string {
	if len(principal.Roles) == 0 {
		return ""
	}
	return principal.Roles[0]
}

func writeLegacy(w http.ResponseWriter, data any, withMeta bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	envelope := legacyEnvelope{Data: data}
	if withMeta {
		envelope.Meta = map[string]string{
			"timestamp": time.Now().Format(time.RFC3339),
		}
	}
	_ = json.NewEncoder(w).Encode(envelope)
}

// legacyEnvelope reproduces the exact pre-module response bytes for the
// post-type endpoints: data plus an explicit error:null plus a timestamp
// meta. encoding/json cannot emit a null for an omitempty field, so this
// dedicated struct (no omitempty on Error) preserves the legacy shape.
// New endpoints should use httpx.Success instead; these keep the legacy
// shape so clients observe zero drift across the extraction.
type legacyEnvelope struct {
	Data  any               `json:"data"`
	Error any               `json:"error"`
	Meta  map[string]string `json:"meta,omitempty"`
}

// roleDTO is the JSON shape of a registered role for the roles endpoint.
type roleDTO struct {
	Name      string   `json:"name"`
	PostTypes []string `json:"postTypes,omitempty"`
	AllTypes  bool     `json:"allTypes"`
	Publish   bool     `json:"publish"`
	Media     bool     `json:"media"`
	Comments  bool     `json:"comments"`
	IsAdmin   bool     `json:"isAdmin"`
}

// PostTypeServiceInterface defines the interface for post type service
type PostTypeServiceInterface interface {
	GetAll() []posttype.PostType
	GetBySlug(slug string) (posttype.PostType, error)
	Register(pt posttype.PostType) error
	GetUserFields() []customfield.FieldSchema
	GetUserSystemFields() []customfield.FieldSchema
}

// Role is the assignable-role shape the roles endpoint serializes. It
// mirrors the users module's role record; core adapts *role.Service to the
// RoleCapabilities interface below until the users module lands.
type Role struct {
	Name      string
	PostTypes []string
	AllTypes  bool
	Publish   bool
	Media     bool
	Comments  bool
	IsAdmin   bool
}

// RoleCapabilities is the narrow role-registry seam this handler depends on.
// Core satisfies it with an adapter over *role.Service (which cannot be
// imported here); the users module implements it natively when it lands.
type RoleCapabilities interface {
	IsAdmin(name string) bool
	ManageablePostTypes(name string, allSlugs []string) []string
	CanPublish(name string) bool
	CanMedia(name string) bool
	CanComment(name string) bool
	GetAll() []Role
}

// Handler handles post type HTTP requests.
type Handler struct {
	postTypeService PostTypeServiceInterface
	commentsEnabled bool
	resolve         kernel.CapabilityResolver
	roles           RoleCapabilities
}

// listPostTypes returns the registered post types. Hidden is a presentation
// flag: the registry (and content validation, templates, importers) still
// resolves hidden types; only external consumers of the list skip them. The
// admin endpoint includes hidden types (with their hidden flag) so the admin
// panel can decide what to surface; the public endpoint excludes them.
// The "comment" sentinel is meaningless without the comment system, so it is
// dropped from the list when comments are disabled.
func (h *Handler) listPostTypes(includeHidden bool) []posttype.PostType {
	all := h.postTypeService.GetAll()
	result := make([]posttype.PostType, 0, len(all))
	for _, pt := range all {
		if !includeHidden && pt.Hidden {
			continue
		}
		if !h.commentsEnabled && pt.Slug == "comment" {
			continue
		}
		result = append(result, pt)
	}
	return result
}

// roleScopedPostTypes narrows postTypes to the ones the caller may manage.
// Without a resolver — or for the admin grant — the list is returned as-is.
// Otherwise each type is checked individually, which is equivalent to the
// legacy batch ManageablePostTypes lookup.
func (h *Handler) roleScopedPostTypes(principal kernel.Principal, postTypes []posttype.PostType) []posttype.PostType {
	if h.resolve == nil {
		return postTypes
	}
	if h.resolve(principal, string(capability.CapabilityAdmin)) {
		return postTypes
	}

	result := make([]posttype.PostType, 0, len(postTypes))
	for _, pt := range postTypes {
		if h.resolve(principal, capability.ManageContent(pt.Slug)) {
			result = append(result, pt)
		}
	}
	return result
}

// GetPostTypes returns all registered post types (including hidden ones, which
// carry their hidden flag so the admin UI can filter its surfaces). With a
// resolver attached, the list is scoped to the caller's manageable types: a
// config-restricted role (e.g. a journalist managing only "article") only sees
// the types it may actually use; admins always see everything.
func (h *Handler) GetPostTypes(w http.ResponseWriter, r *http.Request) {
	principal, _ := kernel.PrincipalFromContext(r.Context())
	postTypes := h.roleScopedPostTypes(principal, h.listPostTypes(true))

	writeLegacy(w, postTypes, true)
}

// GetRoles returns the registered roles (the assignable list the admin user
// management UI needs) plus the caller's own capabilities (what the content
// editor and navigation should surface). Requires roles; when the module was
// built without them it returns 404, mirroring the fail-closed stance of the
// config-driven role feature. Served here until the users module takes it over.
func (h *Handler) GetRoles(w http.ResponseWriter, r *http.Request) {
	if h.roles == nil {
		httpx.Error(w, http.StatusNotFound, "roles_unavailable", "Role registry is not configured", nil)
		return
	}

	principal, _ := kernel.PrincipalFromContext(r.Context())
	roleName := primaryRole(principal)

	all := h.postTypeService.GetAll()
	allSlugs := make([]string, 0, len(all))
	for _, pt := range all {
		allSlugs = append(allSlugs, pt.Slug)
	}

	roles := make([]roleDTO, 0, len(h.roles.GetAll()))
	for _, rl := range h.roles.GetAll() {
		roles = append(roles, roleDTO(rl))
	}

	writeLegacy(w, map[string]any{
		"roles": roles,
		"me": map[string]any{
			"role":      roleName,
			"postTypes": h.roles.ManageablePostTypes(roleName, allSlugs),
			"publish":   h.roles.CanPublish(roleName),
			"media":     h.roles.CanMedia(roleName),
			"comments":  h.roles.CanComment(roleName),
			"isAdmin":   h.roles.IsAdmin(roleName),
		},
	}, true)
}

// GetPublicPostTypes returns visible post types without requiring
// authentication. Hidden types are excluded — public consumers must never see
// them.
func (h *Handler) GetPublicPostTypes(w http.ResponseWriter, r *http.Request) {
	postTypes := h.listPostTypes(false)

	writeLegacy(w, postTypes, true)
}

// GetUserFieldsEndpoint returns user field schemas (custom and system). It is
// admin-gated via the capability resolver, matching the core AdminOnly chain
// it replaces (403 INSUFFICIENT_PERMISSIONS with the identical message).
// Served here until the users module takes it over.
func (h *Handler) GetUserFieldsEndpoint(w http.ResponseWriter, r *http.Request) {
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok || h.resolve == nil || !h.resolve(principal, string(capability.CapabilityAdmin)) {
		httpx.Error(w, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "You do not have permission to access this resource", nil)
		return
	}

	writeLegacy(w, map[string]any{
		"fields":       h.postTypeService.GetUserFields(),
		"systemFields": h.postTypeService.GetUserSystemFields(),
	}, false)
}

// New builds a Handler. A nil resolver disables role scoping (legacy
// allow-all); nil roles fail the roles endpoint closed with 404.
func New(
	postTypeService PostTypeServiceInterface,
	commentsEnabled bool,
	resolve kernel.CapabilityResolver,
	roles RoleCapabilities,
) *Handler {
	return &Handler{
		postTypeService: postTypeService,
		commentsEnabled: commentsEnabled,
		resolve:         resolve,
		roles:           roles,
	}
}
