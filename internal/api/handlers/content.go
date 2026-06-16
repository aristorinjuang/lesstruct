package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/seo"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

var numericPattern = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

var fieldSlugPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func handleContentError(w http.ResponseWriter, err error) {
	statusCode := http.StatusInternalServerError
	code := "internal_error"
	message := "An internal error occurred"

	switch {
	case errors.Is(err, contentdomain.ErrInvalidTitle):
		statusCode = http.StatusBadRequest
		code = "invalid_title"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrInvalidContent):
		statusCode = http.StatusBadRequest
		code = "invalid_content"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrInvalidStatus):
		statusCode = http.StatusBadRequest
		code = "invalid_status"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrInvalidSlug):
		statusCode = http.StatusBadRequest
		code = "invalid_slug"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrSlugAlreadyExists):
		statusCode = http.StatusConflict
		code = "slug_exists"
		message = "A content item with this slug already exists"
	case errors.Is(err, contentdomain.ErrUnauthorized):
		statusCode = http.StatusUnauthorized
		code = "unauthorized"
		message = "You do not have permission to modify this content"
	case errors.Is(err, contentdomain.ErrContentNotFound):
		statusCode = http.StatusNotFound
		code = "content_not_found"
		message = "Content not found"
	case errors.Is(err, contentdomain.ErrInvalidFilterField):
		statusCode = http.StatusBadRequest
		code = "invalid_filter_field"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrInvalidFilterOperator):
		statusCode = http.StatusBadRequest
		code = "invalid_filter_operator"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrInvalidFilterValue):
		statusCode = http.StatusBadRequest
		code = "invalid_filter_value"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrUnknownSystemFieldKey):
		statusCode = http.StatusBadRequest
		code = "unknown_system_field_key"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrSystemFieldValidation):
		statusCode = http.StatusBadRequest
		code = "system_field_validation"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrTranslationGroupNotFound):
		statusCode = http.StatusBadRequest
		code = "translation_group_not_found"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrTranslationAlreadyExists):
		statusCode = http.StatusConflict
		code = "translation_already_exists"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrInvalidLanguage):
		statusCode = http.StatusBadRequest
		code = "invalid_language"
		message = err.Error()
	}

	sendErrorResponse(w, statusCode, code, message, nil)
}

func sendSuccessResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":  data,
		"error": nil,
		"meta": map[string]string{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}

func sendErrorResponse(w http.ResponseWriter, statusCode int, code, message string, details any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
			"details": details,
		},
		"meta": map[string]string{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}

func parseCustomFieldFilters(r *http.Request) []contentdomain.CustomFieldFilter {
	var filters []contentdomain.CustomFieldFilter
	query := r.URL.Query()

	for key, values := range query {
		if len(filters) >= contentdomain.MaxCustomFieldFilters {
			break
		}

		if !strings.HasPrefix(key, "cf_") || len(values) == 0 || values[0] == "" {
			continue
		}

		fieldPart := strings.TrimPrefix(key, "cf_")
		value := values[0]

		if slug, ok := strings.CutSuffix(fieldPart, "_min"); ok {
			if !fieldSlugPattern.MatchString(slug) {
				continue
			}
			if !numericPattern.MatchString(value) {
				continue
			}
			filters = append(filters, contentdomain.CustomFieldFilter{
				Field:    slug,
				Operator: contentdomain.FilterOpMin,
				Value:    value,
			})
		} else if slug, ok := strings.CutSuffix(fieldPart, "_max"); ok {
			if !fieldSlugPattern.MatchString(slug) {
				continue
			}
			if !numericPattern.MatchString(value) {
				continue
			}
			filters = append(filters, contentdomain.CustomFieldFilter{
				Field:    slug,
				Operator: contentdomain.FilterOpMax,
				Value:    value,
			})
		} else {
			if !fieldSlugPattern.MatchString(fieldPart) {
				continue
			}
			filters = append(filters, contentdomain.CustomFieldFilter{
				Field:    fieldPart,
				Operator: contentdomain.FilterOpEqual,
				Value:    value,
			})
		}
	}

	return filters
}

type SearchResult struct {
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	MetaDescription string `json:"metaDescription"`
}

