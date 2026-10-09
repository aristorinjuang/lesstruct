package comments_test

import (
	"strings"
	"testing"

	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	"github.com/stretchr/testify/assert"
)

func TestValidateCommentText(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		wantErr error
	}{
		{
			name:    "valid comment",
			comment: "This is a valid comment",
			wantErr: nil,
		},
		{
			name:    "valid comment with whitespace",
			comment: "  This is a valid comment  ",
			wantErr: nil,
		},
		{
			name:    "invalid empty comment",
			comment: "",
			wantErr: commentdomain.ErrInvalidCommentText,
		},
		{
			name:    "invalid whitespace only comment",
			comment: "   ",
			wantErr: commentdomain.ErrInvalidCommentText,
		},
		{
			name:    "valid 2000 character comment",
			comment: strings.Repeat("A", 2000),
			wantErr: nil,
		},
		{
			name:    "invalid 2001 character comment",
			comment: strings.Repeat("A", 2001),
			wantErr: commentdomain.ErrInvalidCommentText,
		},
		{
			name:    "invalid comment with script tag",
			comment: `<script>alert(1)</script>`,
			wantErr: commentdomain.ErrHTMLInPlainText,
		},
		{
			name:    "invalid comment with img tag",
			comment: `<img src="x" onerror="alert(1)">`,
			wantErr: commentdomain.ErrHTMLInPlainText,
		},
		{
			name:    "invalid comment with bold tag",
			comment: "<b>bold</b>",
			wantErr: commentdomain.ErrHTMLInPlainText,
		},
		{
			name:    "invalid comment with anchor tag",
			comment: `<a href="https://example.com">link</a>`,
			wantErr: commentdomain.ErrHTMLInPlainText,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := commentdomain.ValidateCommentText(tt.comment); err != tt.wantErr {
				t.Errorf("ValidateCommentText() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCommentStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  commentdomain.CommentStatus
		wantErr error
	}{
		{
			name:    "valid pending status",
			status:  commentdomain.CommentStatusPending,
			wantErr: nil,
		},
		{
			name:    "valid approved status",
			status:  commentdomain.CommentStatusApproved,
			wantErr: nil,
		},
		{
			name:    "valid rejected status",
			status:  commentdomain.CommentStatusRejected,
			wantErr: nil,
		},
		{
			name:    "valid spam status",
			status:  commentdomain.CommentStatusSpam,
			wantErr: nil,
		},
		{
			name:    "invalid unknown status",
			status:  commentdomain.CommentStatus("bogus"),
			wantErr: commentdomain.ErrInvalidCommentStatus,
		},
		{
			name:    "invalid empty status",
			status:  commentdomain.CommentStatus(""),
			wantErr: commentdomain.ErrInvalidCommentStatus,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := commentdomain.ValidateCommentStatus(tt.status); err != tt.wantErr {
				t.Errorf("ValidateCommentStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommentStatusPredicates(t *testing.T) {
	tests := []struct {
		name     string
		status   commentdomain.CommentStatus
		valid    bool
		approved bool
		pending  bool
		rejected bool
		spam     bool
	}{
		{
			name:    "pending",
			status:  commentdomain.CommentStatusPending,
			valid:   true,
			pending: true,
		},
		{
			name:     "approved",
			status:   commentdomain.CommentStatusApproved,
			valid:    true,
			approved: true,
		},
		{
			name:     "rejected",
			status:   commentdomain.CommentStatusRejected,
			valid:    true,
			rejected: true,
		},
		{
			name:   "spam",
			status: commentdomain.CommentStatusSpam,
			valid:  true,
			spam:   true,
		},
		{
			name:   "unknown",
			status: commentdomain.CommentStatus("bogus"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.status.IsValid())
			assert.Equal(t, tt.approved, tt.status.IsApproved())
			assert.Equal(t, tt.pending, tt.status.IsPending())
			assert.Equal(t, tt.rejected, tt.status.IsRejected())
			assert.Equal(t, tt.spam, tt.status.IsSpam())
			assert.Equal(t, string(tt.status), tt.status.String())
		})
	}
}
