package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/httpx"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	"github.com/go-chi/chi/v5"
)

// adminRoleName is the reserved administrator role. It matches the users
// module's reserved name; the two reconcile when that module lands.
const adminRoleName = "Admin"

// ErrContentNotFound is returned by the ContentResolver when no published
// content matches the slug. Core translates the content lookup failure to it
// so this package never imports the content domain.
var ErrContentNotFound = errors.New("comments: content not found")

func primaryRole(principal kernel.Principal) string {
	if len(principal.Roles) == 0 {
		return ""
	}
	return principal.Roles[0]
}

func principalUserID(principal kernel.Principal) (int, bool) {
	userID, err := strconv.Atoi(principal.ID)
	if err != nil {
		return 0, false
	}
	return userID, true
}

func handleCommentError(w http.ResponseWriter, err error) {
	statusCode := http.StatusInternalServerError
	code := "internal_error"
	message := "An internal error occurred"

	switch {
	case errors.Is(err, kernel.ErrInvalidPrincipalID):
		statusCode = http.StatusUnauthorized
		code = "unauthorized"
		message = "Invalid user ID"
	case errors.Is(err, commentdomain.ErrCommentNotFound):
		statusCode = http.StatusNotFound
		code = "comment_not_found"
		message = "Comment not found"
	case errors.Is(err, commentdomain.ErrInvalidCommentText):
		statusCode = http.StatusBadRequest
		code = "invalid_comment"
		message = err.Error()
	case errors.Is(err, commentdomain.ErrInvalidCommentStatus):
		statusCode = http.StatusBadRequest
		code = "invalid_status"
		message = err.Error()
	case errors.Is(err, ErrContentNotFound),
		errors.Is(err, commentdomain.ErrContentNotFound):
		statusCode = http.StatusNotFound
		code = "content_not_found"
		message = "Content not found"
	case errors.Is(err, commentdomain.ErrCommentsDisabled):
		statusCode = http.StatusForbidden
		code = "forbidden"
		message = "Comments are not allowed on this content"
	}

	sendErrorResponse(w, statusCode, code, message, nil)
}

func resolveContent(w http.ResponseWriter, r *http.Request, resolve ContentResolver, slug string) (ContentRef, bool) {
	ref, err := resolve(r.Context(), slug, "en")
	if err != nil {
		if errors.Is(err, ErrContentNotFound) {
			sendErrorResponse(w, http.StatusNotFound, "content_not_found", "Content not found", nil)
			return ContentRef{}, false
		}
		handleCommentError(w, err)
		return ContentRef{}, false
	}
	return ref, true
}

// CreateCommentRequest is the browser comment-create payload.
type CreateCommentRequest struct {
	Comment string `json:"comment"`
}

// UpdateCommentStatusRequest is the browser moderation payload.
type UpdateCommentStatusRequest struct {
	Status string `json:"status"`
}