type CreateContentRequest struct {
	Title              string         `json:"title"`
	Content            string         `json:"content"`
	Tags               []string       `json:"tags"`
	Status             string         `json:"status"`
	PostType           string         `json:"postType"`
	MetaDescription    string         `json:"metaDescription,omitempty"`
	OGTitle            string         `json:"ogTitle,omitempty"`
	OGDescription      string         `json:"ogDescription,omitempty"`
	AllowComments      *bool          `json:"allowComments,omitempty"`
	CustomFields       map[string]any `json:"customFields,omitempty"`
	Language           string         `json:"language,omitempty"`
	TranslationGroupID *int           `json:"translationGroupId,omitempty"`
}

type CreateContentResponse struct {
	Data *CreateContentData `json:"data"`
	Meta *ResponseMeta      `json:"meta"`
}

type CreateContentData struct {
	Content *contentdomain.Content `json:"content"`
}

type GenerateSlugRequest struct {
	Title string `json:"title"`
}

type GenerateSlugResponse struct {
	Data *GenerateSlugData `json:"data"`
	Meta *ResponseMeta     `json:"meta"`
}

type GenerateSlugData struct {
	Slug string `json:"slug"`
}

type ContentServiceInterface interface {
	Create(ctx context.Context, userID int, req contentdomain.CreateContentRequest) (*contentdomain.Content, error)
	GetByUser(ctx context.Context, userID int, limit int, offset int) ([]*contentdomain.Content, error)
	GetAll(ctx context.Context, limit int, offset int) ([]*contentdomain.Content, error)
	GetByID(ctx context.Context, id int) (*contentdomain.Content, error)
	GenerateSlugFromTitle(ctx context.Context, title string) (string, error)
	Update(ctx context.Context, id int, userID int, role string, req contentdomain.UpdateContentRequest) (*contentdomain.Content, error)
	DeleteContent(ctx context.Context, id int, userID int, role string) error
	GetPublished(ctx context.Context, limit int, offset int) ([]*contentdomain.Content, error)
	GetPublishedBySlug(ctx context.Context, slug string, language string) (*contentdomain.Content, error)
	GetTranslations(ctx context.Context, translationGroupID int, excludeID int) ([]*contentdomain.Content, error)
	GetPublishedByAuthorUsername(ctx context.Context, username string, limit int, offset int) ([]*contentdomain.Content, error)
	AuthorExists(ctx context.Context, username string) (bool, error)
	ListByFilters(ctx context.Context, userID int, filters contentdomain.ContentFilters) ([]*contentdomain.Content, error)
	SetSystemFields(ctx context.Context, contentID int, systemFields map[string]any) (*contentdomain.Content, error)
	SearchPublished(ctx context.Context, query string, limit int) ([]*contentdomain.Content, error)
}

type ContentHandler struct {
	contentService ContentServiceInterface
	logger         *util.Logger
	baseURL        string
}

func (h *ContentHandler) CreateContent(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID", nil)
		return
	}

	var req CreateContentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request body: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	contentReq := contentdomain.CreateContentRequest{
		Title:              req.Title,
		Content:            req.Content,
		Tags:               req.Tags,
		Status:             contentdomain.Status(req.Status),
		PostType:           req.PostType,
		MetaDescription:    req.MetaDescription,
		OGTitle:            req.OGTitle,
		OGDescription:      req.OGDescription,
		AllowComments:      req.AllowComments,
		CustomFields:       req.CustomFields,
		Language:           req.Language,
		TranslationGroupID: req.TranslationGroupID,
	}

	content, err := h.contentService.Create(r.Context(), userID, contentReq)
	if err != nil {
		h.logger.Error("Failed to create content: %v", err)
		handleContentError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusCreated, &CreateContentData{Content: content})
}

func (h *ContentHandler) GenerateSlug(w http.ResponseWriter, r *http.Request) {
	var req GenerateSlugRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request body: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	slug, err := h.contentService.GenerateSlugFromTitle(r.Context(), req.Title)
	if err != nil {
		h.logger.Error("Failed to generate slug: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_title", err.Error(), nil)
		return
	}

	sendSuccessResponse(w, http.StatusOK, &GenerateSlugData{Slug: slug})
}

