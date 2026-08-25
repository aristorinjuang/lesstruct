package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/aristorinjuang/lesstruct/internal/content/markdown"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/sanitize"
	"github.com/aristorinjuang/lesstruct/internal/seo"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

const (
	formatTiptap   = "tiptap"
	formatMarkdown = "markdown"
	formatHTML     = "html"

	publicFieldOperationFilter = "filter"
	publicFieldOperationSort   = "sort"
	publicFieldOperationExpose = "expose"
)

var (
	fieldSlugPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	numericPattern   = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

	// errInvalidSortByPrefix is returned when sort_by is non-empty but lacks the
	// required "cf:" prefix. Future extensions (e.g. sort_by=created_at) will
	// relax this — for now only custom-field sort is supported publicly.
	errInvalidSortByPrefix = errors.New("sort_by must be of the form 'cf:<field>'")

	errInvalidSortByField = errors.New("sort_by field must match ^[a-z][a-z0-9_]*$")

	errInvalidSortOrder = errors.New("order must be 'asc' or 'desc'")

	// errFieldNotQueryable is returned when a public cf_*/sort_by=cf:* parameter
	// references a field that is not in the [[public_field]] allowlist. The
	// handler maps this to a 400 field_not_queryable response.
	errFieldNotQueryable = errors.New("field is not publicly queryable; add a [[public_field]] entry to config.toml")
)

func handleContentError(w http.ResponseWriter, err error) {
	statusCode := http.StatusInternalServerError
	code := "internal_error"
	message := "An internal error occurred"

	switch {
	case errors.Is(err, contentdomain.ErrInvalidTitle):
		statusCode = http.StatusBadRequest
		code = "invalid_title"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrEmptyContent),
		errors.Is(err, contentdomain.ErrContentTooLong):
		statusCode = http.StatusBadRequest
		code = "invalid_content"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrInvalidFormat):
		statusCode = http.StatusBadRequest
		code = "invalid_format"
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
	case errors.Is(err, contentdomain.ErrForbiddenPostType):
		statusCode = http.StatusForbidden
		code = "forbidden_post_type"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrForbiddenPublish):
		statusCode = http.StatusForbidden
		code = "forbidden_publish"
		message = err.Error()
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

func normalizeBrowserFormat(format string) (contentdomain.Format, string) {
	normalized := strings.ToLower(strings.TrimSpace(format))
	if normalized == "" {
		return contentdomain.FormatTiptap, ""
	}
	switch normalized {
	case formatTiptap:
		return contentdomain.FormatTiptap, ""
	case formatMarkdown:
		return contentdomain.FormatMarkdown, ""
	case formatHTML:
		return contentdomain.FormatHTML, ""
	default:
		return "", "Unsupported format: " + normalized
	}
}

func convertBrowserContentBody(body string, format contentdomain.Format, iframeHosts []string) (string, string) {
	switch format {
	case contentdomain.FormatMarkdown:
		converted, err := markdown.Convert(body)
		if err != nil {
			return "", "Invalid markdown: " + err.Error()
		}
		return converted, ""
	case contentdomain.FormatHTML:
		return sanitize.SanitizeHTMLDocument(body, iframeHosts...), ""
	default:
		return body, ""
	}
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

// parsePublicSortBy reads the sort_by and order query params and returns the
// (field, order) pair on the public query surface. sort_by must be of the form
// "cf:<field>"; the "cf:" prefix is stripped before returning so the domain
// layer receives a bare field slug. An empty or non-cf sort_by returns empty
// strings — the caller treats that as "no sort requested" and falls through to
// the default ranking.
//
// order is validated against {"asc","desc",""}; any other value yields
// errInvalidSortOrder so the handler can map it to a 400 response.
func parsePublicSortBy(r *http.Request) (sortBy, sortOrder string, err error) {
	sortBy = strings.TrimSpace(r.URL.Query().Get("sort_by"))
	if sortBy == "" {
		return "", "", nil
	}
	if !strings.HasPrefix(sortBy, "cf:") {
		return "", "", errInvalidSortByPrefix
	}
	field := strings.TrimPrefix(sortBy, "cf:")
	if field == "" || !fieldSlugPattern.MatchString(field) {
		return "", "", errInvalidSortByField
	}

	order := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("order")))
	if order == "" {
		return field, "", nil
	}
	if order != string(contentdomain.SortOrderAsc) && order != string(contentdomain.SortOrderDesc) {
		return "", "", errInvalidSortOrder
	}
	return field, order, nil
}

