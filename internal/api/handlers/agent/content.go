package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/content/markdown"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

// Supported content formats for the agent create surface.
const (
	// formatTiptap is the canonical content storage format and the default when a
	// request omits "format". The body is the Tiptap JSON string, stored unchanged.
	formatTiptap = "tiptap"
	// formatMarkdown converts the Markdown body to canonical Tiptap JSON via
	// internal/content/markdown before storage. Raw Markdown is never persisted.
	formatMarkdown = "markdown"
)

// List pagination bounds for the agent v1 list surface. Missing/invalid/negative limit
// falls back to the default; anything over the max is clamped down (clamp, never reject,
// so agents can pass a large limit safely). The handler always requests limit+1 from the
// service to compute hasMore.
const (
	defaultListLimit = 50
	minListLimit     = 1
	maxListLimit     = 100
)

// authenticatedUserID reads the injected owning user id from request context using
// the shared, auth-agnostic identity accessor (works for JWT and API key). Returns
// ok=false when the user is missing — a defensive case that only occurs if the
// handler is reached without the Bearer middleware having injected identity.
func authenticatedUserID(r *http.Request) (int, bool) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		return 0, false
	}
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		return 0, false
	}
	return userID, true
}

// authenticatedRole reads the injected role from request context. Returns "" when absent
// (a defensive case — the Bearer middleware always sets both identity and role together,
// so a present userID implies a present role). "" never equals RoleAdmin, so a caller
// that is neither owner nor admin is treated as unauthorized (404, no disclosure).
func authenticatedRole(r *http.Request) string {
	role, ok := middleware.GetRole(r)
	if !ok {
		return ""
	}
	return role
}

// normalizeContentFormat trims + lowercases the format, defaults "" to tiptap, and returns
// the effective format plus an error message. An empty message means the format is
// supported (tiptap is stored unchanged; markdown is converted to Tiptap by the caller).
// Shared by Create and Update so the normalize + switch lives in exactly one place.
func normalizeContentFormat(format string) (string, string) {
	normalized := strings.ToLower(strings.TrimSpace(format))
	if normalized == "" {
		normalized = formatTiptap
	}
	switch normalized {
	case formatTiptap:
		return formatTiptap, ""
	case formatMarkdown:
		return formatMarkdown, ""
	default:
		return "", "Unsupported format: " + normalized
	}
}

// resolveContentBody normalizes the request format and returns the canonical content
// string to store. For format=markdown it converts the Markdown body to Tiptap JSON via
// internal/content/markdown (raw Markdown is never persisted); for format=tiptap (the
// default) it returns the body unchanged. A non-empty errMsg means the format is invalid
// or conversion failed and must be surfaced as 400 VALIDATION_ERROR. Shared by Create
// and Update so the normalize + convert logic lives in exactly one place.
func resolveContentBody(req ContentRequest) (body string, errMsg string) {
	normalized, formatErr := normalizeContentFormat(req.Format)
	if formatErr != "" {
		return "", formatErr
	}
	if normalized == formatMarkdown {
		converted, err := markdown.Convert(req.Body)
		if err != nil {
			return "", "Invalid markdown: " + err.Error()
		}
		return converted, ""
	}
	return req.Body, ""
}

// parseListLimit clamps the ?limit query param into [minListLimit, maxListLimit] with a
// default. Missing/invalid/negative → default; over-max → max. Mirrors the admin
// ListContents clamp convention with tighter agent bounds.
func parseListLimit(r *http.Request) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultListLimit
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < minListLimit {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}

// ContentService is the narrow slice of the content domain service the agent
// handlers depend on. *contentdomain.Service satisfies it. Declaring it locally —
// instead of depending on the admin handler's wider ContentServiceInterface — keeps
// the agent surface independent and makes the dependency explicit. It is exported
// so mockery can generate an exported constructor callable from the package's
// *_test files (CLAUDE.md mandates *_test packages, which cannot reach an
// unexported mock).
type ContentService interface {
	Create(ctx context.Context, userID int, req contentdomain.CreateContentRequest) (*contentdomain.Content, error)
	GetByID(ctx context.Context, id int) (*contentdomain.Content, error)
	ListByCursor(ctx context.Context, userID int, limit int, beforeID int, filters contentdomain.ContentFilters) ([]*contentdomain.Content, error)
	Update(ctx context.Context, id int, userID int, role string, req contentdomain.UpdateContentRequest) (*contentdomain.Content, error)
	DeleteContent(ctx context.Context, id int, userID int, role string) error
	Publish(ctx context.Context, id int, userID int, role string) (*contentdomain.Content, error)
	Unpublish(ctx context.Context, id int, userID int, role string) (*contentdomain.Content, error)
}

