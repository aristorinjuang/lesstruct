package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
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
// carry their hidden flag so the admin UI can filter its surfaces).
func (h *PostTypeHandler) GetPostTypes(w http.ResponseWriter, r *http.Request) {
	postTypes := h.listPostTypes(true)

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
func NewPostTypeHandler(postTypeService PostTypeServiceInterface, commentsEnabled bool, logger *util.Logger) *PostTypeHandler {
	return &PostTypeHandler{
		postTypeService: postTypeService,
		commentsEnabled: commentsEnabled,
		logger:          logger,
	}
}
