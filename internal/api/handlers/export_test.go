package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/content/export"
	aliasdomain "github.com/aristorinjuang/lesstruct/internal/domain/alias"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type exportTestContentSvc struct {
	mock.Mock
}

func (m *exportTestContentSvc) GetAll(ctx context.Context, limit int, offset int) ([]*contentdomain.Content, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]*contentdomain.Content), args.Error(1)
}

func (m *exportTestContentSvc) GetTranslations(ctx context.Context, translationGroupID int, excludeID int) ([]*contentdomain.Content, error) {
	args := m.Called(ctx, translationGroupID, excludeID)
	return args.Get(0).([]*contentdomain.Content), args.Error(1)
}

type exportTestAliasSvc struct {
	mock.Mock
}

func (m *exportTestAliasSvc) FindByContentID(ctx context.Context, contentID int) ([]*aliasdomain.Alias, error) {
	args := m.Called(ctx, contentID)
	return args.Get(0).([]*aliasdomain.Alias), args.Error(1)
}

func TestExportHandler_Export_Success(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	contentSvc := new(exportTestContentSvc)
	aliasSvc := new(exportTestAliasSvc)

	aliasSvc.On("FindByContentID", mock.Anything, 1).Return([]*aliasdomain.Alias{}, nil)
	contentSvc.On("GetAll", mock.Anything, 100, 0).Return([]*contentdomain.Content{
		{
			ID:        1,
			Title:     "Test Post",
			Slug:      "test-post",
			Content:   "<p>Hello</p>",
			Format:    contentdomain.FormatHTML,
			Status:    contentdomain.StatusPublished,
			PostType:  "post",
			Language:  "en",
			CreatedAt: now,
		},
	}, nil)
	contentSvc.On("GetAll", mock.Anything, 100, 100).Return([]*contentdomain.Content{}, nil)

	bodyToHTML := func(s string) (string, error) { return s, nil }

	exporter := export.NewExporter(
		contentSvc,
		aliasSvc,
		bodyToHTML,
		"",
		util.NewLogger(io.Discard),
	)

	logger := util.NewLogger(io.Discard)
	handler := &ExportHandler{
		exporter: exporter,
		logger:   logger,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/export", nil)
	w := httptest.NewRecorder()

	handler.Export(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/gzip", resp.Header.Get("Content-Type"))
	assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment; filename=\"lesstruct-export-")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.NotEmpty(t, body)

	gzr, err := gzip.NewReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	header, err := tr.Next()
	require.NoError(t, err)
	assert.Equal(t, "content/post/test-post.en.html", header.Name)
}

func TestExportHandler_Export_ExporterError(t *testing.T) {
	contentSvc := new(exportTestContentSvc)
	aliasSvc := new(exportTestAliasSvc)

	contentSvc.On("GetAll", mock.Anything, 100, 0).Return([]*contentdomain.Content{}, assert.AnError)

	bodyToHTML := func(s string) (string, error) { return s, nil }

	exporter := export.NewExporter(
		contentSvc,
		aliasSvc,
		bodyToHTML,
		"",
		util.NewLogger(io.Discard),
	)

	logger := util.NewLogger(io.Discard)
	handler := &ExportHandler{
		exporter: exporter,
		logger:   logger,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/export", nil)
	w := httptest.NewRecorder()

	handler.Export(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
