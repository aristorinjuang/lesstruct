package agent

import (
	"time"

	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
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
// commentdomain.CommentStatus (the service re-validates and rejects an unknown
// value with ErrInvalidCommentStatus → 400 VALIDATION_ERROR).
type updateCommentStatusRequest struct {
	Status string `json:"status"`
}

// CommentProjection is the public, whitelisted view of a comment returned by the
// agent comment endpoints. It mirrors the browser comment handler's response
// shape (id, comment, author, username, role, status, createdAt) so the wire
// contract is consistent across the two realms, and excludes the numeric owner
// / content ids the raw commentdomain.Comment carries.
type CommentProjection struct {
	ID        int       `json:"id"`
	Comment   string    `json:"comment"`
	Author    string    `json:"author,omitempty"`
	Username  string    `json:"username,omitempty"`
	Role      string    `json:"role,omitempty"`
	Status    string    `json:"status,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// CommentResponse wraps a projected comment for the agent comment responses.
// Envelope body: {"data":{"comment":{...}}}.
type CommentResponse struct {
	Comment CommentProjection `json:"comment"`
}

// NewCommentResponse builds a CommentResponse projecting the given comment
// entity to its public fields. A nil entity yields an empty projection
// (defensive — the service contract returns a non-nil comment on success).
func NewCommentResponse(c *commentdomain.Comment) CommentResponse {
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
