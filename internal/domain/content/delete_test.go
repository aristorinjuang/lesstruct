package content_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/content/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_DeleteContent(t *testing.T) {
	tests := []struct {
		name    string
		id      int
		userID  int
		setup   func(*mocks.MockRepository)
		wantErr error
	}{
		{
			name:   "successful deletion",
			id:     1,
			userID: 1,
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:     1,
					UserID: 1,
					Title:  "Test Content",
				}, nil)
				m.On("Delete", mock.Anything, 1, 1).Return(nil)
			},
			wantErr: nil,
		},
		{
			name:   "content not found",
			id:     999,
			userID: 1,
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 999).Return((*content.Content)(nil), content.ErrContentNotFound)
			},
			wantErr: content.ErrContentNotFound,
		},
		{
			name:   "unauthorized - wrong owner",
			id:     1,
			userID: 2,
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:     1,
					UserID: 1,
					Title:  "Test Content",
				}, nil)
			},
			wantErr: content.ErrUnauthorized,
		},
		{
			name:   "repository delete fails",
			id:     1,
			userID: 1,
			setup: func(m *mocks.MockRepository) {
				m.On("GetByID", mock.Anything, 1).Return(&content.Content{
					ID:     1,
					UserID: 1,
					Title:  "Test Content",
				}, nil)
				m.On("Delete", mock.Anything, 1, 1).Return(fmt.Errorf("database error"))
			},
			wantErr: fmt.Errorf("database error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			tt.setup(mockRepo)

			service := content.NewService(mockRepo, nil, nil)
			err := service.DeleteContent(context.Background(), tt.id, tt.userID, "")

			if tt.wantErr != nil {
				require.Error(t, err)
				t.Logf("Error: %v", err)
			} else {
				require.NoError(t, err)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}
