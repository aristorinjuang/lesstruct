package modules

import (
	"context"
	"errors"

	contentservice "github.com/aristorinjuang/lesstruct/internal/domain/content"
	contentdomain "github.com/aristorinjuang/lesstruct/kernel/content"
	"github.com/aristorinjuang/lesstruct/kernel"
	commentapi "github.com/aristorinjuang/lesstruct/modules/comments/api"
	commentagent "github.com/aristorinjuang/lesstruct/modules/comments/api/agent"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
)

// CommentServiceAdapter combines the content service with the comments
// module service for the surfaces that need both: the browser and agent
// comment handlers and the contentpage assembler. Content lookups promote
// from the embedded content service; comment operations delegate to the
// comments service. It lives in core — the only layer that may import both
// sides — so routes and handlers stay on a single dependency.
type CommentServiceAdapter struct {
	*contentservice.Service
	Comments *commentdomain.Service
}

// SubmitComment validates the text, enforces the target's AllowComments
// gate, and persists the comment in the pending moderation status.
func (a *CommentServiceAdapter) SubmitComment(
	ctx context.Context,
	contentID int,
	principal kernel.Principal,
	req commentdomain.CreateCommentRequest,
) (*commentdomain.Comment, error) {
	return a.Comments.SubmitComment(ctx, contentID, principal, req)
}

// GetComment returns a single comment by ID.
func (a *CommentServiceAdapter) GetComment(ctx context.Context, commentID int) (*commentdomain.Comment, error) {
	return a.Comments.GetComment(ctx, commentID)
}

// GetCommentsForContent returns the approved comments visible on a content item.
func (a *CommentServiceAdapter) GetCommentsForContent(ctx context.Context, contentID int) ([]*commentdomain.Comment, error) {
	return a.Comments.GetCommentsForContent(ctx, contentID)
}

// GetCommentsForModeration returns every comment on a content item in any
// status for the moderation queue.
func (a *CommentServiceAdapter) GetCommentsForModeration(ctx context.Context, contentID int) ([]*commentdomain.Comment, error) {
	return a.Comments.GetCommentsForModeration(ctx, contentID)
}

// GetCommentsByUserID returns the given user's comments in newest-first order.
func (a *CommentServiceAdapter) GetCommentsByUserID(ctx context.Context, userID int) ([]*commentdomain.Comment, error) {
	return a.Comments.GetCommentsByUserID(ctx, userID)
}

// GetCommentsByStatus returns comments in the given moderation status.
func (a *CommentServiceAdapter) GetCommentsByStatus(ctx context.Context, status commentdomain.CommentStatus) ([]*commentdomain.Comment, error) {
	return a.Comments.GetCommentsByStatus(ctx, status)
}

// CountComments returns the total number of comments in the given scope.
func (a *CommentServiceAdapter) CountComments(ctx context.Context, userID int) (int, error) {
	return a.Comments.CountComments(ctx, userID)
}

// UpdateCommentStatus moves a comment to a validated moderation status.
func (a *CommentServiceAdapter) UpdateCommentStatus(ctx context.Context, commentID int, status commentdomain.CommentStatus) error {
	return a.Comments.UpdateCommentStatus(ctx, commentID, status)
}

// DeleteComment removes a comment by ID (admin path).
func (a *CommentServiceAdapter) DeleteComment(ctx context.Context, commentID int) error {
	return a.Comments.DeleteComment(ctx, commentID)
}

// DeleteOwnComment removes a comment owned by the caller's user ID.
func (a *CommentServiceAdapter) DeleteOwnComment(ctx context.Context, commentID int, principal kernel.Principal) error {
	return a.Comments.DeleteOwnComment(ctx, commentID, principal)
}

// NewAllowCommentsChecker adapts the content service to the comments
// module's SubmitComment gate. It lives in core — the only layer that may
// import both sides — until a later phase moves the seam. A missing item
// translates to the domain's ErrContentNotFound so the submit path maps it
// exactly as before; any other failure propagates for the 500 path.
func NewAllowCommentsChecker(content *contentservice.Service) commentdomain.AllowCommentsChecker {
	return func(ctx context.Context, contentID int) (bool, error) {
		item, err := content.GetByID(ctx, contentID)
		if err != nil {
			if errors.Is(err, contentdomain.ErrContentNotFound) {
				return false, commentdomain.ErrContentNotFound
			}
			return false, err
		}
		return item.AllowComments, nil
	}
}

// NewCommentServiceAdapter combines the content service with the comments
// module service for the surfaces that need both (see CommentServiceAdapter).
func NewCommentServiceAdapter(
	content *contentservice.Service,
	comments *commentdomain.Service,
) *CommentServiceAdapter {
	return &CommentServiceAdapter{
		Service:  content,
		Comments: comments,
	}
}

// NewBrowserContentResolver adapts the content service to the comments
// module's browser seam: slug in, comment-relevant facts out. A missing item
// translates to the seam's ErrContentNotFound; any other failure propagates
// for the handler's 500 path. It lives in core — the only layer that may
// import both sides.
func NewBrowserContentResolver(content *contentservice.Service) commentapi.ContentResolver {
	return func(ctx context.Context, slug, language string) (commentapi.ContentRef, error) {
		item, err := content.GetPublishedBySlug(ctx, slug, language)
		if err != nil {
			if errors.Is(err, contentdomain.ErrContentNotFound) {
				return commentapi.ContentRef{}, commentapi.ErrContentNotFound
			}
			return commentapi.ContentRef{}, err
		}
		return commentapi.ContentRef{
			ID:            item.ID,
			AllowComments: item.AllowComments,
		}, nil
	}
}

// NewAgentContentLookup adapts the content service to the comments module's
// agent seam: content ID in, visibility and gate facts out. A missing item
// translates to the seam's ErrContentNotFound; any other failure propagates
// for the handler's 500 path. It lives in core — the only layer that may
// import both sides.
func NewAgentContentLookup(content *contentservice.Service) commentagent.ContentLookup {
	return func(ctx context.Context, id int) (commentagent.ContentDetail, error) {
		item, err := content.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, contentdomain.ErrContentNotFound) {
				return commentagent.ContentDetail{}, commentagent.ErrContentNotFound
			}
			return commentagent.ContentDetail{}, err
		}
		return commentagent.ContentDetail{
			ID:            item.ID,
			UserID:        item.UserID,
			Published:     item.Status == contentdomain.StatusPublished,
			AllowComments: item.AllowComments,
		}, nil
	}
}