// enforcePublicFieldAllowlist rejects the request when the registry does not
// allow every operation the caller is asking for. Returns nil when the
// registry is nil (fail-closed is enforced elsewhere by the caller not
// reaching this path) — but in practice, every call site first checks that
// the registry is non-nil and returns errFieldNotQueryable directly so the
// user gets a clear "add a [[public_field]] entry" message.
func enforcePublicFieldAllowlist(
	registry PublicFieldLookup,
	resource, postType string,
	customFieldFilters []contentdomain.CustomFieldFilter,
	sortBy, sortOrder string,
) error {
	for _, f := range customFieldFilters {
		operation := publicFieldOperationFilter
		if registry == nil || !registry.IsQueryable(resource, postType, f.Field, operation) {
			return errFieldNotQueryable
		}
	}
	if sortBy != "" {
		if registry == nil || !registry.IsQueryable(resource, postType, sortBy, publicFieldOperationSort) {
			return errFieldNotQueryable
		}
	}
	return nil
}

// projectPublicFields filters a raw custom-fields map to only the slugs that
// are allowlisted with the "expose" operation in the [[public_field]] config
// for the given resource. Returns nil (omitted from JSON via omitempty) when
// no fields are allowlisted or the registry is nil — safe default.
func projectPublicFields(
	registry PublicFieldLookup,
	resource, postType string,
	values map[string]any,
) map[string]any {
	if registry == nil || values == nil {
		return nil
	}

	exposed := registry.ExposedFields(resource, postType)
	if len(exposed) == 0 {
		return nil
	}

	result := make(map[string]any, len(exposed))
	for _, slug := range exposed {
		if val, ok := values[slug]; ok {
			result[slug] = val
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

type SearchResult struct {
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	MetaDescription string `json:"metaDescription"`
}

// PublicAuthorResponse is the safe, public-facing shape for a published author.
// It deliberately omits email, role, and status — those stay behind
// authentication (per Lesstruct's no-enumeration model). PublicFields carries
// the raw values of custom/system fields that are allowlisted with the
// "expose" operation in the [[public_field]] config. When no fields are
// allowlisted, the key is omitted from the JSON response (omitempty).
type PublicAuthorResponse struct {
	Username     string         `json:"username"`
	DisplayName  string         `json:"displayName"`
	AvatarURL    string         `json:"avatarURL"`
	ProfileURL   string         `json:"profileURL"`
	ContentCount int            `json:"contentCount"`
	PostTypes    []string       `json:"postTypes"`
	PublicFields map[string]any `json:"publicFields,omitempty"`
}

// PublicArchiveMonthResponse is the public-facing shape for one month in a
// content archive. URL points to the listing page filtered by year/month.
type PublicArchiveMonthResponse struct {
	Year  int    `json:"year"`
	Month int    `json:"month"`
	Count int    `json:"count"`
	URL   string `json:"url"`
}

type CreateContentRequest struct {
	Title              string         `json:"title"`
	Slug               string         `json:"slug,omitempty"`
	Content            string         `json:"content"`
	Format             string         `json:"format,omitempty"`
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
	Create(ctx context.Context, userID int, role string, req contentdomain.CreateContentRequest) (*contentdomain.Content, error)
	GetByUser(ctx context.Context, userID int, limit int, offset int) ([]*contentdomain.Content, error)
	GetAll(ctx context.Context, limit int, offset int) ([]*contentdomain.Content, error)
	GetByID(ctx context.Context, id int) (*contentdomain.Content, error)
	GenerateSlugFromTitle(ctx context.Context, title string) (string, error)
	Update(ctx context.Context, id int, userID int, role string, req contentdomain.UpdateContentRequest) (*contentdomain.Content, error)
	DeleteContent(ctx context.Context, id int, userID int, role string) error
	GetPublished(ctx context.Context, limit int, offset int) ([]*contentdomain.Content, error)
	GetPublishedBySlug(ctx context.Context, slug string, language string) (*contentdomain.Content, error)
	GetTranslations(ctx context.Context, translationGroupID int, excludeID int) ([]*contentdomain.Content, error)
	GetPublishedByAuthorUsername(ctx context.Context, username string, languages []string, limit int, offset int) ([]*contentdomain.Content, error)
	AuthorExists(ctx context.Context, username string) (bool, error)
	ListByFilters(ctx context.Context, userID int, filters contentdomain.ContentFilters) ([]*contentdomain.Content, error)
	Count(ctx context.Context, userID int, filters contentdomain.ContentFilters) (int, error)
	SetSystemFields(ctx context.Context, contentID int, userID int, systemFields map[string]any) (*contentdomain.Content, error)
	SearchPublished(ctx context.Context, query string, limit int) ([]*contentdomain.Content, error)
	GetPublishedAuthors(ctx context.Context, filters contentdomain.PublishedAuthorFilters) ([]*contentdomain.PublishedAuthor, error)
	GetPublishedAuthor(ctx context.Context, username string) (*contentdomain.PublishedAuthor, error)
	GetPublishedArchive(ctx context.Context, postType string, languages []string) ([]*contentdomain.ArchiveMonth, error)
	GetPublishedByPostType(ctx context.Context, postType string, languages []string, year int, month int, limit int, offset int) ([]*contentdomain.Content, error)
}

type ContentHandler struct {
	contentService            ContentServiceInterface
	logger                    *util.Logger
	baseURL                   string
	profilePictureURLResolver func(string) string
	featuredImageResolver     func(string) string
	publicFieldRegistry       PublicFieldLookup
	iframeHosts               []string
}

// PublicFieldLookup is the minimal subset of *config.PublicFieldRegistry that
// ContentHandler consults to enforce the [[public_field]] allowlist. Defining
// it locally keeps the handler package decoupled from the config package and
// makes the handler trivially testable with a stub.
type PublicFieldLookup interface {
	IsQueryable(resource, postType, field, operation string) bool
	ExposedFields(resource, postType string) []string
}

// WithPublicFieldRegistry attaches a [[public_field]] registry to the handler
// so /api/v1/public/* endpoints can enforce the public query allowlist. When
// not called (or called with nil), the handler rejects every cf_*/sort_by=cf:*
// parameter — fail-closed is the safe default. Returns the receiver for
// chaining at construction time.
func (h *ContentHandler) WithPublicFieldRegistry(registry PublicFieldLookup) *ContentHandler {
	h.publicFieldRegistry = registry
	return h
}

// WithIFrameHosts attaches the sanitizer's iframe host allowlist (derived from
// the CSP frame-src directive) so HTML-format content keeps allowed embeds on
// the write path. When not called, iframes are stripped on save. Returns the
// receiver for chaining at construction time.
func (h *ContentHandler) WithIFrameHosts(hosts ...string) *ContentHandler {
	h.iframeHosts = append(h.iframeHosts, hosts...)
	return h
}

type publicContentItem struct {
	*contentdomain.Content
	FeaturedImage string `json:"featuredImage,omitempty"`
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

	role, _ := middleware.GetRole(r)

	var req CreateContentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request body: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	format, errMsg := normalizeBrowserFormat(req.Format)
	if errMsg != "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_format", errMsg, nil)
		return
	}

	body, errMsg := convertBrowserContentBody(req.Content, format, h.iframeHosts)
	if errMsg != "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content", errMsg, nil)
		return
	}

	// The slug is immutable after creation: any user may supply one at create
	// time (validated and uniqueness-checked by the service); it can never be
	// changed via Update.
	slug := req.Slug

	contentReq := contentdomain.CreateContentRequest{
		Title:              req.Title,
		Slug:               slug,
		Content:            body,
		Format:             format,
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

	content, err := h.contentService.Create(r.Context(), userID, role, contentReq)
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
	status := r.URL.Query().Get("status")
	if status != "" && !contentdomain.Status(status).IsValid() {
		status = ""
	}

	var (
		contents []*contentdomain.Content
		total    int
	)

	if postType != "" || search != "" || len(customFieldFilters) > 0 || language != "" || status != "" {
		filters := contentdomain.ContentFilters{
			Limit:              limit,
			Offset:             offset,
			PostType:           postType,
			Search:             search,
			Language:           language,
			Status:             status,
			CustomFieldFilters: customFieldFilters,
		}
		filterUserID := userID
		if isAdmin {
			filterUserID = 0
		}
		contents, err = h.contentService.ListByFilters(r.Context(), filterUserID, filters)
		if err != nil {
			h.logger.Error("Failed to list contents by filters: %v", err)
			handleContentError(w, err)
			return
		}
		total, err = h.contentService.Count(r.Context(), filterUserID, filters)
		if err != nil {
			h.logger.Error("Failed to count contents by filters: %v", err)
			handleContentError(w, err)
			return
		}
	} else if isAdmin {
		contents, err = h.contentService.GetAll(r.Context(), limit, offset)
		if err != nil {
			h.logger.Error("Failed to list all contents: %v", err)
			handleContentError(w, err)
			return
		}
		total, err = h.contentService.Count(r.Context(), 0, contentdomain.ContentFilters{})
		if err != nil {
			h.logger.Error("Failed to count all contents: %v", err)
			handleContentError(w, err)
			return
		}
	} else {
		contents, err = h.contentService.GetByUser(r.Context(), userID, limit, offset)
		if err != nil {
			h.logger.Error("Failed to list contents: %v", err)
			handleContentError(w, err)
			return
		}
		total, err = h.contentService.Count(r.Context(), userID, contentdomain.ContentFilters{})
		if err != nil {
			h.logger.Error("Failed to count contents: %v", err)
			handleContentError(w, err)
			return
		}
	}

	hasMore := total > offset+len(contents)
	response.SuccessList(
		w,
		contents,
		response.ListMeta{Pagination: response.Pagination{
			Total:   &total,
			Limit:   &limit,
			Offset:  &offset,
			HasMore: hasMore,
		}},
	)
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

	format, errMsg := normalizeBrowserFormat(req.Format)
	if errMsg != "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_format", errMsg, nil)
		return
	}

	body, errMsg := convertBrowserContentBody(req.Content, format, h.iframeHosts)
	if errMsg != "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content", errMsg, nil)
		return
	}

	role, _ := middleware.GetRole(r)

	updateReq := contentdomain.UpdateContentRequest{
		Title:              req.Title,
		Content:            body,
		Format:             format,
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
		"format":             content.Format,
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

	postType := r.URL.Query().Get("post_type")
	language := r.URL.Query().Get("language")
	customFieldFilters := parseCustomFieldFilters(r)
	sortBy, sortOrder, sortErr := parsePublicSortBy(r)
	if sortErr != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_sort", sortErr.Error(), nil)
		return
	}

	// Public custom-field queries are opt-in via the [[public_field]] allowlist.
	// When the caller asks for any cf_* filter or sort_by=cf:*, every referenced
	// field must be allowlisted for the content resource (and the request's
	// post_type when the entry is post-type-scoped). Without an entry here, the
	// request fails closed with 400 field_not_queryable.
	if len(customFieldFilters) > 0 || sortBy != "" {
		if err := enforcePublicFieldAllowlist(
			h.publicFieldRegistry,
			config.PublicFieldResourceContent,
			postType,
			customFieldFilters,
			sortBy,
			sortOrder,
		); err != nil {
			sendErrorResponse(w, http.StatusBadRequest, "field_not_queryable", err.Error(), nil)
			return
		}
	}

	var contents []*contentdomain.Content
	var err error

	if len(customFieldFilters) > 0 || sortBy != "" || language != "" {
		filters := contentdomain.ContentFilters{
			Limit:              limit,
			Offset:             offset,
			PostType:           postType,
			Language:           language,
			Status:             string(contentdomain.StatusPublished),
			CustomFieldFilters: customFieldFilters,
			SortBy:             sortBy,
			SortOrder:          sortOrder,
		}
		contents, err = h.contentService.ListByFilters(r.Context(), 0, filters)
	} else if postType != "" {
		contents, err = h.contentService.GetPublishedByPostType(r.Context(), postType, nil, 0, 0, limit, offset)
	} else {
		contents, err = h.contentService.GetPublished(r.Context(), limit, offset)
	}

	if err != nil {
		h.logger.Error("Failed to list published content: %v", err)
		handleContentError(w, err)
		return
	}

	items := make([]publicContentItem, 0, len(contents))
	for _, c := range contents {
		item := publicContentItem{Content: c}
		img := seo.ExtractImageURL(c.Content)
		if img != "" && h.featuredImageResolver != nil {
			img = h.featuredImageResolver(img)
		}
		if img != "" {
			item.FeaturedImage = seo.BuildURL(h.baseURL, img)
		}
		items = append(items, item)
	}

	sendSuccessResponse(w, http.StatusOK, items)
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

	contents, err := h.contentService.GetPublishedByAuthorUsername(r.Context(), username, nil, limit, offset)
	if err != nil {
		h.logger.Error("Failed to list published content by author: %v", err)
		handleContentError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusOK, contents)
}

