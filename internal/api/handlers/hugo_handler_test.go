package handlers_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	hugopkg "github.com/aristorinjuang/lesstruct/internal/content/hugo"
	hugomocks "github.com/aristorinjuang/lesstruct/internal/content/hugo/mocks"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func createTarGz(t *testing.T, files map[string]string) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	for name, content := range files {
		hdr := &tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0644,
			Typeflag: tar.TypeReg,
		}
		require.NoError(t, tarWriter.WriteHeader(hdr))
		_, err := tarWriter.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzWriter.Close())
	return &buf
}

func buildMultipartRequest(
	t *testing.T,
	method string,
	target string,
	fieldName string,
	filename string,
	body io.Reader,
) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if body != nil {
		fw, err := w.CreateFormFile(fieldName, filename)
		require.NoError(t, err)
		_, err = io.Copy(fw, body)
		require.NoError(t, err)
	}

	require.NoError(t, w.Close())

	req := httptest.NewRequest(method, target, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestHugoHandler_Import(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func(*testing.T) (*http.Request, *hugomocks.MockContentCreator, *hugomocks.MockAliasCreator)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name: "success - imports hugo posts from archive",
			setupRequest: func(t *testing.T) (*http.Request, *hugomocks.MockContentCreator, *hugomocks.MockAliasCreator) {
				tarGz := createTarGz(t, map[string]string{
					"content/post.html": "---\ntitle: My Test Post\n---\n<p>Hello world</p>",
				})

				req := buildMultipartRequest(t, http.MethodPost, "/admin/hugo/import", "file", "site.tar.gz", tarGz)
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
				req = req.WithContext(ctx)

				contentCreator := hugomocks.NewMockContentCreator(t)
				aliasCreator := hugomocks.NewMockAliasCreator(t)

				contentCreator.EXPECT().Create(
					mock.Anything,
					1,
					mock.AnythingOfType("content.CreateContentRequest"),
				).Return(&contentdomain.Content{ID: 1}, nil)

				return req, contentCreator, aliasCreator
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, float64(1), data["imported"])
				assert.Equal(t, float64(0), data["skipped"])
				assert.Nil(t, resp["error"])
			},
		},
		{
			name: "success with tgz extension",
			setupRequest: func(t *testing.T) (*http.Request, *hugomocks.MockContentCreator, *hugomocks.MockAliasCreator) {
				tarGz := createTarGz(t, map[string]string{
					"content/post.md": "---\ntitle: Markdown Post\n---\n# Hello",
				})

				req := buildMultipartRequest(t, http.MethodPost, "/admin/hugo/import", "file", "export.tgz", tarGz)
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
				req = req.WithContext(ctx)

				contentCreator := hugomocks.NewMockContentCreator(t)
				aliasCreator := hugomocks.NewMockAliasCreator(t)

				contentCreator.EXPECT().Create(
					mock.Anything,
					1,
					mock.AnythingOfType("content.CreateContentRequest"),
				).Return(&contentdomain.Content{ID: 1}, nil)

				return req, contentCreator, aliasCreator
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, float64(1), data["imported"])
				assert.Nil(t, resp["error"])
			},
		},
		{
			name: "invalid file type - not tar.gz",
			setupRequest: func(t *testing.T) (*http.Request, *hugomocks.MockContentCreator, *hugomocks.MockAliasCreator) {
				req := buildMultipartRequest(t, http.MethodPost, "/admin/hugo/import", "file", "archive.zip", bytes.NewReader([]byte("not a real zip")))
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
				req = req.WithContext(ctx)

				return req, hugomocks.NewMockContentCreator(t), hugomocks.NewMockAliasCreator(t)
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "invalid_file", err["code"])
			},
		},
		{
			name: "missing file field",
			setupRequest: func(t *testing.T) (*http.Request, *hugomocks.MockContentCreator, *hugomocks.MockAliasCreator) {
				var buf bytes.Buffer
				w := multipart.NewWriter(&buf)
				require.NoError(t, w.Close())

				req := httptest.NewRequest(http.MethodPost, "/admin/hugo/import", &buf)
				req.Header.Set("Content-Type", w.FormDataContentType())
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
				req = req.WithContext(ctx)

				return req, hugomocks.NewMockContentCreator(t), hugomocks.NewMockAliasCreator(t)
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "invalid_request", err["code"])
			},
		},
		{
			name: "unauthorized - no user ID in context",
			setupRequest: func(t *testing.T) (*http.Request, *hugomocks.MockContentCreator, *hugomocks.MockAliasCreator) {
				tarGz := createTarGz(t, map[string]string{
					"content/post.html": "---\ntitle: Test\n---\n<p>body</p>",
				})

				req := buildMultipartRequest(t, http.MethodPost, "/admin/hugo/import", "file", "site.tar.gz", tarGz)

				return req, hugomocks.NewMockContentCreator(t), hugomocks.NewMockAliasCreator(t)
			},
			expectedStatus: http.StatusUnauthorized,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "unauthorized", err["code"])
			},
		},
		{
			name: "no content directory in archive",
			setupRequest: func(t *testing.T) (*http.Request, *hugomocks.MockContentCreator, *hugomocks.MockAliasCreator) {
				tarGz := createTarGz(t, map[string]string{
					"static/style.css": "* { color: red }",
				})

				req := buildMultipartRequest(t, http.MethodPost, "/admin/hugo/import", "file", "site.tar.gz", tarGz)
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
				req = req.WithContext(ctx)

				return req, hugomocks.NewMockContentCreator(t), hugomocks.NewMockAliasCreator(t)
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "invalid_archive", err["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, contentCreator, aliasCreator := tt.setupRequest(t)

			importer := hugopkg.NewImporter(contentCreator, aliasCreator, "en", util.NewLogger(io.Discard))
			handler := handlers.NewHugoHandler(importer, util.NewLogger(io.Discard), 10<<20)

			w := httptest.NewRecorder()
			handler.Import(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			var resp map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}
