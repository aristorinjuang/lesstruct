package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/api/response"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

// commentRequest is the agent comment-create payload for
// POST /api/v1/content/{id}/comments. It carries just the comment text — the
// author is the API-key-owning user injected into context by the Bearer
// middleware, and the new comment always starts in the "pending" moderation
// status (the domain service's SubmitComment enforces both).
type commentRequest struct {
	Comment string `json:"comment"`
}

// updateCommentStatusRequest is the agent moderation payload for
// PUT /api/v1/content/{id}/comments/{commentId}/status. Status must be a valid
// contentdomain.CommentStatus (the service re-validates and rejects an unknown
// value with ErrInvalidCommentStatus → 400 VALIDATION_ERROR).
type updateCommentStatusRequest struct {
	Status string `json:"status"`
}

// CommentService is the narrow slice of the content domain service the agent
// comment handlers depend on. *contentdomain.Service satisfies it. Declaring it
// locally — instead of depending on the wider contentdomain.ServiceInterface —
// keeps the agent surface independent and makes the dependency explicit. It is
// exported so mockery can generate an exported constructor callable from the
// package's *_test files (CLAUDE.md mandates *_test packages, which cannot
// reach an unexported mock).
type CommentService interface {
	GetByID(ctx context.Context, id int) (*contentdomain.Content, error)
	SubmitComment(ctx context.Context, contentID int, userID int, req contentdomain.CreateCommentRequest) (*contentdomain.Comment, error)
	GetCommentsForModeration(ctx context.Context, contentID int) ([]*contentdomain.Comment, error)
	GetComment(ctx context.Context, commentID int) (*contentdomain.Comment, error)
	UpdateCommentStatus(ctx context.Context, commentID int, status contentdomain.CommentStatus) error
	DeleteComment(ctx context.Context, commentID int) error
	DeleteOwnComment(ctx context.Context, commentID int, userID int) error
}

// CommentProjection is the public, whitelisted view of a comment returned by the
// agent comment endpoints. It mirrors the browser comment handler's response
// shape (id, comment, author, username, role, status, createdAt) so the wire
// contract is consistent across the two realms, and excludes the numeric owner
// / content ids the raw contentdomain.Comment carries.
type CommentProjection struct {
	ID        int    `json:"id"`
	Comment   string `json:"comment"`
	Author    string `json:"author,omitempty"`
	Username  string `json:"username,omitempty"`
	Role      string `json:"role,omitempty"`
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// CommentResponse wraps a projected comment for the agent comment responses.
// Envelope body: {"data":{"comment":{...}}}.
type CommentResponse struct {
	Comment CommentProjection `json:"comment"`
}

// NewCommentResponse builds a CommentResponse projecting the given comment
// entity to its public fields. A nil entity yields an empty projection
// (defensive — the service contract returns a non-nil comment on success).
func NewCommentResponse(c *contentdomain.Comment) CommentResponse {
	if c == nil {
		return CommentResponse{}
	}
	return CommentResponse{
		Comment: CommentProjection{
			ID:        c.ID,
			Comment:   c.Comment,
			Author:    c.Author,
			Username:  c.Username,
			Role:      c.Role,
			Status:    string(c.Status),
			CreatedAt: c.CreatedAt,
		},
	}
}

// CommentHandler exposes the Bearer-authenticated agent comment endpoints. It
// reuses the existing content domain service's comment methods (which already
// validate comment text and enforce the AllowComments gate) — it never
// duplicates that logic. The comment surface is nested under the agent's
// content-keyed namespace (/api/v1/content/{id}/comments) so it is collision-free
// vs the browser realm's /api/v1/content_items/.../comments and /api/v1/comments
// routes, and consistent with the agent surface that keys everything by content id.
type CommentHandler struct {
	commentService CommentService
	logger         *util.Logger
}

// Create handles POST /api/v1/content/{id}/comments. It resolves the content by
// id (404 NOT_FOUND when missing, and 404 — never 403 — when the caller is
// neither owner nor Admin and the content is a draft, so existence is not
// disclosed), rejects commenting when the content has comments disabled (403
// FORBIDDEN), then delegates to SubmitComment which validates the text (1–2000
// chars, no HTML → ErrInvalidCommentText → 400 VALIDATION_ERROR) and persists
// the comment in the "pending" moderation status. Like the rest of the agent
// surface, success is 200 (the browser realm returns 201; the agent realm
// standardizes on 200 via response.Success).
func (h *CommentHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	role := authenticatedRole(r)

	contentID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || contentID <= 0 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid content ID", nil)
		return
	}

	var req commentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}

	content, err := h.commentService.GetByID(r.Context(), contentID)
	if err != nil {
		h.logger.Error("agent create comment resolve content failed: contentID=%d err=%v", contentID, err)
		handleError(w, err)
		return
	}

	// Visibility / no-enumeration: only the owner (or an Admin) may interact with a
	// draft; anyone may interact with published content. Mirrors the agent content
	// Get/Update/Delete visibility model.
	if content.Status != contentdomain.StatusPublished && content.UserID != userID && role != contentdomain.RoleAdmin {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "Content not found", nil)
		return
	}

	if !content.AllowComments {
		response.Error(w, http.StatusForbidden, "FORBIDDEN", "Comments are not allowed on this content", nil)
		return
	}

	comment, err := h.commentService.SubmitComment(
		r.Context(),
		contentID,
		userID,
		contentdomain.CreateCommentRequest{Comment: req.Comment},
	)
	if err != nil {
		h.logger.Error("agent create comment failed: contentID=%d userID=%d err=%v", contentID, userID, err)
		handleError(w, err)
		return
	}

	response.Success(w, NewCommentResponse(comment))
}

