package content

import (
	"context"
)

// ServiceInterface defines the interface for the content service
type ServiceInterface interface {
	SubmitComment(ctx context.Context, contentID int, userID int, req CreateCommentRequest) (*Comment, error)
	GetComment(ctx context.Context, commentID int) (*Comment, error)
	GetCommentsForContent(ctx context.Context, contentID int) ([]*Comment, error)
	GetCommentsForModeration(ctx context.Context, contentID int) ([]*Comment, error)
	GetCommentsByUserID(ctx context.Context, userID int) ([]*Comment, error)
	GetCommentsByStatus(ctx context.Context, status CommentStatus) ([]*Comment, error)
	UpdateCommentStatus(ctx context.Context, commentID int, status CommentStatus) error
	DeleteComment(ctx context.Context, commentID int) error
	DeleteOwnComment(ctx context.Context, commentID int, userID int) error
	GetPublishedBySlug(ctx context.Context, slug string, language string) (*Content, error)
	GetTranslations(ctx context.Context, translationGroupID int, excludeID int) ([]*Content, error)
}