// CommentResponse is the browser-facing representation of a comment.
type CommentResponse struct {
	ID        int       `json:"id"`
	Comment   string    `json:"comment"`
	Author    string    `json:"author,omitempty"`
	Username  string    `json:"username,omitempty"`
	Role      string    `json:"role,omitempty"`
	Status    string    `json:"status,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// PendingCommentResponse is the admin-facing representation of a comment awaiting
// moderation, enriched with the content item it belongs to.
type PendingCommentResponse struct {
	ID           int       `json:"id"`
	ContentID    int       `json:"contentId"`
	ContentTitle string    `json:"contentTitle,omitempty"`
	ContentSlug  string    `json:"contentSlug,omitempty"`
	Comment      string    `json:"comment"`
	Author       string    `json:"author,omitempty"`
	Username     string    `json:"username,omitempty"`
	Status       string    `json:"status,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// RoleCapabilities is the narrow role-registry seam this handler depends on.
// Core satisfies it with an adapter over *role.Service (which cannot be
// imported here); the users module implements it natively when it lands.
type RoleCapabilities interface {
	CanComment(name string) bool
}

// ContentRef is the comment-relevant projection of a published content item:
// its ID for scoping and its AllowComments flag for gating.
type ContentRef struct {
	ID            int
	AllowComments bool
}

// ContentResolver resolves a published slug to its comment-relevant facts.
// Core satisfies it with a closure over the content service (which cannot be
// imported here); tests stub it with a func literal.
type ContentResolver func(ctx context.Context, slug, language string) (ContentRef, error)

// CommentService is the narrow slice of comment operations this handler
// depends on. *commentdomain.Service satisfies it.
type CommentService interface {
	SubmitComment(ctx context.Context, contentID int, principal kernel.Principal, req commentdomain.CreateCommentRequest) (*commentdomain.Comment, error)
	GetComment(ctx context.Context, commentID int) (*commentdomain.Comment, error)
	GetCommentsForContent(ctx context.Context, contentID int) ([]*commentdomain.Comment, error)
	GetCommentsForModeration(ctx context.Context, contentID int) ([]*commentdomain.Comment, error)
	GetCommentsByUserID(ctx context.Context, userID int) ([]*commentdomain.Comment, error)
	GetCommentsByStatus(ctx context.Context, status commentdomain.CommentStatus) ([]*commentdomain.Comment, error)
	CountComments(ctx context.Context, userID int) (int, error)
	UpdateCommentStatus(ctx context.Context, commentID int, status commentdomain.CommentStatus) error
	DeleteComment(ctx context.Context, commentID int) error
	DeleteOwnComment(ctx context.Context, commentID int, principal kernel.Principal) error
}

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

// CommentHandler serves the browser-realm comment surface.
type CommentHandler struct {
	commentService CommentService
	content        ContentResolver
	roleService    RoleCapabilities
}

// commentAllowed reports whether the caller's role may post comments. Without a
// role service every authenticated user may, matching the legacy behavior.
func (h *CommentHandler) commentAllowed(r *http.Request) bool {
	if h.roleService == nil {
		return true
	}
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		return false
	}
	return slices.ContainsFunc(principal.Roles, func(role string) bool {
		return h.roleService.CanComment(role)
	})
}

func (h *CommentHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	if slug == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_slug", "Slug is required", nil)
		return
	}

	content, ok := resolveContent(w, r, h.content, slug)
	if !ok {
		return
	}

	if !content.AllowComments {
		sendSuccessResponse(w, http.StatusOK, []CommentResponse{})
		return
	}

	comments, err := h.commentService.GetCommentsForContent(r.Context(), content.ID)
	if err != nil {
		handleCommentError(w, err)
		return
	}

	response := make([]CommentResponse, 0, len(comments))
	for _, comment := range comments {
		response = append(response, CommentResponse{
			ID:        comment.ID,
			Comment:   comment.Comment,
			Author:    comment.Author,
			Username:  comment.Username,
			Role:      comment.Role,
			CreatedAt: comment.CreatedAt,
		})
	}

	sendSuccessResponse(w, http.StatusOK, response)
}

func (h *CommentHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	if slug == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_slug", "Slug is required", nil)
		return
	}

	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
		return
	}

	if !h.commentAllowed(r) {
		sendErrorResponse(w, http.StatusForbidden, "forbidden", "Your role cannot post comments", nil)
		return
	}

	var req CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	content, ok := resolveContent(w, r, h.content, slug)
	if !ok {
		return
	}

	if !content.AllowComments {
		sendErrorResponse(w, http.StatusForbidden, "forbidden", "Comments are not allowed on this content", nil)
		return
	}

	createReq := commentdomain.CreateCommentRequest{
		Comment: req.Comment,
	}

	comment, err := h.commentService.SubmitComment(r.Context(), content.ID, principal, createReq)
	if err != nil {
		handleCommentError(w, err)
		return
	}

	response := CommentResponse{
		ID:        comment.ID,
		Comment:   comment.Comment,
		CreatedAt: comment.CreatedAt,
	}

	sendSuccessResponse(w, http.StatusCreated, response)
}

