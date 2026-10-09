package comments_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	"github.com/aristorinjuang/lesstruct/modules/comments/domain/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testPrincipal(userID int) kernel.Principal {
	return kernel.Principal{
		ID:       strconv.Itoa(userID),
		Username: "testuser",
	}
}

func allowComments(allow bool, err error) commentdomain.AllowCommentsChecker {
	return func(context.Context, int) (bool, error) {
		return allow, err
	}
}

func TestService_SubmitComment(t *testing.T) {
	tests := []struct {
		name      string
		contentID int
		text      string
		allow     bool
		allowErr  error
		setupMock func(*mocks.MockCommentRepository)
		wantErr   error
	}{
		{
			name:      "success - persists a trimmed pending comment",
			contentID: 1,
			text:      "  Great article!  ",
			allow:     true,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("Create", mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: nil,
		},
		{
			name:      "error - invalid text is rejected before the gate",
			contentID: 1,
			text:      "   ",
			allow:     true,
			setupMock: func(m *mocks.MockCommentRepository) {},
			wantErr:   commentdomain.ErrInvalidCommentText,
		},
		{
			name:      "error - comments disabled on content",
			contentID: 1,
			text:      "Great article!",
			allow:     false,
			setupMock: func(m *mocks.MockCommentRepository) {},
			wantErr:   commentdomain.ErrCommentsDisabled,
		},
		{
			name:      "error - gate lookup failure wraps as content failure",
			contentID: 1,
			text:      "Great article!",
			allowErr:  errors.New("connection refused"),
			setupMock: func(m *mocks.MockCommentRepository) {},
			wantErr:   errors.New("failed to get content: connection refused"),
		},
		{
			name:      "error - deleted content maps to the domain not-found sentinel",
			contentID: 1,
			text:      "Great article!",
			allowErr:  commentdomain.ErrContentNotFound,
			setupMock: func(m *mocks.MockCommentRepository) {},
			wantErr:   commentdomain.ErrContentNotFound,
		},
		{
			name:      "error - repository failure wraps as create failure",
			contentID: 1,
			text:      "Great article!",
			allow:     true,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("Create", mock.Anything, mock.Anything).Return(errors.New("database error"))
			},
			wantErr: errors.New("failed to create comment: database error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockCommentRepository(t)
			tt.setupMock(mockRepo)

			s := commentdomain.NewService(
				mockRepo,
				commentdomain.WithAllowCommentsChecker(allowComments(tt.allow, tt.allowErr)),
			)
			comment, err := s.SubmitComment(
				context.Background(),
				tt.contentID,
				testPrincipal(1),
				commentdomain.CreateCommentRequest{Comment: tt.text},
			)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr.Error())
				return
			}

			require.NoError(t, err)
			require.NotNil(t, comment)
			assert.Equal(t, tt.contentID, comment.ContentID)
			assert.Equal(t, 1, comment.UserID)
			assert.Equal(t, "Great article!", comment.Comment)
			assert.Equal(t, commentdomain.CommentStatusPending, comment.Status)
		})
	}
}

func TestService_SubmitComment_FailClosedWithoutChecker(t *testing.T) {
	mockRepo := mocks.NewMockCommentRepository(t)
	s := commentdomain.NewService(mockRepo)

	_, err := s.SubmitComment(
		context.Background(),
		1,
		testPrincipal(1),
		commentdomain.CreateCommentRequest{Comment: "hi"},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, commentdomain.ErrCommentsDisabled)
}

func TestService_SubmitComment_InvalidPrincipal(t *testing.T) {
	mockRepo := mocks.NewMockCommentRepository(t)
	s := commentdomain.NewService(mockRepo, commentdomain.WithAllowCommentsChecker(allowComments(true, nil)))

	_, err := s.SubmitComment(
		context.Background(),
		1,
		kernel.Principal{ID: "not-a-number"},
		commentdomain.CreateCommentRequest{Comment: "hi"},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, kernel.ErrInvalidPrincipalID)
}

