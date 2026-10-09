package comments

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	// ErrCommentNotFound is returned when comment cannot be found
	ErrCommentNotFound = errors.New("comment not found")
	// ErrContentNotFound is returned when the target content item cannot be
	// found. Core translates the content lookup failure to it so the module
	// never imports the content domain.
	ErrContentNotFound = errors.New("comment target content not found")
	// ErrInvalidCommentText is returned when comment text validation fails
	ErrInvalidCommentText = errors.New("comment text is required and must be between 1 and 2000 characters")
	// ErrInvalidCommentStatus is returned when comment status is not valid
	ErrInvalidCommentStatus = errors.New("comment status must be one of: pending, approved, rejected, spam")
	// ErrHTMLInPlainText is returned when plain text contains HTML tags
	ErrHTMLInPlainText = errors.New("field must not contain HTML tags")
	// ErrCommentsDisabled is returned when the target content does not
	// accept comments. It replaces the content package's ErrUnauthorized on
	// this path so the failure names the feature, not the content.
	ErrCommentsDisabled = errors.New("comments are not allowed on this content")
)

// CommentStatus represents the moderation status of a comment
type CommentStatus string

const (
	// CommentStatusPending represents a comment awaiting moderation
	CommentStatusPending CommentStatus = "pending"
	// CommentStatusApproved represents an approved comment
	CommentStatusApproved CommentStatus = "approved"
	// CommentStatusRejected represents a comment rejected by moderation
	CommentStatusRejected CommentStatus = "rejected"
	// CommentStatusSpam represents a comment marked as spam
	CommentStatusSpam CommentStatus = "spam"
)

// IsValid checks if the comment status is valid
func (s CommentStatus) IsValid() bool {
	return s == CommentStatusPending || s == CommentStatusApproved || s == CommentStatusRejected || s == CommentStatusSpam
}

// String returns the string representation of the comment status
func (s CommentStatus) String() string {
	return string(s)
}

// IsApproved returns true if the comment status is approved
func (s CommentStatus) IsApproved() bool {
	return s == CommentStatusApproved
}

// IsPending returns true if the comment status is pending
func (s CommentStatus) IsPending() bool {
	return s == CommentStatusPending
}

// IsRejected returns true if the comment status is rejected
func (s CommentStatus) IsRejected() bool {
	return s == CommentStatusRejected
}

// IsSpam returns true if the comment status is spam
func (s CommentStatus) IsSpam() bool {
	return s == CommentStatusSpam
}

// Comment represents a comment on a content item
type Comment struct {
	ID           int           `json:"id"`
	ContentID    int           `json:"contentId"`
	UserID       int           `json:"userId"`
	Comment      string        `json:"comment"`
	Status       CommentStatus `json:"status"`
	Author       string        `json:"author,omitempty"`
	Username     string        `json:"username,omitempty"`
	Role         string        `json:"role,omitempty"`
	ContentTitle string        `json:"contentTitle,omitempty"`
	ContentSlug  string        `json:"contentSlug,omitempty"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
}

// CreateCommentRequest represents a request to create a comment
type CreateCommentRequest struct {
	Comment string `json:"comment"`
}

// UpdateCommentStatusRequest represents a request to update comment status
type UpdateCommentStatusRequest struct {
	Status CommentStatus `json:"status"`
}

// htmlTagPattern matches the start of an HTML tag: "<" followed by a tag-name
// letter, a "/" for closing tags, or a "!" for comments/DOCTYPE. It mirrors
// the content sanitize package's pattern (the canonical definition) so plain
// text with ampersands, apostrophes, or math "<" stays valid here too.
var htmlTagPattern = regexp.MustCompile(`<[a-zA-Z!/]`)

// ValidateCommentText validates the comment text field
func ValidateCommentText(comment string) error {
	comment = strings.TrimSpace(comment)
	if comment == "" || utf8.RuneCountInString(comment) > 2000 {
		return ErrInvalidCommentText
	}
	if htmlTagPattern.MatchString(comment) {
		return ErrHTMLInPlainText
	}
	return nil
}

// ValidateCommentStatus validates the comment status
func ValidateCommentStatus(status CommentStatus) error {
	if !status.IsValid() {
		return ErrInvalidCommentStatus
	}
	return nil
}