func (h *ContentHandler) ListContents(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID", nil)
		return
	}

	role, _ := middleware.GetRole(r)
	isAdmin := role == contentdomain.RoleAdmin

	limit := 100
	limitQuery := r.URL.Query().Get("limit")
	if limitQuery != "" {
		if l, err := strconv.Atoi(limitQuery); err != nil {
			limit = 100
		} else if l <= 0 {
			limit = 100
		} else if l > 1000 {
			limit = 1000
		} else {
			limit = l
		}
	}

	offset := 0
	offsetQuery := r.URL.Query().Get("offset")
	if offsetQuery != "" {
		if o, err := strconv.Atoi(offsetQuery); err != nil {
			offset = 0
		} else if o < 0 {
			offset = 0
		} else {
			offset = o
		}
	}

	postType := r.URL.Query().Get("post_type")
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if len(search) < 2 {
		search = ""
	}
	customFieldFilters := parseCustomFieldFilters(r)
	language := r.URL.Query().Get("language")

	if postType != "" || search != "" || len(customFieldFilters) > 0 || language != "" {
		filters := contentdomain.ContentFilters{
			Limit:               limit,
			Offset:              offset,
			PostType:            postType,
			Search:              search,
			Language:            language,
			CustomFieldFilters:  customFieldFilters,
		}
		filterUserID := userID
		if isAdmin {
			filterUserID = 0
		}
		contents, err := h.contentService.ListByFilters(r.Context(), filterUserID, filters)
		if err != nil {
			h.logger.Error("Failed to list contents by filters: %v", err)
			handleContentError(w, err)
			return
		}
		sendSuccessResponse(w, http.StatusOK, contents)
		return
	}

	if isAdmin {
		contents, err := h.contentService.GetAll(r.Context(), limit, offset)
		if err != nil {
			h.logger.Error("Failed to list all contents: %v", err)
			handleContentError(w, err)
			return
		}
		sendSuccessResponse(w, http.StatusOK, contents)
		return
	}

	contents, err := h.contentService.GetByUser(r.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("Failed to list contents: %v", err)
		handleContentError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusOK, contents)
}

func (h *ContentHandler) UpdateContent(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID", nil)
		return
	}

	pathValue := r.PathValue("id")
	if pathValue == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content_id", "Content ID is required", nil)
		return
	}

	contentID, err := strconv.Atoi(pathValue)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content_id", "Invalid content ID", nil)
		return
	}

	var req CreateContentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request body: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	role, _ := middleware.GetRole(r)

	updateReq := contentdomain.UpdateContentRequest{
		Title:              req.Title,
		Content:            req.Content,
		Tags:               req.Tags,
		Status:             contentdomain.Status(req.Status),
		PostType:           req.PostType,
		MetaDescription:    req.MetaDescription,
		OGTitle:            req.OGTitle,
		OGDescription:      req.OGDescription,
		AllowComments:      req.AllowComments,
		CustomFields:       req.CustomFields,
		Language:           req.Language,
		TranslationGroupID: req.TranslationGroupID,
	}

	content, err := h.contentService.Update(r.Context(), contentID, userID, role, updateReq)
	if err != nil {
		h.logger.Error("Failed to update content: %v", err)
		if errors.Is(err, contentdomain.ErrUnauthorized) {
			sendErrorResponse(w, http.StatusForbidden, "forbidden", "You do not have permission to modify this content", nil)
			return
		}
		handleContentError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusOK, &CreateContentData{Content: content})
}

func (h *ContentHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID", nil)
		return
	}

	pathValue := r.PathValue("id")
	if pathValue == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content_id", "Content ID is required", nil)
		return
	}

	contentID, err := strconv.Atoi(pathValue)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content_id", "Invalid content ID", nil)
		return
	}

	content, err := h.contentService.GetByID(r.Context(), contentID)
	if err != nil {
		h.logger.Error("Failed to get content: %v", err)
		handleContentError(w, err)
		return
	}

	role, _ := middleware.GetRole(r)
	if content.UserID != userID && role != contentdomain.RoleAdmin {
		sendErrorResponse(w, http.StatusForbidden, "forbidden", "You do not have permission to access this content", nil)
		return
	}

	// Fetch translations: primary content uses its own ID as group ID;
	// translations use their translation_group_id.
	var translations []*contentdomain.Content
	groupID := content.ID
	if content.TranslationGroupID != nil {
		groupID = *content.TranslationGroupID
	}
	trans, transErr := h.contentService.GetTranslations(r.Context(), groupID, content.ID)
	if transErr != nil {
		h.logger.Error("Failed to get translations: %v", transErr)
	} else {
		translations = trans
	}

	response := map[string]any{
		"id":                 content.ID,
		"userId":             content.UserID,
		"title":              content.Title,
		"slug":               content.Slug,
		"content":            content.Content,
		"tags":               content.Tags,
		"status":             content.Status,
		"postType":           content.PostType,
		"metaDescription":    content.MetaDescription,
		"ogTitle":            content.OGTitle,
		"ogDescription":      content.OGDescription,
		"author":             content.Author,
		"username":           content.Username,
		"allowComments":      content.AllowComments,
		"customFields":       content.CustomFields,
		"language":           content.Language,
		"translationGroupId": content.TranslationGroupID,
		"updatedBy":          content.UpdatedBy,
		"updatedByUsername":  content.UpdatedByUsername,
		"createdAt":          content.CreatedAt,
		"updatedAt":          content.UpdatedAt,
	}

	if len(translations) > 0 {
		response["translations"] = translations
	}

	sendSuccessResponse(w, http.StatusOK, response)
}