func (h *CommentHandler) GetCommentsForModeration(w http.ResponseWriter, r *http.Request) {
	contentIDStr := chi.URLParam(r, "id")

	contentID, err := strconv.Atoi(contentIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_id", "Invalid content ID", nil)
		return
	}

	comments, err := h.commentService.GetCommentsForModeration(r.Context(), contentID)
	if err != nil {
		handleCommentError(w, err)
		return
	}

	response := make([]map[string]any, 0, len(comments))
	for _, comment := range comments {
		response = append(response, map[string]any{
			"id":        comment.ID,
			"comment":   comment.Comment,
			"author":    comment.Author,
			"username":  comment.Username,
			"status":    comment.Status,
			"createdAt": comment.CreatedAt,
		})
	}

	sendSuccessResponse(w, http.StatusOK, response)
}

func (h *CommentHandler) GetPendingComments(w http.ResponseWriter, r *http.Request) {
	comments, err := h.commentService.GetCommentsByStatus(r.Context(), commentdomain.CommentStatusPending)
	if err != nil {
		handleCommentError(w, err)
		return
	}

	response := make([]PendingCommentResponse, 0, len(comments))
	for _, comment := range comments {
		response = append(response, PendingCommentResponse{
			ID:           comment.ID,
			ContentID:    comment.ContentID,
			ContentTitle: comment.ContentTitle,
			ContentSlug:  comment.ContentSlug,
			Comment:      comment.Comment,
			Author:       comment.Author,
			Username:     comment.Username,
			Status:       string(comment.Status),
			CreatedAt:    comment.CreatedAt,
		})
	}

	sendSuccessResponse(w, http.StatusOK, response)
}

func (h *CommentHandler) UpdateCommentStatus(w http.ResponseWriter, r *http.Request) {
	commentIDStr := chi.URLParam(r, "id")

	commentID, err := strconv.Atoi(commentIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_id", "Invalid comment ID", nil)
		return
	}

	var req UpdateCommentStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	status := commentdomain.CommentStatus(req.Status)
	if err := h.commentService.UpdateCommentStatus(r.Context(), commentID, status); err != nil {
		handleCommentError(w, err)
		return
	}

	comment, err := h.commentService.GetComment(r.Context(), commentID)
	if err != nil {
		handleCommentError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusOK, CommentResponse{
		ID:        comment.ID,
		Comment:   comment.Comment,
		Author:    comment.Author,
		Username:  comment.Username,
		Status:    string(comment.Status),
		CreatedAt: comment.CreatedAt,
	})
}

func (h *CommentHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	commentIDStr := chi.URLParam(r, "id")

	commentID, err := strconv.Atoi(commentIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_id", "Invalid comment ID", nil)
		return
	}

	if err := h.commentService.DeleteComment(r.Context(), commentID); err != nil {
		handleCommentError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CommentHandler) GetMyComments(w http.ResponseWriter, r *http.Request) {
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
		return
	}

	userID, ok := principalUserID(principal)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Invalid user ID", nil)
		return
	}

	comments, err := h.commentService.GetCommentsByUserID(r.Context(), userID)
	if err != nil {
		handleCommentError(w, err)
		return
	}

	countUserID := userID
	if primaryRole(principal) == adminRoleName {
		countUserID = 0
	}

	total, err := h.commentService.CountComments(r.Context(), countUserID)
	if err != nil {
		handleCommentError(w, err)
		return
	}

	commentList := make([]map[string]any, 0, len(comments))
	for _, comment := range comments {
		commentList = append(commentList, map[string]any{
			"id":        comment.ID,
			"comment":   comment.Comment,
			"status":    comment.Status,
			"createdAt": comment.CreatedAt,
		})
	}

	httpx.SuccessList(
		w,
		commentList,
		httpx.ListMeta{
			Pagination: httpx.Pagination{Total: &total, HasMore: false},
		},
	)
}

func (h *CommentHandler) DeleteOwnComment(w http.ResponseWriter, r *http.Request) {
	commentIDStr := chi.URLParam(r, "id")

	commentID, err := strconv.Atoi(commentIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_id", "Invalid comment ID", nil)
		return
	}

	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
		return
	}

	if err := h.commentService.DeleteOwnComment(r.Context(), commentID, principal); err != nil {
		handleCommentError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// NewCommentHandler constructs a CommentHandler backed by the given comment
// service and published-content resolver.
func NewCommentHandler(
	commentService CommentService,
	content ContentResolver,
	opts ...CommentHandlerOption,
) *CommentHandler {
	h := &CommentHandler{
		commentService: commentService,
		content:        content,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}
