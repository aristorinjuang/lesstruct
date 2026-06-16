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
	logger          *util.Logger
}

// GetPostTypes returns all registered post types
func (h *PostTypeHandler) GetPostTypes(w http.ResponseWriter, r *http.Request) {
	postTypes := h.postTypeService.GetAll()

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

// GetPublicPostTypes returns all post types without requiring authentication
func (h *PostTypeHandler) GetPublicPostTypes(w http.ResponseWriter, r *http.Request) {
	postTypes := h.postTypeService.GetAll()

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
func NewPostTypeHandler(postTypeService PostTypeServiceInterface, logger *util.Logger) *PostTypeHandler {
	return &PostTypeHandler{
		postTypeService: postTypeService,
		logger:          logger,
	}
}