func (h *ContentHandler) GetSEO(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID", nil)
		return
	}

	pathValue := r.PathValue("id")
	if pathValue == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content_id", "Content ID is required", nil)
		return
	}

	contentID, err := strconv.Atoi(pathValue)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content_id", "Invalid content ID", nil)
		return
	}

	content, err := h.contentService.GetByID(r.Context(), contentID)
	if err != nil {
		h.logger.Error("Failed to get content: %v", err)
		handleContentError(w, err)
		return
	}

	seoRole, _ := middleware.GetRole(r)
	if content.UserID != userID && seoRole != contentdomain.RoleAdmin {
		sendErrorResponse(w, http.StatusForbidden, "forbidden", "You do not have permission to access this content", nil)
		return
	}

	// Extract image URL from TipTap content for OG/Twitter cards
	ogImage := seo.ExtractImageURL(content.Content)
	ogURL := "/posts/" + content.Slug

	seoData := map[string]any{
		"metaDescription":    content.MetaDescription,
		"ogTitle":            content.OGTitle,
		"ogDescription":      content.OGDescription,
		"ogImage":            ogImage,
		"ogUrl":              ogURL,
		"ogType":             "article",
		"ogSiteName":         "Lesstruct",
		"twitterCard":        "summary_large_image",
		"twitterTitle":       content.OGTitle,
		"twitterDescription": content.OGDescription,
		"twitterImage":       ogImage,
		"jsonLd": map[string]any{
			"@context":      "https://schema.org",
			"@type":         "Article",
			"headline":      content.Title,
			"description":   content.MetaDescription,
			"datePublished": content.CreatedAt,
			"dateModified":  content.UpdatedAt,
			"author": map[string]any{
				"@type": "Person",
				"name":  content.Author,
			},
		},
	}

	if content.Author == "" {
		delete(seoData["jsonLd"].(map[string]any), "author")
	}

	if ogImage != "" {
		seoData["jsonLd"].(map[string]any)["image"] = ogImage
	}

	sendSuccessResponse(w, http.StatusOK, map[string]any{
		"seo": seoData,
	})
}

func (h *ContentHandler) ListPublishedContents(w http.ResponseWriter, r *http.Request) {
	limit := 100
	limitQuery := r.URL.Query().Get("limit")
	if limitQuery != "" {
		if l, err := strconv.Atoi(limitQuery); err != nil {
			limit = 100
		} else if l <= 0 {
			limit = 100
		} else if l > 1000 {
			limit = 1000
		} else {
			limit = l
		}
	}

	offset := 0
	offsetQuery := r.URL.Query().Get("offset")
	if offsetQuery != "" {
		if o, err := strconv.Atoi(offsetQuery); err != nil {
			offset = 0
		} else if o < 0 {
			offset = 0
		} else {
			offset = o
		}
	}

	contents, err := h.contentService.GetPublished(r.Context(), limit, offset)
	if err != nil {
		h.logger.Error("Failed to list published content: %v", err)
		handleContentError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusOK, contents)
}

func (h *ContentHandler) GetPublishedContent(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_slug", "Slug is required", nil)
		return
	}

	content, err := h.contentService.GetPublishedBySlug(r.Context(), slug, "en")
	if err != nil {
		handleContentError(w, err)
		return
	}

	response := map[string]any{
		"id":              content.ID,
		"userId":          content.UserID,
		"title":           content.Title,
		"slug":            content.Slug,
		"content":         content.Content,
		"tags":            content.Tags,
		"status":          content.Status,
		"postType":        content.PostType,
		"metaDescription": content.MetaDescription,
		"ogTitle":         content.OGTitle,
		"ogDescription":   content.OGDescription,
		"author":          content.Author,
		"username":        content.Username,
		"allowComments":   content.AllowComments,
		"createdAt":       content.CreatedAt,
		"updatedAt":       content.UpdatedAt,
	}

	if content.CustomFields != nil {
		response["customFields"] = content.CustomFields
	}

	ogImage := seo.ExtractImageURL(content.Content)
	if ogImage != "" {
		response["ogImage"] = seo.BuildURL(h.baseURL, ogImage)
	}

	sendSuccessResponse(w, http.StatusOK, response)
}

