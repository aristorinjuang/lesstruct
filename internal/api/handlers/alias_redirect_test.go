package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/domain/alias"
	"github.com/aristorinjuang/lesstruct/internal/domain/alias/mocks"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockContentGetter struct {
	mock.Mock
}

func (m *mockContentGetter) GetByID(ctx context.Context, id int) (*contentdomain.Content, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*contentdomain.Content), args.Error(1)
}

func TestAliasRedirectHandler(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		aliasStr       string
		aliasRet       *alias.Alias
		aliasErr       error
		contentRet     *contentdomain.Content
		contentErr     error
		expectedStatus int
		expectedURL    string
	}{
		{
			name:           "redirects on alias match",
			path:           "/old-post.html",
			aliasStr:       "old-post.html",
			aliasRet:       &alias.Alias{ID: 1, ContentID: 10, Alias: "old-post.html"},
			aliasErr:       nil,
			contentRet:     &contentdomain.Content{ID: 10, Slug: "new-post"},
			contentErr:     nil,
			expectedStatus: http.StatusMovedPermanently,
			expectedURL:    "/new-post",
		},
		{
			name:           "passes through on alias not found",
			path:           "/unknown-path",
			aliasStr:       "unknown-path",
			aliasRet:       nil,
			aliasErr:       alias.ErrAliasNotFound,
			contentRet:     nil,
			contentErr:     nil,
			expectedStatus: http.StatusOK,
			expectedURL:    "",
		},
		{
			name:           "passes through on alias lookup error",
			path:           "/error-path",
			aliasStr:       "error-path",
			aliasRet:       nil,
			aliasErr:       assert.AnError,
			contentRet:     nil,
			contentErr:     nil,
			expectedStatus: http.StatusOK,
			expectedURL:    "",
		},
		{
			name:           "passes through on content not found",
			path:           "/orphaned-alias.html",
			aliasStr:       "orphaned-alias.html",
			aliasRet:       &alias.Alias{ID: 2, ContentID: 99, Alias: "orphaned-alias.html"},
			aliasErr:       nil,
			contentRet:     nil,
			contentErr:     assert.AnError,
			expectedStatus: http.StatusOK,
			expectedURL:    "",
		},
		{
			name:           "redirects preserving query params",
			path:           "/old-post.html?utm_source=test",
			aliasStr:       "old-post.html",
			aliasRet:       &alias.Alias{ID: 1, ContentID: 10, Alias: "old-post.html"},
			aliasErr:       nil,
			contentRet:     &contentdomain.Content{ID: 10, Slug: "new-post"},
			contentErr:     nil,
			expectedStatus: http.StatusMovedPermanently,
			expectedURL:    "/new-post?utm_source=test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			aliasSvc := alias.NewService(mockRepo)

			mockContentSvc := new(mockContentGetter)

			mockRepo.EXPECT().FindByAlias(mock.Anything, tt.aliasStr).Return(tt.aliasRet, tt.aliasErr)

			if tt.aliasRet != nil && tt.aliasErr == nil {
				mockContentSvc.On("GetByID", mock.Anything, tt.aliasRet.ContentID).Return(tt.contentRet, tt.contentErr)
			}

			nextCalled := false
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := handlers.NewAliasRedirectHandler(aliasSvc, mockContentSvc, nextHandler)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if tt.expectedStatus == http.StatusMovedPermanently {
				require.Equal(t, tt.expectedStatus, rec.Code)
				require.Equal(t, tt.expectedURL, rec.Header().Get("Location"))
				assert.False(t, nextCalled, "next handler should not be called on redirect")
			} else {
				require.Equal(t, tt.expectedStatus, rec.Code)
				assert.True(t, nextCalled, "next handler should be called on pass-through")
			}
		})
	}
}
