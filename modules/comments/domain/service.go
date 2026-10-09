package comments

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aristorinjuang/lesstruct/kernel"
)

// principalUserID parses the principal's decimal ID for repository calls.
func principalUserID(principal kernel.Principal) (int, error) {
	id, err := strconv.Atoi(principal.ID)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", kernel.ErrInvalidPrincipalID, principal.ID)
	}
	return id, nil
}

// AllowCommentsChecker reports whether a content item accepts comments. Core
// satisfies it with a closure over the content service (which the module
// cannot import); the error chain preserves the content lookup failure so
// callers map a missing item exactly as before.
type AllowCommentsChecker func(ctx context.Context, contentID int) (bool, error)

// Option configures a Service at construction time.
type Option func(*Service)

// WithAllowCommentsChecker attaches the content gate for SubmitComment.
// Without it submissions fail closed — the gate is required, never optional.
func WithAllowCommentsChecker(checker AllowCommentsChecker) Option {
	return func(s *Service) {
		s.allowComments = checker
	}
}

// Service handles comment business logic
type Service struct {
	repo          CommentRepository
	allowComments AllowCommentsChecker
}

// SubmitComment validates the text, enforces the target's AllowComments gate,
// and persists the comment in the pending moderation status.
func (s *Service) SubmitComment(ctx context.Context, contentID int, principal kernel.Principal, req CreateCommentRequest) (*Comment, error) {
	userID, err := principalUserID(principal)
	if err != nil {
		return nil, err
	}
	if err := ValidateCommentText(req.Comment); err != nil {
		return nil, fmt.Errorf("comment validation failed: %w", err)
	}

	allow, err := s.allowComments(ctx, contentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get content: %w", err)
	}

	if !allow {
		return nil, ErrCommentsDisabled
	}

	comment := &Comment{
		ContentID: contentID,
		UserID:    userID,
		Comment:   strings.TrimSpace(req.Comment),
		Status:    CommentStatusPending,
	}

	if err := s.repo.Create(ctx, comment); err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	return comment, nil
}

// GetCommentsByUserID returns the given user's comments in newest-first order.
func (s *Service) GetCommentsByUserID(ctx context.Context, userID int) ([]*Comment, error) {
	comments, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments by user: %w", err)
	}

	return comments, nil
}

// CountComments returns the total number of comments. userID <= 0 counts ALL comments
// (admin global scope); otherwise it counts only the given user's comments, mirroring
// GetCommentsByUserID's scope so a list's total matches its rows.
func (s *Service) CountComments(ctx context.Context, userID int) (int, error) {
	total, err := s.repo.Count(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to count comments: %w", err)
	}
	return total, nil
}

// GetCommentsForContent returns the approved comments visible on a content item.
func (s *Service) GetCommentsForContent(ctx context.Context, contentID int) ([]*Comment, error) {
	comments, err := s.repo.GetByContentID(ctx, contentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments: %w", err)
	}

	return comments, nil
}

// GetCommentsForModeration returns every comment on a content item in any
// status for the moderation queue.
func (s *Service) GetCommentsForModeration(ctx context.Context, contentID int) ([]*Comment, error) {
	comments, err := s.repo.GetByContentIDForModeration(ctx, contentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments for moderation: %w", err)
	}

	return comments, nil
}

// GetCommentsByStatus returns comments in the given moderation status with
// their content context for the pending queue.
func (s *Service) GetCommentsByStatus(ctx context.Context, status CommentStatus) ([]*Comment, error) {
	comments, err := s.repo.GetByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments by status: %w", err)
	}

	return comments, nil
}

// ApproveComment moves a comment to the approved status.
func (s *Service) ApproveComment(ctx context.Context, commentID int) error {
	if err := s.repo.UpdateStatus(ctx, commentID, CommentStatusApproved); err != nil {
		return fmt.Errorf("failed to approve comment: %w", err)
	}

	return nil
}

// RejectComment moves a comment to the rejected status.
func (s *Service) RejectComment(ctx context.Context, commentID int) error {
	if err := s.repo.UpdateStatus(ctx, commentID, CommentStatusRejected); err != nil {
		return fmt.Errorf("failed to reject comment: %w", err)
	}

	return nil
}

// MarkAsSpam moves a comment to the spam status.
func (s *Service) MarkAsSpam(ctx context.Context, commentID int) error {
	if err := s.repo.UpdateStatus(ctx, commentID, CommentStatusSpam); err != nil {
		return fmt.Errorf("failed to mark comment as spam: %w", err)
	}

	return nil
}

// DeleteComment removes a comment by ID (admin path).
func (s *Service) DeleteComment(ctx context.Context, commentID int) error {
	if err := s.repo.Delete(ctx, commentID); err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	return nil
}

// DeleteOwnComment removes a comment owned by the caller's user ID. A missing
// comment or one owned by someone else both yield ErrCommentNotFound so
// ownership is never disclosed.
func (s *Service) DeleteOwnComment(ctx context.Context, commentID int, principal kernel.Principal) error {
	userID, err := principalUserID(principal)
	if err != nil {
		return err
	}
	comment, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return ErrCommentNotFound
	}

	if comment.UserID != userID {
		return ErrCommentNotFound
	}

	if err := s.repo.Delete(ctx, commentID); err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	return nil
}

// DeleteCommentsByUserID removes every comment owned by the given user.
func (s *Service) DeleteCommentsByUserID(ctx context.Context, userID int) error {
	if err := s.repo.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete comments by user: %w", err)
	}

	return nil
}

// GetComment returns a single comment by ID.
func (s *Service) GetComment(ctx context.Context, commentID int) (*Comment, error) {
	comment, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comment: %w", err)
	}

	return comment, nil
}

// UpdateCommentStatus moves a comment to a validated moderation status.
func (s *Service) UpdateCommentStatus(ctx context.Context, commentID int, status CommentStatus) error {
	if err := ValidateCommentStatus(status); err != nil {
		return fmt.Errorf("invalid comment status: %w", err)
	}

	if err := s.repo.UpdateStatus(ctx, commentID, status); err != nil {
		return fmt.Errorf("failed to update comment status: %w", err)
	}

	return nil
}

// NewService creates a comment Service on the given repository. The
// AllowComments gate defaults to closed; the composition root attaches the
// content-backed checker via WithAllowCommentsChecker.
func NewService(
	repo CommentRepository,
	opts ...Option,
) *Service {
	s := &Service{
		repo: repo,
		allowComments: func(context.Context, int) (bool, error) {
			return false, nil
		},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