// ListPublishedAuthors returns the users who have published at least one
// content item, with only safe public fields (username, display name, avatar,
// profile URL, published-content count, and the distinct post types they
// publish under). No email, role, or custom fields are exposed. Ordered by
// published-content count (desc) then username (asc) — useful for "most active
// contributors" widgets and author directories.
func (h *ContentHandler) ListPublishedAuthors(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if limitQuery := r.URL.Query().Get("limit"); limitQuery != "" {
		if l, err := strconv.Atoi(limitQuery); err == nil && l > 0 {
			limit = l
		}
	}
	if limit > 100 {
		limit = 100
	}

	offset := 0
	if offsetQuery := r.URL.Query().Get("offset"); offsetQuery != "" {
		if o, err := strconv.Atoi(offsetQuery); err == nil && o >= 0 {
			offset = o
		}
	}

	customFieldFilters := parseCustomFieldFilters(r)
	sortBy, sortOrder, sortErr := parsePublicSortBy(r)
	if sortErr != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_sort", sortErr.Error(), nil)
		return
	}

	if len(customFieldFilters) > 0 || sortBy != "" {
		if err := enforcePublicFieldAllowlist(
			h.publicFieldRegistry,
			config.PublicFieldResourceUser,
			"",
			customFieldFilters,
			sortBy,
			sortOrder,
		); err != nil {
			sendErrorResponse(w, http.StatusBadRequest, "field_not_queryable", err.Error(), nil)
			return
		}
	}

	authors, err := h.contentService.GetPublishedAuthors(r.Context(), contentdomain.PublishedAuthorFilters{
		Limit:              limit,
		Offset:             offset,
		SortBy:             sortBy,
		SortOrder:          sortOrder,
		CustomFieldFilters: customFieldFilters,
	})
	if err != nil {
		h.logger.Error("Failed to list published authors: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to list published authors", nil)
		return
	}

	responses := make([]PublicAuthorResponse, 0, len(authors))
	for _, a := range authors {
		var avatarURL string
		if a.ProfilePicture != "" && h.profilePictureURLResolver != nil {
			avatarURL = h.profilePictureURLResolver(a.ProfilePicture)
		}
		responses = append(responses, PublicAuthorResponse{
			Username:     a.Username,
			DisplayName:  a.DisplayName,
			AvatarURL:    avatarURL,
			ProfileURL:   h.baseURL + "/authors/" + a.Username,
			ContentCount: a.ContentCount,
			PostTypes:    a.PostTypes,
			PublicFields: projectPublicFields(h.publicFieldRegistry, config.PublicFieldResourceUser, "", a.CustomFields),
		})
	}

	sendSuccessResponse(w, http.StatusOK, responses)
}

