package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/httpx"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
)

// adminRoleName is the reserved administrator role. It matches the users
// module's reserved name; the two reconcile when that module lands.
const adminRoleName = "Admin"

// ErrContentNotFound is returned by the ContentLookup when no content matches
// the ID. Core translates the content lookup failure to it so this package
// never imports the content domain.
var ErrContentNotFound = errors.New("comments: content not found")

func authenticatedPrincipal(r *http.Request) (kernel.Principal, bool) {
	return kernel.PrincipalFromContext(r.Context())
}

func authenticatedUserID(r *http.Request) (int, bool) {
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		return 0, false
	}
	userID, err := strconv.Atoi(principal.ID)
	if err != nil {
		return 0, false
	}
	return userID, true
}

func authenticatedRole(r *http.Request) string {
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok || len(principal.Roles) == 0 {
		return ""
	}
	return principal.Roles[0]
}

func handleCommentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrContentNotFound),
		errors.Is(err, commentdomain.ErrContentNotFound):
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Content not found", nil)
	case errors.Is(err, commentdomain.ErrCommentNotFound):
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Comment not found", nil)
	case errors.Is(err, commentdomain.ErrCommentsDisabled):
		httpx.Error(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this content", nil)
	case errors.Is(err, commentdomain.ErrInvalidCommentText),
		errors.Is(err, commentdomain.ErrInvalidCommentStatus),
		errors.Is(err, kernel.ErrInvalidPrincipalID):
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
	default:
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
	}
}

// resolveContent loads the content detail for visibility and gate checks. A
// missing item answers 404 NOT_FOUND here so no handler duplicates the
// mapping; other lookup failures surface as 500.
func resolveContent(w http.ResponseWriter, r *http.Request, lookup ContentLookup, contentID int) (ContentDetail, bool) {
	content, err := lookup(r.Context(), contentID)
	if err != nil {
		if errors.Is(err, ErrContentNotFound) {
			httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Content not found", nil)
			return ContentDetail{}, false
		}
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
		return ContentDetail{}, false
	}
	return content, true
}

// visibleToCaller reports whether the caller may interact with the content:
// published items are visible to everyone; drafts only to the owner or an
// Admin. Anyone else gets 404, never 403, so existence is not disclosed.
func visibleToCaller(content ContentDetail, userID int, role string) bool {
	return content.Published || content.UserID == userID || role == adminRoleName
}

// discardLogger drops log lines. It backs the nil-logger fallback in
// NewCommentHandler so the handler is safe to construct in any context.
type discardLogger struct{}

func (discardLogger) Info(string, ...any) {}

func (discardLogger) Error(string, ...any) {}

// RoleCapabilities is the narrow role-registry seam this handler depends on.
// Core satisfies it with an adapter over *role.Service (which cannot be
// imported here); the users module implements it natively when it lands.
type RoleCapabilities interface {
	CanComment(name string) bool
}

// ContentDetail is the comment-relevant projection of a content item: its ID
// and owner for visibility checks, its published flag, and its AllowComments
// flag for gating.
type ContentDetail struct {
	ID            int
	UserID        int
	Published     bool
	AllowComments bool
}

// ContentLookup resolves a content ID to its comment-relevant facts. Core
// satisfies it with a closure over the content service (which cannot be
// imported here); tests stub it with a func literal.
type ContentLookup func(ctx context.Context, id int) (ContentDetail, error)

// CommentHandlerOption configures a CommentHandler. A role service makes the
// comment capability config-driven; without one every authenticated user may
// comment (the legacy behavior).
type CommentHandlerOption func(*CommentHandler)

// WithCommentRoleService attaches the config-driven role registry so posting comments
// is gated by the caller's CanComment capability.
func WithCommentRoleService(rs RoleCapabilities) CommentHandlerOption {
	return func(h *CommentHandler) {
		h.roleService = rs
	}
}

// CommentService is the narrow slice of comment operations the agent comment
// handlers depend on. *commentdomain.Service satisfies it. It is exported so
// mockery can generate an exported constructor callable from the package's
// *_test files, which cannot reach an unexported mock.
type CommentService interface {
	SubmitComment(ctx context.Context, contentID int, principal kernel.Principal, req commentdomain.CreateCommentRequest) (*commentdomain.Comment, error)
	GetCommentsForContent(ctx context.Context, contentID int) ([]*commentdomain.Comment, error)
	GetCommentsForModeration(ctx context.Context, contentID int) ([]*commentdomain.Comment, error)
	GetComment(ctx context.Context, commentID int) (*commentdomain.Comment, error)
	UpdateCommentStatus(ctx context.Context, commentID int, status commentdomain.CommentStatus) error
	DeleteComment(ctx context.Context, commentID int) error
	DeleteOwnComment(ctx context.Context, commentID int, principal kernel.Principal) error
}