func TestService_GetCommentsByUserID(t *testing.T) {
	tests := []struct {
		name      string
		userID    int
		setupMock func(*mocks.MockCommentRepository)
		wantLen   int
		wantErr   bool
	}{
		{
			name:   "success - returns the user's comments",
			userID: 1,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByUserID", mock.Anything, 1).Return([]*commentdomain.Comment{
					{ID: 1, UserID: 1, Comment: "Mine"},
				}, nil)
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name:   "error - repository failure wraps",
			userID: 1,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByUserID", mock.Anything, 1).Return(nil, errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockCommentRepository(t)
			tt.setupMock(mockRepo)

			s := commentdomain.NewService(mockRepo)
			comments, err := s.GetCommentsByUserID(context.Background(), tt.userID)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to get comments by user")
				return
			}

			require.NoError(t, err)
			assert.Len(t, comments, tt.wantLen)
		})
	}
}

func TestService_CountComments(t *testing.T) {
	tests := []struct {
		name        string
		userID      int
		setupMock   func(*mocks.MockCommentRepository)
		expected    int
		expectedErr error
	}{
		{
			name:   "counts all comments for admin scope",
			userID: 0,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("Count", mock.Anything, 0).Return(42, nil)
			},
			expected: 42,
		},
		{
			name:   "counts comments for a specific user",
			userID: 7,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("Count", mock.Anything, 7).Return(3, nil)
			},
			expected: 3,
		},
		{
			name:   "wraps repository error",
			userID: 0,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("Count", mock.Anything, 0).Return(0, errors.New("database error"))
			},
			expectedErr: errors.New("failed to count comments"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockCommentRepository(t)
			tt.setupMock(mockRepo)

			s := commentdomain.NewService(mockRepo)
			total, err := s.CountComments(context.Background(), tt.userID)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, total)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetCommentsForContent(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mocks.MockCommentRepository)
		wantLen   int
		wantErr   bool
	}{
		{
			name: "success - returns approved comments",
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByContentID", mock.Anything, 1).Return([]*commentdomain.Comment{
					{ID: 1, ContentID: 1, Comment: "Nice"},
				}, nil)
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "error - repository failure wraps",
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByContentID", mock.Anything, 1).Return(nil, errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockCommentRepository(t)
			tt.setupMock(mockRepo)

			s := commentdomain.NewService(mockRepo)
			comments, err := s.GetCommentsForContent(context.Background(), 1)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to get comments")
				return
			}

			require.NoError(t, err)
			assert.Len(t, comments, tt.wantLen)
		})
	}
}

func TestService_GetCommentsForModeration(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mocks.MockCommentRepository)
		wantLen   int
		wantErr   bool
	}{
		{
			name: "success - returns every status",
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByContentIDForModeration", mock.Anything, 1).Return([]*commentdomain.Comment{
					{ID: 1, ContentID: 1, Status: commentdomain.CommentStatusPending},
					{ID: 2, ContentID: 1, Status: commentdomain.CommentStatusSpam},
				}, nil)
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "error - repository failure wraps",
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByContentIDForModeration", mock.Anything, 1).Return(nil, errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockCommentRepository(t)
			tt.setupMock(mockRepo)

			s := commentdomain.NewService(mockRepo)
			comments, err := s.GetCommentsForModeration(context.Background(), 1)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to get comments for moderation")
				return
			}

			require.NoError(t, err)
			assert.Len(t, comments, tt.wantLen)
		})
	}
}