// GetPublishedAuthor returns a single published author's public profile.
// It follows the same response shape as the ListPublishedAuthors endpoint
// (PublicAuthorResponse) but for one author. Returns 404 when the author has
// no published content or does not exist.
func (h *ContentHandler) GetPublishedAuthor(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_username", "Username is required", nil)
		return
	}

	author, err := h.contentService.GetPublishedAuthor(r.Context(), username)
	if err != nil {
		h.logger.Error("Failed to get published author: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to get published author", nil)
		return
	}
	if author == nil {
		sendErrorResponse(w, http.StatusNotFound, "author_not_found", "Author not found", nil)
		return
	}

	var avatarURL string
	if author.ProfilePicture != "" && h.profilePictureURLResolver != nil {
		avatarURL = h.profilePictureURLResolver(author.ProfilePicture)
	}

	response := PublicAuthorResponse{
		Username:     author.Username,
		DisplayName:  author.DisplayName,
		AvatarURL:    avatarURL,
		ProfileURL:   h.baseURL + "/authors/" + author.Username,
		ContentCount: author.ContentCount,
		PostTypes:    author.PostTypes,
		PublicFields: projectPublicFields(h.publicFieldRegistry, config.PublicFieldResourceUser, "", author.CustomFields),
	}

	sendSuccessResponse(w, http.StatusOK, response)
}

