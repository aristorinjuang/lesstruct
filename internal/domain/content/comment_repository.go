package content

import "context"

// CommentRepository defines the interface for comment repository operations
type CommentRepository interface {
	Create(ctx context.Context, comment *Comment) error
	GetByContentID(ctx context.Context, contentID int) ([]*Comment, error)
	GetByContentIDForModeration(ctx context.Context, contentID int) ([]*Comment, error)
	GetByID(ctx context.Context, id int) (*Comment, error)
	GetByUserID(ctx context.Context, userID int) ([]*Comment, error)
	GetByStatus(ctx context.Context, status CommentStatus) ([]*Comment, error)
	// Count returns the total number of comments. userID <= 0 counts ALL comments
	// (admin global scope); otherwise it counts only the given user's comments,
	// mirroring GetByUserID's WHERE clause so a list's total matches its rows.
	Count(ctx context.Context, userID int) (int, error)
	UpdateStatus(ctx context.Context, id int, status CommentStatus) error
	Delete(ctx context.Context, id int) error
	DeleteByUserID(ctx context.Context, userID int) error
}