func (h *ContentHandler) GetPublishedContentByAuthor(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_username", "Username is required", nil)
		return
	}

	// Check if author exists before querying content
	exists, err := h.contentService.AuthorExists(r.Context(), username)
	if err != nil {
		h.logger.Error("Failed to check author existence: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to check author", nil)
		return
	}
	if !exists {
		sendErrorResponse(w, http.StatusNotFound, "author_not_found", "Author not found", nil)
		return
	}

	limit := 100
	limitQuery := r.URL.Query().Get("limit")
	if limitQuery != "" {
		if l, err := strconv.Atoi(limitQuery); err != nil {
			limit = 100
		} else if l <= 0 {
			limit = 100
		} else if l > 100 {
			limit = 100
		} else {
			limit = l
		}
	}

	offset := 0
	offsetQuery := r.URL.Query().Get("offset")
	if offsetQuery != "" {
		if o, err := strconv.Atoi(offsetQuery); err != nil {
			offset = 0
		} else if o < 0 {
			offset = 0
		} else {
			offset = o
		}
	}

	contents, err := h.contentService.GetPublishedByAuthorUsername(r.Context(), username, limit, offset)
	if err != nil {
		h.logger.Error("Failed to list published content by author: %v", err)
		handleContentError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusOK, contents)
}

func (h *ContentHandler) DeleteContent(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID", nil)
		return
	}

	pathValue := r.PathValue("id")
	if pathValue == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content_id", "Content ID is required", nil)
		return
	}

	contentID, err := strconv.Atoi(pathValue)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content_id", "Invalid content ID", nil)
		return
	}

	deleteRole, _ := middleware.GetRole(r)

	if err := h.contentService.DeleteContent(r.Context(), contentID, userID, deleteRole); err != nil {
		h.logger.Error("Failed to delete content: %v", err)
		if errors.Is(err, contentdomain.ErrUnauthorized) {
			sendErrorResponse(w, http.StatusForbidden, "forbidden", "You do not have permission to delete this content", nil)
			return
		}
		handleContentError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusOK, map[string]string{
		"message": "Content deleted successfully",
	})
}

func (h *ContentHandler) SetSystemFields(w http.ResponseWriter, r *http.Request) {
	pathValue := r.PathValue("id")
	if pathValue == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content_id", "Content ID is required", nil)
		return
	}

	contentID, err := strconv.Atoi(pathValue)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content_id", "Invalid content ID", nil)
		return
	}

	var req struct {
		SystemFields map[string]any `json:"systemFields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request body: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	content, err := h.contentService.SetSystemFields(r.Context(), contentID, req.SystemFields)
	if err != nil {
		h.logger.Error("Failed to set system fields: %v", err)
		handleContentError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusOK, &CreateContentData{Content: content})
}

func (h *ContentHandler) SearchPublished(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		sendSuccessResponse(w, http.StatusOK, []SearchResult{})
		return
	}

	limit := 10
	limitQuery := r.URL.Query().Get("limit")
	if limitQuery != "" {
		if l, err := strconv.Atoi(limitQuery); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	contents, err := h.contentService.SearchPublished(r.Context(), query, limit)
	if err != nil {
		h.logger.Error("Failed to search published content: %v", err)
		handleContentError(w, err)
		return
	}

	results := make([]SearchResult, len(contents))
	for i, c := range contents {
		results[i] = SearchResult{
			Slug:            c.Slug,
			Title:           c.Title,
			MetaDescription: c.MetaDescription,
		}
	}

	sendSuccessResponse(w, http.StatusOK, results)
}

func NewContentHandler(contentService ContentServiceInterface, logger *util.Logger, baseURL string) *ContentHandler {
	return &ContentHandler{
		contentService: contentService,
		logger:         logger,
		baseURL:        strings.TrimRight(baseURL, "/"),
	}
}