// ListPublishedArchive returns published-content counts grouped by year and
// month, newest first. Optional ?post_type filters to a single post type;
// optional ?language filters to a single language. Each entry includes a URL
// pointing to the matching listing page with ?year= and ?month= params.
func (h *ContentHandler) ListPublishedArchive(w http.ResponseWriter, r *http.Request) {
	postType := r.URL.Query().Get("post_type")
	language := r.URL.Query().Get("language")
	var languages []string
	if language != "" {
		languages = []string{language}
	}

	archive, err := h.contentService.GetPublishedArchive(r.Context(), postType, languages)
	if err != nil {
		h.logger.Error("Failed to list published archive: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to list published archive", nil)
		return
	}

	listingBase := "/"
	if postType != "" {
		listingBase = "/" + postType
	}

	responses := make([]PublicArchiveMonthResponse, 0, len(archive))
	for _, m := range archive {
		responses = append(responses, PublicArchiveMonthResponse{
			Year:  m.Year,
			Month: m.Month,
			Count: m.Count,
			URL:   fmt.Sprintf("%s%s?year=%d&month=%d", h.baseURL, listingBase, m.Year, m.Month),
		})
	}

	sendSuccessResponse(w, http.StatusOK, responses)
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

	var req struct {
		SystemFields map[string]any `json:"systemFields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request body: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	content, err := h.contentService.SetSystemFields(r.Context(), contentID, userID, req.SystemFields)
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

func NewContentHandler(
	contentService ContentServiceInterface,
	logger *util.Logger,
	baseURL string,
	profilePictureURLResolver func(string) string,
	featuredImageResolver func(string) string,
) *ContentHandler {
	return &ContentHandler{
		contentService:            contentService,
		logger:                    logger,
		baseURL:                   strings.TrimRight(baseURL, "/"),
		profilePictureURLResolver: profilePictureURLResolver,
		featuredImageResolver:     featuredImageResolver,
	}
}
