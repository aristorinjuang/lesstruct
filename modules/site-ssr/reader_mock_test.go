package sitessr_test

import (
	"context"

	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	"github.com/stretchr/testify/mock"
)

// mockReader is a hand-rolled testify mock for the sitessr.ContentReader
// seam. The engine only ever reads published projections through it.
type mockReader struct {
	mock.Mock
}

// newMockReader builds the reader without expectation enforcement — matching
// the historical new(mocks.MockContentService) usage where only tests that
// explicitly called AssertExpectations enforced calls.
func newMockReader() *mockReader {
	return &mockReader{}
}

func (m *mockReader) GetPublished(ctx context.Context, limit, offset int) ([]sitessr.ContentRef, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]sitessr.ContentRef), args.Error(1)
}

func (m *mockReader) GetPublishedPages(ctx context.Context) ([]sitessr.ContentRef, error) {
	args := m.Called(ctx)
	return args.Get(0).([]sitessr.ContentRef), args.Error(1)
}

func (m *mockReader) GetPublishedCustomPostTypes(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockReader) GetTranslations(ctx context.Context, groupID, excludeID int) ([]sitessr.ContentRef, error) {
	args := m.Called(ctx, groupID, excludeID)
	return args.Get(0).([]sitessr.ContentRef), args.Error(1)
}

func (m *mockReader) GetPublishedByID(ctx context.Context, id int) (sitessr.ContentRef, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sitessr.ContentRef), args.Error(1)
}

func (m *mockReader) GetPublishedByPostType(
	ctx context.Context,
	postType string,
	languages []string,
	year, month, limit, offset int,
) ([]sitessr.ContentRef, error) {
	args := m.Called(ctx, postType, languages, year, month, limit, offset)
	return args.Get(0).([]sitessr.ContentRef), args.Error(1)
}

func (m *mockReader) GetPublishedTags(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockReader) GetPublishedBySlugAny(ctx context.Context, slug string) (sitessr.ContentRef, error) {
	args := m.Called(ctx, slug)
	return args.Get(0).(sitessr.ContentRef), args.Error(1)
}

func (m *mockReader) GetCommentsForContent(ctx context.Context, contentID int) ([]*commentdomain.Comment, error) {
	args := m.Called(ctx, contentID)
	return args.Get(0).([]*commentdomain.Comment), args.Error(1)
}

func (m *mockReader) GetRelated(ctx context.Context, id, limit int) ([]sitessr.ContentRef, error) {
	args := m.Called(ctx, id, limit)
	return args.Get(0).([]sitessr.ContentRef), args.Error(1)
}

func (m *mockReader) GetPublishedByAuthorUsername(
	ctx context.Context,
	username string,
	languages []string,
	limit, offset int,
) ([]sitessr.ContentRef, error) {
	args := m.Called(ctx, username, languages, limit, offset)
	return args.Get(0).([]sitessr.ContentRef), args.Error(1)
}

func (m *mockReader) AuthorExists(ctx context.Context, username string) (bool, error) {
	args := m.Called(ctx, username)
	return args.Bool(0), args.Error(1)
}

func (m *mockReader) GetPublishedByTag(
	ctx context.Context,
	tag string,
	languages []string,
	year, month, limit, offset int,
) ([]sitessr.ContentRef, error) {
	args := m.Called(ctx, tag, languages, year, month, limit, offset)
	return args.Get(0).([]sitessr.ContentRef), args.Error(1)
}
