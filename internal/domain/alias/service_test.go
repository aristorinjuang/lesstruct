package alias_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/alias"
	"github.com/aristorinjuang/lesstruct/internal/domain/alias/mocks"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_Create(t *testing.T) {
	tests := []struct {
		name      string
		contentID int
		aliasStr  string
		repoErr   error
		wantErr   bool
	}{
		{
			name:      "success - creates alias",
			contentID: 1,
			aliasStr:  "old-post.html",
			repoErr:   nil,
			wantErr:   false,
		},
		{
			name:      "error - repo returns error",
			contentID: 2,
			aliasStr:  "another-post.html",
			repoErr:   errors.New("db error"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(a *alias.Alias) bool {
				return a.ContentID == tt.contentID && a.Alias == tt.aliasStr
			})).Return(tt.repoErr)

			svc := alias.NewService(mockRepo)
			err := svc.Create(context.Background(), tt.contentID, tt.aliasStr)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestService_FindByAlias(t *testing.T) {
	expected := &alias.Alias{ID: 1, ContentID: 10, Alias: "old-post.html"}

	tests := []struct {
		name     string
		aliasStr string
		repoRet  *alias.Alias
		repoErr  error
		want     *alias.Alias
		wantErr  bool
	}{
		{
			name:     "success - finds alias",
			aliasStr: "old-post.html",
			repoRet:  expected,
			repoErr:  nil,
			want:     expected,
			wantErr:  false,
		},
		{
			name:     "error - alias not found",
			aliasStr: "missing.html",
			repoRet:  nil,
			repoErr:  alias.ErrAliasNotFound,
			want:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockRepo.EXPECT().FindByAlias(mock.Anything, tt.aliasStr).Return(tt.repoRet, tt.repoErr)

			svc := alias.NewService(mockRepo)
			got, err := svc.FindByAlias(context.Background(), tt.aliasStr)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestService_FindByContentID(t *testing.T) {
	expected := []*alias.Alias{
		{ID: 1, ContentID: 10, Alias: "old-post.html"},
		{ID: 2, ContentID: 10, Alias: "older-post.html"},
	}

	tests := []struct {
		name      string
		contentID int
		repoRet   []*alias.Alias
		repoErr   error
		want      []*alias.Alias
		wantErr   bool
	}{
		{
			name:      "success - finds aliases",
			contentID: 10,
			repoRet:   expected,
			repoErr:   nil,
			want:      expected,
			wantErr:   false,
		},
		{
			name:      "success - empty result",
			contentID: 99,
			repoRet:   []*alias.Alias{},
			repoErr:   nil,
			want:      []*alias.Alias{},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockRepo.EXPECT().FindByContentID(mock.Anything, tt.contentID).Return(tt.repoRet, tt.repoErr)

			svc := alias.NewService(mockRepo)
			got, err := svc.FindByContentID(context.Background(), tt.contentID)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestService_FindAll(t *testing.T) {
	expected := []*alias.Alias{
		{ID: 1, ContentID: 10, Alias: "first.html"},
		{ID: 2, ContentID: 20, Alias: "second.html"},
	}

	tests := []struct {
		name    string
		repoRet []*alias.Alias
		repoErr error
		want    []*alias.Alias
		wantErr bool
	}{
		{
			name:    "success - returns all aliases",
			repoRet: expected,
			repoErr: nil,
			want:    expected,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockRepo.EXPECT().FindAll(mock.Anything).Return(tt.repoRet, tt.repoErr)

			svc := alias.NewService(mockRepo)
			got, err := svc.FindAll(context.Background())
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestService_DeleteByContentID(t *testing.T) {
	tests := []struct {
		name      string
		contentID int
		repoErr   error
		wantErr   bool
	}{
		{
			name:      "success - deletes aliases",
			contentID: 10,
			repoErr:   nil,
			wantErr:   false,
		},
		{
			name:      "error - repo returns error",
			contentID: 99,
			repoErr:   errors.New("db error"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockRepo.EXPECT().DeleteByContentID(mock.Anything, tt.contentID).Return(tt.repoErr)

			svc := alias.NewService(mockRepo)
			err := svc.DeleteByContentID(context.Background(), tt.contentID)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestService_Repoint(t *testing.T) {
	tests := []struct {
		name          string
		aliasStr      string
		fromContentID int
		toContentID   int
		repoErr       error
		wantErr       bool
	}{
		{
			name:          "success - re-points alias",
			aliasStr:      "old-post.html",
			fromContentID: 99,
			toContentID:   42,
			repoErr:       nil,
			wantErr:       false,
		},
		{
			name:          "error - repo returns error",
			aliasStr:      "old-post.html",
			fromContentID: 99,
			toContentID:   42,
			repoErr:       errors.New("db error"),
			wantErr:       true,
		},
		{
			name:          "error - alias not found",
			aliasStr:      "old-post.html",
			fromContentID: 99,
			toContentID:   42,
			repoErr:       alias.ErrAliasNotFound,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockRepo.EXPECT().Repoint(mock.Anything, tt.aliasStr, tt.fromContentID, tt.toContentID).Return(tt.repoErr)

			svc := alias.NewService(mockRepo)
			err := svc.Repoint(context.Background(), tt.aliasStr, tt.fromContentID, tt.toContentID)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
