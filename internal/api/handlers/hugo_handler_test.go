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
	"time"

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

// newHugoTestHandler wires an importer over the given mocks and returns the
// handler plus the mocks for per-case expectations.
func newHugoTestHandler(
	cc *hugomocks.MockContentCreator,
	ac *hugomocks.MockAliasCreator,
	sc *hugomocks.MockSlugResolver,
	ms *hugomocks.MockMediaService,
) *handlers.HugoHandler {
	importer := hugopkg.NewImporter(cc, ac, sc, ms, nil, "en", util.NewLogger(io.Discard))
	return handlers.NewHugoHandler(importer, util.NewLogger(io.Discard), 10<<20, time.Minute)
}

// pollJobStatus polls the status endpoint until the job reaches a terminal
// state (done/failed) or the timeout expires.
func pollJobStatus(
	t *testing.T,
	handler *handlers.HugoHandler,
	jobID string,
	path string,
) map[string]any {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		req := httptest.NewRequest(http.MethodGet, path+jobID, nil)
		w := httptest.NewRecorder()
		handler.ImportStatus(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		job, ok := resp["data"].(map[string]any)
		if !ok {
			continue
		}
		state, ok := job["job"].(map[string]any)["state"].(string)
		if !ok {
			continue
		}
		if state == "done" || state == "failed" {
			return resp
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for import job to finish")
	return nil
}

func TestHugoHandler_Import(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func(*testing.T) (*http.Request, *hugomocks.MockContentCreator, *hugomocks.MockAliasCreator, *hugomocks.MockSlugResolver, *hugomocks.MockMediaService)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
		expectJobDone  bool
	}{
		{
			name: "success - imports hugo posts from archive",
			setupRequest: func(t *testing.T) (*http.Request, *hugomocks.MockContentCreator, *hugomocks.MockAliasCreator, *hugomocks.MockSlugResolver, *hugomocks.MockMediaService) {
				tarGz := createTarGz(t, map[string]string{
					"content/post.html": "---\ntitle: My Test Post\n---\n<p>Hello world</p>",
				})

				req := buildMultipartRequest(t, http.MethodPost, "/admin/hugo/import", "file", "site.tar.gz", tarGz)
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
				req = req.WithContext(ctx)

				contentCreator := hugomocks.NewMockContentCreator(t)
				aliasCreator := hugomocks.NewMockAliasCreator(t)
				slugChecker := hugomocks.NewMockSlugResolver(t)
				mediaService := hugomocks.NewMockMediaService(t)

				slugChecker.EXPECT().SlugExists(
					mock.Anything,
					"my-test-post",
					"en",
				).Return(false, nil).Once()
				contentCreator.EXPECT().Create(
					mock.Anything,
					1,
					mock.AnythingOfType("content.CreateContentRequest"),
				).Return(&contentdomain.Content{ID: 1}, nil)

				return req, contentCreator, aliasCreator, slugChecker, mediaService
			},
			expectedStatus: http.StatusAccepted,
			expectJobDone:  true,
			validateResp: func(t *testing.T, resp map[string]any) {
				job := resp["data"].(map[string]any)["job"].(map[string]any)
				assert.Equal(t, "done", job["state"])
				assert.Equal(t, float64(1), job["imported"])
				assert.Equal(t, float64(0), job["skipped"])
			},
		},
		{
			name: "success with tgz extension",
			setupRequest: func(t *testing.T) (*http.Request, *hugomocks.MockContentCreator, *hugomocks.MockAliasCreator, *hugomocks.MockSlugResolver, *hugomocks.MockMediaService) {
				tarGz := createTarGz(t, map[string]string{
					"content/post.md": "---\ntitle: Markdown Post\n---\n# Hello",
				})

				req := buildMultipartRequest(t, http.MethodPost, "/admin/hugo/import", "file", "export.tgz", tarGz)
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
				req = req.WithContext(ctx)

				contentCreator := hugomocks.NewMockContentCreator(t)
				aliasCreator := hugomocks.NewMockAliasCreator(t)
				slugChecker := hugomocks.NewMockSlugResolver(t)
				mediaService := hugomocks.NewMockMediaService(t)

				slugChecker.EXPECT().SlugExists(
					mock.Anything,
					"markdown-post",
					"en",
				).Return(false, nil).Once()
				contentCreator.EXPECT().Create(
					mock.Anything,
					1,
					mock.AnythingOfType("content.CreateContentRequest"),
				).Return(&contentdomain.Content{ID: 1}, nil)

				return req, contentCreator, aliasCreator, slugChecker, mediaService
			},
			expectedStatus: http.StatusAccepted,
			expectJobDone:  true,
			validateResp: func(t *testing.T, resp map[string]any) {
				job := resp["data"].(map[string]any)["job"].(map[string]any)
				assert.Equal(t, "done", job["state"])
				assert.Equal(t, float64(1), job["imported"])
			},
		},
		{
			name: "invalid file type - not tar.gz",
			setupRequest: func(t *testing.T) (*http.Request, *hugomocks.MockContentCreator, *hugomocks.MockAliasCreator, *hugomocks.MockSlugResolver, *hugomocks.MockMediaService) {
				req := buildMultipartRequest(t, http.MethodPost, "/admin/hugo/import", "file", "archive.zip", bytes.NewReader([]byte("not a real zip")))
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
				req = req.WithContext(ctx)

				return req, hugomocks.NewMockContentCreator(t), hugomocks.NewMockAliasCreator(t), hugomocks.NewMockSlugResolver(t), hugomocks.NewMockMediaService(t)
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
			setupRequest: func(t *testing.T) (*http.Request, *hugomocks.MockContentCreator, *hugomocks.MockAliasCreator, *hugomocks.MockSlugResolver, *hugomocks.MockMediaService) {
				var buf bytes.Buffer
				w := multipart.NewWriter(&buf)
				require.NoError(t, w.Close())

				req := httptest.NewRequest(http.MethodPost, "/admin/hugo/import", &buf)
				req.Header.Set("Content-Type", w.FormDataContentType())
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
				req = req.WithContext(ctx)

				return req, hugomocks.NewMockContentCreator(t), hugomocks.NewMockAliasCreator(t), hugomocks.NewMockSlugResolver(t), hugomocks.NewMockMediaService(t)
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
			setupRequest: func(t *testing.T) (*http.Request, *hugomocks.MockContentCreator, *hugomocks.MockAliasCreator, *hugomocks.MockSlugResolver, *hugomocks.MockMediaService) {
				tarGz := createTarGz(t, map[string]string{
					"content/post.html": "---\ntitle: Test\n---\n<p>body</p>",
				})

				req := buildMultipartRequest(t, http.MethodPost, "/admin/hugo/import", "file", "site.tar.gz", tarGz)

				return req, hugomocks.NewMockContentCreator(t), hugomocks.NewMockAliasCreator(t), hugomocks.NewMockSlugResolver(t), hugomocks.NewMockMediaService(t)
			},
			expectedStatus: http.StatusUnauthorized,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "unauthorized", err["code"])
			},
		},
		{
			name: "no content directory in archive fails the job",
			setupRequest: func(t *testing.T) (*http.Request, *hugomocks.MockContentCreator, *hugomocks.MockAliasCreator, *hugomocks.MockSlugResolver, *hugomocks.MockMediaService) {
				tarGz := createTarGz(t, map[string]string{
					"static/style.css": "* { color: red }",
				})

				req := buildMultipartRequest(t, http.MethodPost, "/admin/hugo/import", "file", "site.tar.gz", tarGz)
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
				req = req.WithContext(ctx)

				return req, hugomocks.NewMockContentCreator(t), hugomocks.NewMockAliasCreator(t), hugomocks.NewMockSlugResolver(t), hugomocks.NewMockMediaService(t)
			},
			expectedStatus: http.StatusAccepted,
			expectJobDone:  true,
			validateResp: func(t *testing.T, resp map[string]any) {
				job := resp["data"].(map[string]any)["job"].(map[string]any)
				assert.Equal(t, "failed", job["state"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, cc, ac, sc, ms := tt.setupRequest(t)

			handler := newHugoTestHandler(cc, ac, sc, ms)

			w := httptest.NewRecorder()
			handler.Import(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			var resp map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

			if tt.expectedStatus != http.StatusAccepted {
				if tt.validateResp != nil {
					tt.validateResp(t, resp)
				}
				return
			}

			// Async flow: extract job ID and poll until terminal.
			jobID := resp["data"].(map[string]any)["jobId"].(string)
			require.NotEmpty(t, jobID)

			if !tt.expectJobDone {
				return
			}

			finalResp := pollJobStatus(t, handler, jobID, "/admin/hugo/import/status/")
			if tt.validateResp != nil {
				tt.validateResp(t, finalResp)
			}
		})
	}
}

func TestHugoHandler_ImportStatus(t *testing.T) {
	tests := []struct {
		name           string
		jobID          string
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:           "unknown job ID returns not found",
			jobID:          "/unknown-job",
			expectedStatus: http.StatusNotFound,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "not_found", err["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := hugomocks.NewMockContentCreator(t)
			ac := hugomocks.NewMockAliasCreator(t)
			sc := hugomocks.NewMockSlugResolver(t)
			ms := hugomocks.NewMockMediaService(t)
			handler := newHugoTestHandler(cc, ac, sc, ms)

			req := httptest.NewRequest(http.MethodGet, "/admin/hugo/import/status"+tt.jobID, nil)
			w := httptest.NewRecorder()
			handler.ImportStatus(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)
			var resp map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}
