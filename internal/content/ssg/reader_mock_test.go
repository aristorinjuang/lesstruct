package ssg_test

import (
	"context"

	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	"github.com/stretchr/testify/mock"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
)

// mockContentReader is a hand-rolled testify mock for the site-ssr
// ContentReader seam used by the generator.
type mockContentReader struct {
	mock.Mock
}

func newMockContentReader(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockContentReader {
	m := &mockContentReader{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

func (m *mockContentReader) GetPublished(ctx context.Context, limit, offset int) ([]sitessr.ContentRef, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]sitessr.ContentRef), args.Error(1)
}

func (m *mockContentReader) GetPublishedPages(ctx context.Context) ([]sitessr.ContentRef, error) {
	args := m.Called(ctx)
	return args.Get(0).([]sitessr.ContentRef), args.Error(1)
}

func (m *mockContentReader) GetPublishedCustomPostTypes(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockContentReader) GetTranslations(ctx context.Context, groupID, excludeID int) ([]sitessr.ContentRef, error) {
	args := m.Called(ctx, groupID, excludeID)
	return args.Get(0).([]sitessr.ContentRef), args.Error(1)
}

func (m *mockContentReader) GetPublishedByID(ctx context.Context, id int) (sitessr.ContentRef, error) {
	args := m.Called(ctx, id)
	ref, _ := args.Get(0).(sitessr.ContentRef)
	return ref, args.Error(1)
}

func (m *mockContentReader) GetPublishedByPostType(
	ctx context.Context,
	postType string,
	languages []string,
	year, month, limit, offset int,
) ([]sitessr.ContentRef, error) {
	args := m.Called(ctx, postType, languages, year, month, limit, offset)
	return args.Get(0).([]sitessr.ContentRef), args.Error(1)
}

func (m *mockContentReader) GetPublishedTags(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockContentReader) GetPublishedBySlugAny(ctx context.Context, slug string) (sitessr.ContentRef, error) {
	args := m.Called(ctx, slug)
	return args.Get(0).(sitessr.ContentRef), args.Error(1)
}

func (m *mockContentReader) GetCommentsForContent(ctx context.Context, contentID int) ([]*commentdomain.Comment, error) {
	args := m.Called(ctx, contentID)
	return args.Get(0).([]*commentdomain.Comment), args.Error(1)
}

func (m *mockContentReader) GetRelated(ctx context.Context, id, limit int) ([]sitessr.ContentRef, error) {
	args := m.Called(ctx, id, limit)
	return args.Get(0).([]sitessr.ContentRef), args.Error(1)
}

func (m *mockContentReader) GetPublishedByAuthorUsername(
	ctx context.Context,
	username string,
	languages []string,
	limit, offset int,
) ([]sitessr.ContentRef, error) {
	args := m.Called(ctx, username, languages, limit, offset)
	return args.Get(0).([]sitessr.ContentRef), args.Error(1)
}

func (m *mockContentReader) AuthorExists(ctx context.Context, username string) (bool, error) {
	args := m.Called(ctx, username)
	return args.Bool(0), args.Error(1)
}

func (m *mockContentReader) GetPublishedByTag(
	ctx context.Context,
	tag string,
	languages []string,
	year, month, limit, offset int,
) ([]sitessr.ContentRef, error) {
	args := m.Called(ctx, tag, languages, year, month, limit, offset)
	return args.Get(0).([]sitessr.ContentRef), args.Error(1)
}

// stubResolver is a testify-backed PostTypeResolver stub; tests set
// expectations with On("GetBySlug", ...).Returns(...).
type stubResolver struct {
	mock.Mock
}

func (m *stubResolver) GetBySlug(slug string) (posttype.PostType, error) {
	args := m.Called(slug)
	return args.Get(0).(posttype.PostType), args.Error(1)
}