// CommentHandler exposes the Bearer-authenticated agent comment endpoints. It
// reuses the comments module service (which already validates comment text
// and enforces the AllowComments gate) — it never duplicates that logic. The
// comment surface is nested under the agent's content-keyed namespace
// (/api/v1/content/{id}/comments) so it is collision-free vs the browser
// realm's /api/v1/content_items/.../comments and /api/v1/comments routes, and
// consistent with the agent surface that keys everything by content id.
type CommentHandler struct {
	commentService CommentService
	content        ContentLookup
	logger         kernel.Logger
	roleService    RoleCapabilities
}

// commentAllowed reports whether the caller's role may post comments. Without a
// role service every authenticated user may, matching the legacy behavior.
func (h *CommentHandler) commentAllowed(r *http.Request) bool {
	if h.roleService == nil {
		return true
	}
	return h.roleService.CanComment(authenticatedRole(r))
}

// Create handles POST /api/v1/content/{id}/comments. It resolves the content by
// id (404 NOT_FOUND when missing, and 404 — never 403 — when the caller is
// neither owner nor Admin and the content is a draft, so existence is not
// disclosed), rejects commenting when the content has comments disabled (403
// FORBIDDEN), then delegates to SubmitComment which validates the text (1–2000
// chars, no HTML → ErrInvalidCommentText → 400 VALIDATION_ERROR) and persists
// the comment in the "pending" moderation status. Like the rest of the agent
// surface, success is 200 (the browser realm returns 201; the agent realm
// standardizes on 200 via httpx.Success).
func (h *CommentHandler) Create(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	userID, ok := authenticatedUserID(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	role := authenticatedRole(r)

	if !h.commentAllowed(r) {
		httpx.Error(w, http.StatusForbidden, "FORBIDDEN", "Your role cannot post comments", nil)
		return
	}

	contentID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || contentID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid content ID", nil)
		return
	}

	var req commentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}

	content, ok := resolveContent(w, r, h.content, contentID)
	if !ok {
		return
	}

	// Visibility / no-enumeration: only the owner (or an Admin) may interact with a
	// draft; anyone may interact with published content. Mirrors the agent content
	// Get/Update/Delete visibility model.
	if !visibleToCaller(content, userID, role) {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Content not found", nil)
		return
	}

	if !content.AllowComments {
		httpx.Error(w, http.StatusForbidden, "FORBIDDEN", "Comments are not allowed on this content", nil)
		return
	}

	comment, err := h.commentService.SubmitComment(
		r.Context(),
		contentID,
		principal,
		commentdomain.CreateCommentRequest{Comment: req.Comment},
	)
	if err != nil {
		h.logger.Error("agent create comment failed: contentID=%d userID=%d err=%v", contentID, userID, err)
		handleCommentError(w, err)
		return
	}

	httpx.Success(w, NewCommentResponse(comment))
}

// List handles GET /api/v1/content/{id}/comments. An Admin sees the moderation
// view (every comment, any status — mirroring the browser realm's admin-gated
// moderation queue); any other caller sees only approved comments, mirroring the
// public GET /api/v1/public/content_items/{slug}/comments. Both are scoped by the
// same visibility model as Create (published, or owned by the caller, or Admin;
// else 404). Comments-disabled content yields an empty list for non-admins
// (admins still moderate); the data array is always present, even when empty.
func (h *CommentHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	role := authenticatedRole(r)

	contentID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || contentID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid content ID", nil)
		return
	}

	content, ok := resolveContent(w, r, h.content, contentID)
	if !ok {
		return
	}

	if !visibleToCaller(content, userID, role) {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Content not found", nil)
		return
	}

	// Comments are disabled: non-moderators see no queue — mirrors the public
	// GET /api/v1/public/content_items/{slug}/comments, which short-circuits to an
	// empty list when !AllowComments. Admins still moderate regardless, matching the
	// browser realm's moderation view (which ignores AllowComments).
	if !content.AllowComments && role != adminRoleName {
		httpx.SuccessList(w, []CommentProjection{}, nil)
		return
	}

	// Admin sees the full moderation queue (every status); a non-admin sees only
	// approved comments, so the pending/spam/rejected queue and its author/role
	// metadata are never leaked to a Commentator-level key. Mirrors the browser
	// realm, where the moderation queue sits behind ModerationOnly (admin-only).
	var comments []*commentdomain.Comment
	if role == adminRoleName {
		comments, err = h.commentService.GetCommentsForModeration(r.Context(), contentID)
	} else {
		comments, err = h.commentService.GetCommentsForContent(r.Context(), contentID)
	}
	if err != nil {
		h.logger.Error("agent list comments failed: contentID=%d err=%v", contentID, err)
		handleCommentError(w, err)
		return
	}

	projections := make([]CommentProjection, 0, len(comments))
	for _, c := range comments {
		projections = append(projections, NewCommentResponse(c).Comment)
	}

	httpx.SuccessList(w, projections, nil)
}