// ContentHandler exposes the Bearer-authenticated agent content endpoints. It
// reuses the existing ContentService (which already validates custom fields and
// fires plugin hooks) — it never duplicates that logic.
type ContentHandler struct {
	contentService ContentService
	logger         *util.Logger
}

// Create handles POST /api/v1/content. It maps the streamlined agent payload onto a
// contentdomain.CreateContentRequest (reusing the same validation + plugin hook
// path as the admin surface) and returns the created content in the envelope.
func (h *ContentHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	var req ContentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}

	// Resolve the canonical content body. format=tiptap (default) stores the body
	// unchanged; format=markdown converts the Markdown body to Tiptap JSON via
	// internal/content/markdown so raw Markdown is never persisted (the service's
	// ValidateContent path then unmarshals + sanitizes the Tiptap). An unknown
	// format or a conversion failure is surfaced as VALIDATION_ERROR.
	body, formatErr := resolveContentBody(req)
	if formatErr != "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", formatErr, nil)
		return
	}

	status := contentdomain.StatusDraft
	if req.IsPublished {
		status = contentdomain.StatusPublished
	}

	// Slug is intentionally NOT mapped: CreateContentRequest has no Slug field and
	// the service auto-generates the slug from the title. See Completion Notes.
	// Tags and Language are forwarded as-given; the service normalizes Tags via
	// contentdomain.ValidateTags and rejects an unknown Language with
	// ErrInvalidLanguage (mapped to VALIDATION_ERROR by handleError).
	domainReq := contentdomain.CreateContentRequest{
		Title:        req.Title,
		Content:      body,
		Status:       status,
		PostType:     req.PostType,
		CustomFields: req.CustomFields,
		Tags:         req.Tags,
		Language:     req.Language,
	}

	created, err := h.contentService.Create(r.Context(), userID, domainReq)
	if err != nil {
		h.logger.Error("agent create content failed: userID=%d err=%v", userID, err)
		handleError(w, err)
		return
	}

	response.Success(w, NewContentResponse(created))
}

// Get handles GET /api/v1/content/{id}. It returns the content in the envelope, or
// 404 NOT_FOUND when it does not exist OR the caller is not allowed to see it.
//
// Visibility scoping (review fix for the GET IDOR): the underlying GetByID returns
// content regardless of owner/status, so the handler enforces "published OR owned by
// the requesting user" and otherwise returns NOT_FOUND (never FORBIDDEN, so existence
// is not disclosed to unauthorized callers). This mirrors the ownership pattern the
// admin Update/DeleteContent paths use (existing.UserID != userID).
func (h *ContentHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid content ID", nil)
		return
	}

	content, err := h.contentService.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("agent get content failed: id=%d err=%v", id, err)
		handleError(w, err)
		return
	}

	// Scope by visibility: only the owner may read a draft; anyone may read published.
	if content.Status != contentdomain.StatusPublished && content.UserID != userID {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "Content not found", nil)
		return
	}

	response.Success(w, NewContentResponse(content))
}

// minListSearchLength is the minimum run length of the ?search= query param; shorter
// values are dropped to the empty filter (no search applied). Mirrors the admin
// ListContents handler's behavior in content.go.
const minListSearchLength = 2

// parseListFilters reads the agent list's filter query params into a contentdomain
// ContentFilters. Empty values (after trim) are dropped, so a zero ContentFilters is
// returned when the caller passes no filter params. status is validated via
// Status.IsValid — an unknown value returns an empty string AND the validation error
// so the caller can map it to 400 VALIDATION_ERROR. The minListSearchLength rule
// applies to search only (so a single-character search is silently dropped, mirroring
// the admin surface). The author filter is NOT enforced here — the agent List handler
// rejects non-admin callers with 403 before calling this helper, so the Author field
// is only ever set for admins.
func parseListFilters(r *http.Request) (contentdomain.ContentFilters, error) {
	q := r.URL.Query()

	postType := strings.TrimSpace(q.Get("post_type"))
	language := strings.TrimSpace(q.Get("language"))
	status := strings.TrimSpace(q.Get("status"))
	author := strings.TrimSpace(q.Get("author"))
	search := strings.TrimSpace(q.Get("search"))
	if len(search) < minListSearchLength {
		search = ""
	}

	// Repeated ?tag=foo&tag=bar → AND-of-tags. Trim each entry, drop empties (the
	// caller might pass ?tag= with a trailing empty value), preserve first-occurrence
	// order. nil/empty input yields nil (no filter clause emitted).
	tags := make([]string, 0)
	for _, raw := range q["tag"] {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		tags = append(tags, t)
	}
	if len(tags) == 0 {
		tags = nil
	}

	if status != "" && !contentdomain.Status(status).IsValid() {
		return contentdomain.ContentFilters{}, fmt.Errorf("invalid status: %q (want draft or published)", status)
	}

	return contentdomain.ContentFilters{
		PostType: postType,
		Language: language,
		Status:   status,
		Author:   author,
		Search:   search,
		Tags:     tags,
	}, nil
}