// List handles GET /api/v1/content/{id}/comments. It returns every comment on
// the content (any moderation status — the management view), scoped by the same
// visibility model as Create (published, or owned by the caller, or Admin; else
// 404). The data array is always present, even when empty.
func (h *CommentHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	role := authenticatedRole(r)

	contentID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || contentID <= 0 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid content ID", nil)
		return
	}

	content, err := h.commentService.GetByID(r.Context(), contentID)
	if err != nil {
		h.logger.Error("agent list comments resolve content failed: contentID=%d err=%v", contentID, err)
		handleError(w, err)
		return
	}

	if content.Status != contentdomain.StatusPublished && content.UserID != userID && role != contentdomain.RoleAdmin {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "Content not found", nil)
		return
	}

	comments, err := h.commentService.GetCommentsForModeration(r.Context(), contentID)
	if err != nil {
		h.logger.Error("agent list comments failed: contentID=%d err=%v", contentID, err)
		handleError(w, err)
		return
	}

	projections := make([]CommentProjection, 0, len(comments))
	for _, c := range comments {
		projections = append(projections, NewCommentResponse(c).Comment)
	}

	response.SuccessList(w, projections, nil)
}

// Delete handles DELETE /api/v1/content/{id}/comments/{commentId}. An Admin may
// delete any comment (DeleteComment); any other caller may delete only their own
// (DeleteOwnComment, which returns ErrCommentNotFound — a clean 404 with no
// ownership disclosure — when the comment is missing or belongs to someone
// else). The content id is path context (the route is nested to avoid colliding
// with the browser realm's /api/v1/comments/{id}); the delete operates by
// comment id, matching the domain's comment-keyed operations and the browser
// moderation surface. Success is 204 No Content.
func (h *CommentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	role := authenticatedRole(r)

	contentID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || contentID <= 0 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid content ID", nil)
		return
	}
	commentID, err := strconv.Atoi(r.PathValue("commentId"))
	if err != nil || commentID <= 0 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid comment ID", nil)
		return
	}

	var delErr error
	if role == contentdomain.RoleAdmin {
		delErr = h.commentService.DeleteComment(r.Context(), commentID)
	} else {
		delErr = h.commentService.DeleteOwnComment(r.Context(), commentID, userID)
	}
	if delErr != nil {
		h.logger.Error(
			"agent delete comment failed: contentID=%d commentID=%d userID=%d err=%v",
			contentID,
			commentID,
			userID,
			delErr,
		)
		handleError(w, delErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateStatus handles PUT /api/v1/content/{id}/comments/{commentId}/status. It
// is admin-only (a non-admin API key gets 403 FORBIDDEN) and delegates to
// UpdateCommentStatus, which validates the status (a valid
// contentdomain.CommentStatus — pending/approved/rejected/spam; an unknown value
// returns ErrInvalidCommentStatus → 400 VALIDATION_ERROR). The updated comment
// is returned so the caller sees the new status. As with Delete, the content id
// is path context and the update operates by comment id.
func (h *CommentHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	role := authenticatedRole(r)
	if role != contentdomain.RoleAdmin {
		response.Error(w, http.StatusForbidden, "FORBIDDEN", "comment moderation is admin-only", nil)
		return
	}

	contentID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || contentID <= 0 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid content ID", nil)
		return
	}
	commentID, err := strconv.Atoi(r.PathValue("commentId"))
	if err != nil || commentID <= 0 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid comment ID", nil)
		return
	}

	var req updateCommentStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}

	status := contentdomain.CommentStatus(strings.TrimSpace(req.Status))
	if err := h.commentService.UpdateCommentStatus(r.Context(), commentID, status); err != nil {
		h.logger.Error(
			"agent update comment status failed: contentID=%d commentID=%d err=%v",
			contentID,
			commentID,
			err,
		)
		handleError(w, err)
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
		handleError(w, err)
		return
	}

	response.Success(w, NewCommentResponse(comment))
}

// NewCommentHandler constructs a CommentHandler backed by the given content
// service. A nil logger degrades to a discard sink so the handler is safe to
// construct in any context (mirrors NewContentHandler/NewMediaHandler).
func NewCommentHandler(s CommentService, logger *util.Logger) *CommentHandler {
	if logger == nil {
		logger = util.NewLogger(io.Discard)
	}
	return &CommentHandler{
		commentService: s,
		logger:         logger,
	}
}