// Delete handles DELETE /api/v1/content/{id}/comments/{commentId}. The path
// content id is bound to the comment (a mismatch — the comment belongs to other
// content — is rejected with a clean 404, no disclosure), then an Admin may
// delete any comment (DeleteComment) while any other caller may delete only
// their own (DeleteOwnComment, which returns ErrCommentNotFound — a 404 with no
// ownership disclosure — when the comment is missing or belongs to someone
// else). Success is 204 No Content.
func (h *CommentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	userID, ok := authenticatedUserID(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	role := authenticatedRole(r)

	contentID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || contentID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid content ID", nil)
		return
	}
	commentID, err := strconv.Atoi(r.PathValue("commentId"))
	if err != nil || commentID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid comment ID", nil)
		return
	}

	// Bind the path content id to the comment so the /content/{id}/comments/{commentId}
	// nesting is honest: a comment that belongs to different content yields a clean 404
	// (no disclosure), the same way DeleteOwnComment hides ownership below.
	existing, err := h.commentService.GetComment(r.Context(), commentID)
	if err != nil {
		h.logger.Error(
			"agent delete comment bind failed: contentID=%d commentID=%d err=%v",
			contentID,
			commentID,
			err,
		)
		handleCommentError(w, err)
		return
	}
	if existing.ContentID != contentID {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Comment not found", nil)
		return
	}

	var delErr error
	if role == adminRoleName {
		delErr = h.commentService.DeleteComment(r.Context(), commentID)
	} else {
		delErr = h.commentService.DeleteOwnComment(r.Context(), commentID, principal)
	}
	if delErr != nil {
		h.logger.Error(
			"agent delete comment failed: contentID=%d commentID=%d userID=%d err=%v",
			contentID,
			commentID,
			userID,
			delErr,
		)
		handleCommentError(w, delErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateStatus handles PUT /api/v1/content/{id}/comments/{commentId}/status. It
// is admin-only (a non-admin API key gets 403 FORBIDDEN). The path content id is
// bound to the comment (a mismatch yields a clean 404), then it delegates to
// UpdateCommentStatus, which validates the status (a valid
// commentdomain.CommentStatus — pending/approved/rejected/spam; an unknown value
// returns ErrInvalidCommentStatus → 400 VALIDATION_ERROR). The updated comment
// is returned so the caller sees the new status.
func (h *CommentHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	role := authenticatedRole(r)
	if role != adminRoleName {
		httpx.Error(w, http.StatusForbidden, "FORBIDDEN", "comment moderation is admin-only", nil)
		return
	}

	contentID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || contentID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid content ID", nil)
		return
	}
	commentID, err := strconv.Atoi(r.PathValue("commentId"))
	if err != nil || commentID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid comment ID", nil)
		return
	}

	var req updateCommentStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}

	// Bind the path content id to the comment (see Delete): a comment that belongs to
	// other content is rejected with a clean 404 before the status is changed.
	existing, err := h.commentService.GetComment(r.Context(), commentID)
	if err != nil {
		h.logger.Error(
			"agent update comment status bind failed: contentID=%d commentID=%d err=%v",
			contentID,
			commentID,
			err,
		)
		handleCommentError(w, err)
		return
	}
	if existing.ContentID != contentID {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Comment not found", nil)
		return
	}

	status := commentdomain.CommentStatus(strings.TrimSpace(req.Status))
	if err := h.commentService.UpdateCommentStatus(r.Context(), commentID, status); err != nil {
		h.logger.Error(
			"agent update comment status failed: contentID=%d commentID=%d err=%v",
			contentID,
			commentID,
			err,
		)
		handleCommentError(w, err)
		return
	}

	comment, err := h.commentService.GetComment(r.Context(), commentID)
	if err != nil {
		h.logger.Error(
			"agent update comment status fetch failed: contentID=%d commentID=%d err=%v",
			contentID,
			commentID,
			err,
		)
		handleCommentError(w, err)
		return
	}

	httpx.Success(w, NewCommentResponse(comment))
}

// NewCommentHandler constructs a CommentHandler backed by the given comment
// service and content lookup. A nil logger degrades to a discard sink so the
// handler is safe to construct in any context.
func NewCommentHandler(
	commentService CommentService,
	content ContentLookup,
	logger kernel.Logger,
	opts ...CommentHandlerOption,
) *CommentHandler {
	if logger == nil {
		logger = discardLogger{}
	}
	h := &CommentHandler{
		commentService: commentService,
		content:        content,
		logger:         logger,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}