// List handles GET /api/v1/content. It returns the caller's own content (any status —
// drafts + published) in newest-first order using opaque keyset (cursor) pagination,
// each item projected via the ContentProjection whitelist. hasMore/nextCursor are
// computed by requesting limit+1 rows; nextCursor is the opaque token of the last item
// on the current page and is only present when there is another page.
//
// Optional filters: ?tag=foo&tag=bar (AND-of-tags), ?language=en, ?status=draft|published,
// ?post_type=post, ?author=alice (admin only — non-admins get 403), ?search=foo
// (min 2 chars). All filters AND with the cursor pagination. The agent v1 surface
// scopes to the caller's own content; the author filter is meaningful only for admins
// because the agent handler never expands the userID scope (admin still sees their own
// content here — use the admin surface for cross-user listings).
func (h *ContentHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	limit := parseListLimit(r)
	beforeID, err := decodeCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid cursor", nil)
		return
	}

	filters, err := parseListFilters(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	// author is admin-only on the agent surface: the agent endpoint never expands
	// to cross-user listings, so the filter is a no-op for non-admins; reject it
	// explicitly so a non-admin cannot be misled into thinking the filter is active.
	if filters.Author != "" && authenticatedRole(r) != contentdomain.RoleAdmin {
		response.Error(w, http.StatusForbidden, "FORBIDDEN", "author filter is only available for admins", nil)
		return
	}

	items, err := h.contentService.ListByCursor(r.Context(), userID, limit+1, beforeID, filters)
	if err != nil {
		h.logger.Error("agent list content failed: userID=%d err=%v", userID, err)
		handleError(w, err)
		return
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	nextCursor := ""
	if hasMore && len(items) > 0 {
		nextCursor = encodeCursor(items[len(items)-1].ID)
	}

	projections := make([]ContentProjection, 0, len(items))
	for _, c := range items {
		projections = append(projections, NewContentResponse(c).Content)
	}

	response.SuccessList(
		w,
		projections,
		response.ListMeta{Pagination: response.Pagination{NextCursor: nextCursor, HasMore: hasMore}},
	)
}

// Update handles PUT /api/v1/content/{id}. It pre-fetches the existing content for an
// ownership check (404 NOT_FOUND when the caller is neither owner nor Admin — existence
// is never disclosed) and then delegates to the existing ContentService.Update so hooks
// + custom-field validation run through the same domain path the admin uses. format=tiptap
// stores the body unchanged; markdown is rejected (Story 2.4).
//
// v1 settable on update: title, body, format, postType, customFields, isPublished,
// tags, language. Server-managed (preserved from the existing item): SEO metadata
// (metaDescription, ogTitle, ogDescription), allowComments, translationGroupId.
func (h *ContentHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	role := authenticatedRole(r)

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid content ID", nil)
		return
	}

	var req ContentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}

	// Resolve the canonical content body (tiptap stored unchanged; markdown
	// converted to Tiptap so raw Markdown is never persisted). The ownership
	// pre-fetch below runs after resolution — conversion is a cheap, pure step.
	body, formatErr := resolveContentBody(req)
	if formatErr != "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", formatErr, nil)
		return
	}

	existing, err := h.contentService.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("agent update content pre-fetch failed: id=%d err=%v", id, err)
		handleError(w, err)
		return
	}

	// Ownership / no-enumeration: never 403 — return 404 so existence is not disclosed,
	// consistent with the Story 2.1 GET visibility fix.
	if existing.UserID != userID && role != contentdomain.RoleAdmin {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "Content not found", nil)
		return
	}

	status := contentdomain.StatusDraft
	if req.IsPublished {
		status = contentdomain.StatusPublished
	}

	// Override the v1-supplied fields and preserve every server-managed field
	// (SEO metadata, allowComments, translationGroupId) from the existing item.
	// CustomFields is passed through verbatim: nil → service keeps existing, {} →
	// clears, {...} → replaces (the service guards the replace on
	// req.CustomFields != nil). Tags and Language are forwarded as-given; the
	// service normalizes Tags via contentdomain.ValidateTags and rejects an
	// unknown Language with ErrInvalidLanguage (mapped to VALIDATION_ERROR by
	// handleError).
	domainReq := contentdomain.UpdateContentRequest{
		Title:              req.Title,
		Content:            body,
		Status:             status,
		CustomFields:       req.CustomFields,
		PostType:           req.PostType,
		Tags:               req.Tags,
		Language:           req.Language,
		MetaDescription:    existing.MetaDescription,
		OGTitle:            existing.OGTitle,
		OGDescription:      existing.OGDescription,
		AllowComments:      &existing.AllowComments,
		TranslationGroupID: existing.TranslationGroupID,
	}

	updated, err := h.contentService.Update(r.Context(), id, userID, role, domainReq)
	if err != nil {
		h.logger.Error("agent update content failed: id=%d err=%v", id, err)
		handleError(w, err)
		return
	}

	response.Success(w, NewContentResponse(updated))
}