func TestService_GetCommentsByStatus(t *testing.T) {
	tests := []struct {
		name        string
		status      commentdomain.CommentStatus
		setupMock   func(*mocks.MockCommentRepository)
		expectedLen int
		expectedErr error
	}{
		{
			name:   "returns comments by status with content context",
			status: commentdomain.CommentStatusPending,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByStatus", mock.Anything, commentdomain.CommentStatusPending).Return([]*commentdomain.Comment{
					{
						ID:           1,
						ContentID:    10,
						ContentTitle: "Getting Started with Go",
						ContentSlug:  "getting-started-with-go",
						Comment:      "Great article!",
						Author:       "Jane Doe",
						Status:       commentdomain.CommentStatusPending,
						CreatedAt:    time.Date(2026, 4, 19, 10, 30, 0, 0, time.UTC),
					},
				}, nil)
			},
			expectedLen: 1,
			expectedErr: nil,
		},
		{
			name:   "returns empty list when no comments match status",
			status: commentdomain.CommentStatusPending,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByStatus", mock.Anything, commentdomain.CommentStatusPending).Return([]*commentdomain.Comment{}, nil)
			},
			expectedLen: 0,
			expectedErr: nil,
		},
		{
			name:   "wraps repository error",
			status: commentdomain.CommentStatusPending,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByStatus", mock.Anything, commentdomain.CommentStatusPending).Return(nil, errors.New("database error"))
			},
			expectedErr: errors.New("failed to get comments by status"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockCommentRepository(t)
			tt.setupMock(mockRepo)

			s := commentdomain.NewService(mockRepo)
			result, err := s.GetCommentsByStatus(context.Background(), tt.status)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err)
			require.Len(t, result, tt.expectedLen)
			if tt.expectedLen > 0 {
				assert.Equal(t, "Getting Started with Go", result[0].ContentTitle)
				assert.Equal(t, "getting-started-with-go", result[0].ContentSlug)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_ModerationTransitions(t *testing.T) {
	tests := []struct {
		name       string
		transition func(*commentdomain.Service, context.Context) error
		wantStatus commentdomain.CommentStatus
		wantErr    bool
	}{
		{
			name: "success - approve moves to approved",
			transition: func(s *commentdomain.Service, ctx context.Context) error {
				return s.ApproveComment(ctx, 1)
			},
			wantStatus: commentdomain.CommentStatusApproved,
			wantErr:    false,
		},
		{
			name: "success - reject moves to rejected",
			transition: func(s *commentdomain.Service, ctx context.Context) error {
				return s.RejectComment(ctx, 1)
			},
			wantStatus: commentdomain.CommentStatusRejected,
			wantErr:    false,
		},
		{
			name: "success - mark as spam moves to spam",
			transition: func(s *commentdomain.Service, ctx context.Context) error {
				return s.MarkAsSpam(ctx, 1)
			},
			wantStatus: commentdomain.CommentStatusSpam,
			wantErr:    false,
		},
		{
			name: "error - approve failure wraps",
			transition: func(s *commentdomain.Service, ctx context.Context) error {
				return s.ApproveComment(ctx, 1)
			},
			wantStatus: commentdomain.CommentStatusApproved,
			wantErr:    true,
		},
		{
			name: "error - reject failure wraps",
			transition: func(s *commentdomain.Service, ctx context.Context) error {
				return s.RejectComment(ctx, 1)
			},
			wantStatus: commentdomain.CommentStatusRejected,
			wantErr:    true,
		},
		{
			name: "error - spam failure wraps",
			transition: func(s *commentdomain.Service, ctx context.Context) error {
				return s.MarkAsSpam(ctx, 1)
			},
			wantStatus: commentdomain.CommentStatusSpam,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockCommentRepository(t)
			if tt.wantErr {
				mockRepo.On("UpdateStatus", mock.Anything, 1, tt.wantStatus).
					Return(errors.New("database error"))
			} else {
				mockRepo.On("UpdateStatus", mock.Anything, 1, tt.wantStatus).Return(nil)
			}

			s := commentdomain.NewService(mockRepo)
			err := tt.transition(s, context.Background())

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to")
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestService_DeleteComment(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mocks.MockCommentRepository)
		wantErr   bool
	}{
		{
			name: "success - deletes by id",
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("Delete", mock.Anything, 1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "error - repository failure wraps",
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("Delete", mock.Anything, 1).Return(errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockCommentRepository(t)
			tt.setupMock(mockRepo)

			s := commentdomain.NewService(mockRepo)
			err := s.DeleteComment(context.Background(), 1)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to delete comment")
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestService_DeleteOwnComment(t *testing.T) {
	tests := []struct {
		name          string
		commentID     int
		userID        int
		setupMock     func(*mocks.MockCommentRepository)
		expectedErr   error
		matchContains bool
	}{
		{
			name:      "successful delete own comment",
			commentID: 1,
			userID:    1,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&commentdomain.Comment{
					ID:     1,
					UserID: 1,
				}, nil)
				m.On("Delete", mock.Anything, 1).Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:      "delete comment owned by another user",
			commentID: 2,
			userID:    1,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByID", mock.Anything, 2).Return(&commentdomain.Comment{
					ID:     2,
					UserID: 99,
				}, nil)
			},
			expectedErr: commentdomain.ErrCommentNotFound,
		},
		{
			name:      "delete non-existent comment",
			commentID: 999,
			userID:    1,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByID", mock.Anything, 999).Return(nil, errors.New("not found"))
			},
			expectedErr: commentdomain.ErrCommentNotFound,
		},
		{
			name:      "repository delete fails after ownership check",
			commentID: 1,
			userID:    1,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&commentdomain.Comment{
					ID:     1,
					UserID: 1,
				}, nil)
				m.On("Delete", mock.Anything, 1).Return(errors.New("database error"))
			},
			expectedErr:   errors.New("failed to delete comment: database error"),
			matchContains: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockCommentRepository(t)
			tt.setupMock(mockRepo)

			s := commentdomain.NewService(mockRepo)

			err := s.DeleteOwnComment(context.Background(), tt.commentID, testPrincipal(tt.userID))

			if tt.expectedErr != nil {
				if tt.matchContains {
					assert.ErrorContains(t, err, tt.expectedErr.Error())
				} else {
					require.ErrorIs(t, err, tt.expectedErr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestService_DeleteOwnComment_InvalidPrincipal(t *testing.T) {
	mockRepo := mocks.NewMockCommentRepository(t)
	s := commentdomain.NewService(mockRepo)

	err := s.DeleteOwnComment(context.Background(), 1, kernel.Principal{ID: "not-a-number"})
	require.Error(t, err)
	assert.ErrorIs(t, err, kernel.ErrInvalidPrincipalID)
}

func TestService_DeleteCommentsByUserID(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mocks.MockCommentRepository)
		wantErr   bool
	}{
		{
			name: "success - deletes every owned comment",
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("DeleteByUserID", mock.Anything, 7).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "error - repository failure wraps",
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("DeleteByUserID", mock.Anything, 7).Return(errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockCommentRepository(t)
			tt.setupMock(mockRepo)

			s := commentdomain.NewService(mockRepo)
			err := s.DeleteCommentsByUserID(context.Background(), 7)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to delete comments by user")
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestService_GetComment(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mocks.MockCommentRepository)
		wantID    int
		wantErr   bool
	}{
		{
			name: "success - returns the comment",
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&commentdomain.Comment{
					ID:        1,
					ContentID: 5,
					Comment:   "Nice",
				}, nil)
			},
			wantID:  1,
			wantErr: false,
		},
		{
			name: "error - repository failure wraps",
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("GetByID", mock.Anything, 1).Return(nil, errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockCommentRepository(t)
			tt.setupMock(mockRepo)

			s := commentdomain.NewService(mockRepo)
			comment, err := s.GetComment(context.Background(), 1)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to get comment")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantID, comment.ID)
		})
	}
}

func TestService_UpdateCommentStatus(t *testing.T) {
	tests := []struct {
		name      string
		status    commentdomain.CommentStatus
		setupMock func(*mocks.MockCommentRepository)
		wantErr   error
	}{
		{
			name:   "success - moves to a valid status",
			status: commentdomain.CommentStatusApproved,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("UpdateStatus", mock.Anything, 1, commentdomain.CommentStatusApproved).Return(nil)
			},
			wantErr: nil,
		},
		{
			name:      "error - unknown status is rejected before the repository",
			status:    commentdomain.CommentStatus("bogus"),
			setupMock: func(m *mocks.MockCommentRepository) {},
			wantErr:   commentdomain.ErrInvalidCommentStatus,
		},
		{
			name:   "error - repository failure wraps",
			status: commentdomain.CommentStatusSpam,
			setupMock: func(m *mocks.MockCommentRepository) {
				m.On("UpdateStatus", mock.Anything, 1, commentdomain.CommentStatusSpam).
					Return(errors.New("database error"))
			},
			wantErr: errors.New("failed to update comment status"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockCommentRepository(t)
			tt.setupMock(mockRepo)

			s := commentdomain.NewService(mockRepo)
			err := s.UpdateCommentStatus(context.Background(), 1, tt.status)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr.Error())
				return
			}

			require.NoError(t, err)
		})
	}
}
