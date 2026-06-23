package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/go-chi/chi/v5"
)

type CommentHandler struct {
	contentService contentdomain.ServiceInterface
}

type CreateCommentRequest struct {
	Comment string `json:"comment"`
}

type UpdateCommentStatusRequest struct {
	Status string `json:"status"`
}

type CommentResponse struct {
	ID        int    `json:"id"`
	Comment   string `json:"comment"`
	Author    string `json:"author,omitempty"`
	Username  string `json:"username,omitempty"`
	Role      string `json:"role,omitempty"`
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// PendingCommentResponse is the admin-facing representation of a comment awaiting
// moderation, enriched with the content item it belongs to.
type PendingCommentResponse struct {
	ID           int    `json:"id"`
	ContentID    int    `json:"contentId"`
	ContentTitle string `json:"contentTitle,omitempty"`
	ContentSlug  string `json:"contentSlug,omitempty"`
	Comment      string `json:"comment"`
	Author       string `json:"author,omitempty"`
	Username     string `json:"username,omitempty"`
	Status       string `json:"status,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

func handleCommentError(w http.ResponseWriter, err error) {
	statusCode := http.StatusInternalServerError
	code := "internal_error"
	message := "An internal error occurred"

	switch {
	case errors.Is(err, contentdomain.ErrCommentNotFound):
		statusCode = http.StatusNotFound
		code = "comment_not_found"
		message = "Comment not found"
	case errors.Is(err, contentdomain.ErrInvalidCommentText):
		statusCode = http.StatusBadRequest
		code = "invalid_comment"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrInvalidCommentStatus):
		statusCode = http.StatusBadRequest
		code = "invalid_status"
		message = err.Error()
	case errors.Is(err, contentdomain.ErrContentNotFound):
		statusCode = http.StatusNotFound
		code = "content_not_found"
		message = "Content not found"
	case errors.Is(err, contentdomain.ErrUnauthorized):
		statusCode = http.StatusForbidden
		code = "forbidden"
		message = "Comments are not allowed on this content"
	}

	sendErrorResponse(w, statusCode, code, message, nil)
}

func (h *CommentHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	if slug == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_slug", "Slug is required", nil)
		return
	}

	content, err := h.contentService.GetPublishedBySlug(r.Context(), slug, "en")
	if err != nil {
		if errors.Is(err, contentdomain.ErrContentNotFound) {
			sendErrorResponse(w, http.StatusNotFound, "content_not_found", "Content not found", nil)
			return
		}
		handleCommentError(w, err)
		return
	}

	if !content.AllowComments {
		sendSuccessResponse(w, http.StatusOK, []CommentResponse{})
		return
	}

	comments, err := h.contentService.GetCommentsForContent(r.Context(), content.ID)
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

	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Invalid user ID", nil)
		return
	}

	var req CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	content, err := h.contentService.GetPublishedBySlug(r.Context(), slug, "en")
	if err != nil {
		if errors.Is(err, contentdomain.ErrContentNotFound) {
			sendErrorResponse(w, http.StatusNotFound, "content_not_found", "Content not found", nil)
			return
		}
		handleCommentError(w, err)
		return
	}

	if !content.AllowComments {
		sendErrorResponse(w, http.StatusForbidden, "forbidden", "Comments are not allowed on this content", nil)
		return
	}

	createReq := contentdomain.CreateCommentRequest{
		Comment: req.Comment,
	}

	comment, err := h.contentService.SubmitComment(r.Context(), content.ID, userID, createReq)
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

	comments, err := h.contentService.GetCommentsForModeration(r.Context(), contentID)
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
	comments, err := h.contentService.GetCommentsByStatus(r.Context(), contentdomain.CommentStatusPending)
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

	status := contentdomain.CommentStatus(req.Status)
	if err := h.contentService.UpdateCommentStatus(r.Context(), commentID, status); err != nil {
		handleCommentError(w, err)
		return
	}

	comment, err := h.contentService.GetComment(r.Context(), commentID)
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

	if err := h.contentService.DeleteComment(r.Context(), commentID); err != nil {
		handleCommentError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CommentHandler) GetMyComments(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Invalid user ID", nil)
		return
	}

	comments, err := h.contentService.GetCommentsByUserID(r.Context(), userID)
	if err != nil {
		handleCommentError(w, err)
		return
	}

	response := make([]map[string]any, 0, len(comments))
	for _, comment := range comments {
		response = append(response, map[string]any{
			"id":        comment.ID,
			"comment":   comment.Comment,
			"status":    comment.Status,
			"createdAt": comment.CreatedAt,
		})
	}

	sendSuccessResponse(w, http.StatusOK, response)
}

func (h *CommentHandler) DeleteOwnComment(w http.ResponseWriter, r *http.Request) {
	commentIDStr := chi.URLParam(r, "id")

	commentID, err := strconv.Atoi(commentIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_id", "Invalid comment ID", nil)
		return
	}

	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Invalid user ID", nil)
		return
	}

	if err := h.contentService.DeleteOwnComment(r.Context(), commentID, userID); err != nil {
		handleCommentError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func NewCommentHandler(contentService contentdomain.ServiceInterface) *CommentHandler {
	return &CommentHandler{
		contentService: contentService,
	}
}