// Delete handles DELETE /api/v1/content/{id}. It pre-fetches for the ownership check
// (404 on not-found or not-owned-per-role) then delegates to the existing
// ContentService.DeleteContent. A successful delete returns 204 No Content; a subsequent
// GET returns 404 (the service hard-deletes for Admins and scoped-deletes otherwise).
func (h *ContentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	role := authenticatedRole(r)

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid content ID", nil)
		return
	}

	existing, err := h.contentService.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("agent delete content pre-fetch failed: id=%d err=%v", id, err)
		handleError(w, err)
		return
	}

	if existing.UserID != userID && role != contentdomain.RoleAdmin {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "Content not found", nil)
		return
	}

	if err := h.contentService.DeleteContent(r.Context(), id, userID, role); err != nil {
		h.logger.Error("agent delete content failed: id=%d err=%v", id, err)
		handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Publish handles POST /api/v1/content/{id}/publish. It delegates to
// ContentService.Publish so ownership, the draft→published transition, SEO
// auto-generation, and the AfterPublish hook fire through the same domain
// path the Update endpoint uses. The endpoint accepts an empty body and is
// idempotent: publishing an already-published post returns 200 with the
// current projection, fires no hook, runs no SEO regen.
//
// A non-admin caller must be the owner; otherwise 404 (never 403, so
// existence is not disclosed). An admin can publish another user's content.
// Validation and persistence errors map to the same error envelope the
// other endpoints use.
func (h *ContentHandler) Publish(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	role := authenticatedRole(r)

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid content ID", nil)
		return
	}

	updated, err := h.contentService.Publish(r.Context(), id, userID, role)
	if err != nil {
		// The service's ownership check returns ErrUnauthorized on a
		// not-owned-by-non-admin case; surface that as 404 (no disclosure),
		// matching the Update + Delete conventions.
		if errors.Is(err, contentdomain.ErrUnauthorized) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Content not found", nil)
			return
		}
		h.logger.Error("agent publish content failed: id=%d err=%v", id, err)
		handleError(w, err)
		return
	}

	response.Success(w, NewContentResponse(updated))
}

// Unpublish handles POST /api/v1/content/{id}/unpublish. It delegates to
// ContentService.Unpublish so the status flip persists through the same
// domain path. The endpoint accepts an empty body and is idempotent:
// unpublishing an already-draft post returns 200 with the current
// projection, fires no hook, runs no SEO regen. Unpublish never fires the
// AfterPublish hook (the hook is wired to the draft→published edge only).
//
// Same ownership/role contract as Publish: 404 (not 403) for non-admin
// non-owners; admins can unpublish anyone's content.
func (h *ContentHandler) Unpublish(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	role := authenticatedRole(r)

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid content ID", nil)
		return
	}

	updated, err := h.contentService.Unpublish(r.Context(), id, userID, role)
	if err != nil {
		if errors.Is(err, contentdomain.ErrUnauthorized) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Content not found", nil)
			return
		}
		h.logger.Error("agent unpublish content failed: id=%d err=%v", id, err)
		handleError(w, err)
		return
	}

	response.Success(w, NewContentResponse(updated))
}

// NewContentHandler constructs a ContentHandler backed by the given content
// service. A nil logger degrades to a discard sink so the handler is safe to
// construct in any context (mirrors NewAPIKeyHandler/NewAPIKeyAuthMiddleware).
func NewContentHandler(s ContentService, logger *util.Logger) *ContentHandler {
	if logger == nil {
		logger = util.NewLogger(io.Discard)
	}
	return &ContentHandler{
		contentService: s,
		logger:         logger,
	}
}
