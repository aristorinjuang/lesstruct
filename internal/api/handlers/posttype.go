package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
	roledomain "github.com/aristorinjuang/lesstruct/internal/domain/role"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

// PostTypeServiceInterface defines the interface for post type service
type PostTypeServiceInterface interface {
	GetAll() []posttype.PostType
	GetBySlug(slug string) (posttype.PostType, error)
	Register(pt posttype.PostType) error
	GetUserFields() []customfield.FieldSchema
	GetUserSystemFields() []customfield.FieldSchema
}

// PostTypeHandler handles post type HTTP requests
type PostTypeHandler struct {
	postTypeService PostTypeServiceInterface
	commentsEnabled bool
	logger          *util.Logger
	roleService     *roledomain.Service
}

// PostTypeOption configures a PostTypeHandler. A role service enables role-based
// post type scoping on the admin list and the roles capability endpoint; without
// one the handler behaves as before (every authenticated user sees all types).
type PostTypeOption func(*PostTypeHandler)

// WithPostTypeRoleService attaches the config-driven role registry so the admin post
// type list is scoped to the caller's manageable types and the roles endpoint
// can report assignable roles and caller capabilities.
func WithPostTypeRoleService(rs *roledomain.Service) PostTypeOption {
	return func(h *PostTypeHandler) {
		h.roleService = rs
	}
}

// listPostTypes returns the registered post types. Hidden is a presentation
// flag: the registry (and content validation, templates, importers) still
// resolves hidden types; only external consumers of the list skip them. The
// admin endpoint includes hidden types (with their hidden flag) so the admin
// panel can decide what to surface; the public endpoint excludes them.
// The "comment" sentinel is meaningless without the comment system, so it is
// dropped from the list when comments are disabled.
func (h *PostTypeHandler) listPostTypes(includeHidden bool) []posttype.PostType {
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

// GetPostTypes returns all registered post types (including hidden ones, which
// carry their hidden flag so the admin UI can filter its surfaces). With a role
// service attached, the list is scoped to the caller's manageable types: a
// config-restricted role (e.g. a journalist managing only "article") only sees
// the types it may actually use; admins always see everything.
func (h *PostTypeHandler) GetPostTypes(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.GetRole(r)
	postTypes := h.roleScopedPostTypes(role, h.listPostTypes(true))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":  postTypes,
		"error": nil,
		"meta": map[string]string{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}

// roleScopedPostTypes narrows postTypes to the ones the given role may manage.
// Without a role service — or for the Admin role — the list is returned as-is.
func (h *PostTypeHandler) roleScopedPostTypes(role string, postTypes []posttype.PostType) []posttype.PostType {
	if h.roleService == nil || h.roleService.IsAdmin(role) {
		return postTypes
	}

	all := h.postTypeService.GetAll()
	allSlugs := make([]string, 0, len(all))
	for _, pt := range all {
		allSlugs = append(allSlugs, pt.Slug)
	}
	manageable := h.roleService.ManageablePostTypes(role, allSlugs)
	allowed := make(map[string]bool, len(manageable))
	for _, slug := range manageable {
		allowed[slug] = true
	}

	result := make([]posttype.PostType, 0, len(postTypes))
	for _, pt := range postTypes {
		if allowed[pt.Slug] {
			result = append(result, pt)
		}
	}
	return result
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

// GetRoles returns the registered roles (the assignable list the admin user
// management UI needs) plus the caller's own capabilities (what the content
// editor and navigation should surface). Requires the role service; when the
// handler was built without one it returns 404, mirroring the fail-closed
// stance of the config-driven role feature.
func (h *PostTypeHandler) GetRoles(w http.ResponseWriter, r *http.Request) {
	if h.roleService == nil {
		sendErrorResponse(w, http.StatusNotFound, "roles_unavailable", "Role registry is not configured", nil)
		return
	}

	roleName, _ := middleware.GetRole(r)

	all := h.postTypeService.GetAll()
	allSlugs := make([]string, 0, len(all))
	for _, pt := range all {
		allSlugs = append(allSlugs, pt.Slug)
	}

	roles := make([]roleDTO, 0, len(h.roleService.GetAll()))
	for _, rl := range h.roleService.GetAll() {
		roles = append(roles, roleDTO{
			Name:      rl.Name,
			PostTypes: rl.PostTypes,
			AllTypes:  rl.AllTypes,
			Publish:   rl.Publish,
			Media:     rl.Media,
			Comments:  rl.Comments,
			IsAdmin:   rl.IsAdmin,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"roles": roles,
			"me": map[string]any{
				"role":      roleName,
				"postTypes": h.roleService.ManageablePostTypes(roleName, allSlugs),
				"publish":   h.roleService.CanPublish(roleName),
				"media":     h.roleService.CanMedia(roleName),
				"comments":  h.roleService.CanComment(roleName),
				"isAdmin":   h.roleService.IsAdmin(roleName),
			},
		},
		"error": nil,
		"meta": map[string]string{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}

// GetPublicPostTypes returns visible post types without requiring
// authentication. Hidden types are excluded — public consumers must never see
// them.
func (h *PostTypeHandler) GetPublicPostTypes(w http.ResponseWriter, r *http.Request) {
	postTypes := h.listPostTypes(false)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":  postTypes,
		"error": nil,
		"meta": map[string]string{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}

// GetUserFieldsEndpoint returns user field schemas (custom and system)
func (h *PostTypeHandler) GetUserFieldsEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"fields":       h.postTypeService.GetUserFields(),
			"systemFields": h.postTypeService.GetUserSystemFields(),
		},
		"error": nil,
	})
}

// NewPostTypeHandler creates a new post type handler
func NewPostTypeHandler(postTypeService PostTypeServiceInterface, commentsEnabled bool, logger *util.Logger, opts ...PostTypeOption) *PostTypeHandler {
	h := &PostTypeHandler{
		postTypeService: postTypeService,
		commentsEnabled: commentsEnabled,
		logger:          logger,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}
