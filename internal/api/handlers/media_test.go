package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	handlersmocks "github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	appresponse "github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/domain/media"
	mediamocks "github.com/aristorinjuang/lesstruct/internal/domain/media/mocks"
	roledomain "github.com/aristorinjuang/lesstruct/internal/domain/role"

	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func createMultipartFormData(t *testing.T, fieldName, filename string, fileData []byte) (*bytes.Buffer, string) {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile(fieldName, filename)
	require.NoError(t, err)

	_, err = part.Write(fileData)
	require.NoError(t, err)

	err = writer.WriteField("alt_text", "Test image")
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	return &buf, writer.FormDataContentType()
}

func TestMediaHandler_Upload_Duplicate(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "duplicate upload returns existing media",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := util.NewLogger(os.Stdout)

			existingMedia := &media.Media{
				ID:               42,
				UserID:           1,
				Filename:         "abc123def4567890.webp",
				OriginalFilename: "sunset.jpg",
				URL:              "http://localhost:8080/uploads/media/abc123def4567890.webp",
				Hash:             "sha256hash",
			}

			dupErr := &media.DuplicateMediaError{Existing: existingMedia}

			mockService := handlersmocks.NewMockMediaServiceInterface(t)
			mockService.EXPECT().Upload(
				mock.Anything,
				mock.AnythingOfType("media.UploadRequest"),
			).Return((*media.Media)(nil), dupErr)

			handler := handlers.NewMediaHandler(mockService, nil, logger)

			imgData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
			body, contentType := createMultipartFormData(t, "image", "sunset.jpg", imgData)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/media/upload", body)
			req.Header.Set("Content-Type", contentType)

			ctx := chi.NewRouteContext()
			ctx.URLParams.Add("userID", "1")
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "1"))
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

			w := httptest.NewRecorder()
			handler.Upload(w, req)

			require.Equal(t, http.StatusOK, w.Code)

			var resp map[string]any
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)

			data, ok := resp["data"].(map[string]any)
			require.True(t, ok, "expected data field in response")
			assert.Equal(t, true, data["duplicate"])

			existing, ok := data["existingMedia"].(map[string]any)
			require.True(t, ok, "expected existingMedia field in response")
			assert.Equal(t, float64(42), existing["id"])
			assert.Equal(t, "sunset.jpg", existing["originalFilename"])
		})
	}
}

func TestMediaHandler_Upload_Force(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "force upload returns created status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := util.NewLogger(os.Stdout)

			forceMedia := &media.Media{
				ID:               43,
				UserID:           1,
				Filename:         "abc123def4567890_1.webp",
				OriginalFilename: "sunset-2.jpg",
				URL:              "http://localhost:8080/uploads/media/abc123def4567890_1.webp",
				Hash:             "sha256hash_1",
			}

			mockService := handlersmocks.NewMockMediaServiceInterface(t)
			mockService.EXPECT().ForceUpload(
				mock.Anything,
				mock.AnythingOfType("media.UploadRequest"),
			).Return(forceMedia, nil)

			handler := handlers.NewMediaHandler(mockService, nil, logger)

			imgData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
			body, contentType := createMultipartFormData(t, "image", "sunset.jpg", imgData)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/media/upload?force=true", body)
			req.Header.Set("Content-Type", contentType)

			ctx := chi.NewRouteContext()
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "1"))
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

			w := httptest.NewRecorder()
			handler.Upload(w, req)

			require.Equal(t, http.StatusCreated, w.Code)
		})
	}
}

func TestMediaHandler_Upload_ForceInvalidParam(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "invalid force param falls back to normal upload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := util.NewLogger(os.Stdout)

			uploadedMedia := &media.Media{
				ID:               44,
				UserID:           1,
				Filename:         "abc123def4567890.webp",
				OriginalFilename: "photo.jpg",
				URL:              "http://localhost:8080/uploads/media/abc123def4567890.webp",
			}

			mockService := handlersmocks.NewMockMediaServiceInterface(t)
			mockService.EXPECT().Upload(
				mock.Anything,
				mock.AnythingOfType("media.UploadRequest"),
			).Return(uploadedMedia, nil)

			handler := handlers.NewMediaHandler(mockService, nil, logger)

			imgData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
			body, contentType := createMultipartFormData(t, "image", "photo.jpg", imgData)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/media/upload?force=yes", body)
			req.Header.Set("Content-Type", contentType)

			ctx := chi.NewRouteContext()
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "1"))
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

			w := httptest.NewRecorder()
			handler.Upload(w, req)

			require.Equal(t, http.StatusCreated, w.Code)

			var resp map[string]any
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)

			data, ok := resp["data"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "photo.jpg", data["originalFilename"])
		})
	}
}

func TestMediaHandler_Upload_OtherErrors(t *testing.T) {
	tests := []struct {
		name           string
		serviceError   error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "invalid file",
			serviceError:   media.ErrInvalidFile,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "invalid_file",
		},
		{
			name:           "file too large",
			serviceError:   media.ErrFileTooLarge,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "file_too_large",
		},
		{
			name:           "invalid alt text",
			serviceError:   media.ErrInvalidAltText,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "invalid_alt_text",
		},
		{
			name:           "media not found",
			serviceError:   media.ErrMediaNotFound,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "media_not_found",
		},
		{
			name:           "unauthorized",
			serviceError:   media.ErrUnauthorized,
			expectedStatus: http.StatusForbidden,
			expectedCode:   "forbidden",
		},
		{
			name:           "internal error",
			serviceError:   errors.New("internal error"),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := util.NewLogger(os.Stdout)

			mockService := handlersmocks.NewMockMediaServiceInterface(t)
			mockService.EXPECT().Upload(
				mock.Anything,
				mock.AnythingOfType("media.UploadRequest"),
			).Return((*media.Media)(nil), tt.serviceError)

			handler := handlers.NewMediaHandler(mockService, nil, logger)

			imgData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
			body, contentType := createMultipartFormData(t, "image", "test.png", imgData)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/media/upload", body)
			req.Header.Set("Content-Type", contentType)

			ctx := chi.NewRouteContext()
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "1"))
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

			w := httptest.NewRecorder()
			handler.Upload(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			var resp map[string]any
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)

			errObj, ok := resp["error"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tt.expectedCode, errObj["code"])
		})
	}
}

func TestMediaHandler_GetMediaByID_Variants(t *testing.T) {
	tests := []struct {
		name     string
		variants map[string]media.MediaVariant
		wantErr  bool
	}{
		{
			name: "single variant",
			variants: map[string]media.MediaVariant{
				"_thumb": {
					FilePath: "data/uploads/media/abc123def456789a_thumb.webp",
					URL:      "http://localhost:8080/uploads/media/abc123def456789a_thumb.webp",
					Width:    370,
					Height:   247,
				},
			},
			wantErr: false,
		},
		{
			name: "multiple variants",
			variants: map[string]media.MediaVariant{
				"_thumb": {
					FilePath: "data/uploads/media/abc123def456789a_thumb.webp",
					URL:      "http://localhost:8080/uploads/media/abc123def456789a_thumb.webp",
					Width:    370,
					Height:   247,
				},
				"_medium": {
					FilePath: "data/uploads/media/abc123def456789a_medium.webp",
					URL:      "http://localhost:8080/uploads/media/abc123def456789a_medium.webp",
					Width:    768,
					Height:   512,
				},
			},
			wantErr: false,
		},
		{
			name:     "empty variants",
			variants: map[string]media.MediaVariant{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := util.NewLogger(os.Stdout)

			mediaItem := &media.Media{
				ID:               1,
				UserID:           1,
				Filename:         "abc123def456789a.webp",
				OriginalFilename: "sunset.jpg",
				MimeType:         media.MimeTypeWebP,
				FileSize:         50000,
				Width:            1920,
				Height:           1280,
				AltText:          "A beautiful sunset",
				IsWebP:           true,
				FilePath:         "data/uploads/media/abc123def456789a.webp",
				URL:              "http://localhost:8080/uploads/media/abc123def456789a.webp",
				Hash:             "abc123def456789a",
				Variants:         tt.variants,
				UploadedBy:       "Test User",
				CreatedAt:        time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC),
				UpdatedAt:        time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC),
			}

			mockService := handlersmocks.NewMockMediaServiceInterface(t)
			mockService.EXPECT().GetByID(mock.Anything, 1).Return(mediaItem, nil)

			handler := handlers.NewMediaHandler(mockService, nil, logger)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/media/1", nil)
			req.SetPathValue("id", "1")
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "1"))

			w := httptest.NewRecorder()
			handler.GetMediaByID(w, req)

			require.Equal(t, http.StatusOK, w.Code)

			var resp map[string]any
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)

			data, ok := resp["data"].(map[string]any)
			require.True(t, ok)

			variants, ok := data["variants"].(map[string]any)
			require.True(t, ok)

			if len(tt.variants) == 0 {
				assert.Empty(t, variants)
				return
			}

			for suffix, expected := range tt.variants {
				v, ok := variants[suffix].(map[string]any)
				require.True(t, ok, "expected variant %q in response", suffix)
				assert.Equal(t, expected.URL, v["url"])
				assert.Equal(t, float64(expected.Width), v["width"])
				assert.Equal(t, float64(expected.Height), v["height"])
			}
		})
	}
}

func TestMediaHandler_GetMedia(t *testing.T) {
	makeMedia := func(id int) *media.Media {
		return &media.Media{
			ID:               id,
			UserID:           1,
			Filename:         "media" + strconv.Itoa(id) + ".webp",
			OriginalFilename: "image" + strconv.Itoa(id) + ".jpg",
			MimeType:         media.MimeTypeWebP,
			FileSize:         50000,
			Width:            1920,
			Height:           1280,
			AltText:          "Image " + strconv.Itoa(id),
			IsWebP:           true,
			FilePath:         "data/uploads/media/media" + strconv.Itoa(id) + ".webp",
			URL:              "http://localhost:8080/uploads/media/media" + strconv.Itoa(id) + ".webp",
			Hash:             "hash-" + strconv.Itoa(id),
			Variants:         map[string]media.MediaVariant{},
			UploadedBy:       "Test User",
			CreatedAt:        time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC),
			UpdatedAt:        time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC),
		}
	}

	tests := []struct {
		name           string
		target         string
		mockSearch     string
		mockDateFilter string
		mockLimit      int
		mockBeforeID   int
		mediaList      []*media.Media
		mockTotal      int
		wantTotal      int
		wantCount      int
		wantHasMore    bool
		wantNextCursor int
		wantCode       string
	}{
		{
			name:           "list with mixed variants returns bare array without pagination",
			target:         "/api/v1/media",
			mockSearch:     "",
			mockDateFilter: "",
			mockLimit:      51,
			mockBeforeID:   0,
			mediaList: func() []*media.Media {
				first := makeMedia(1)
				first.Filename = "abc123def456789a.webp"
				first.OriginalFilename = "sunset.jpg"
				first.Hash = "abc123def456789a"
				first.FilePath = "data/uploads/media/abc123def456789a.webp"
				first.URL = "http://localhost:8080/uploads/media/abc123def456789a.webp"
				first.Variants = map[string]media.MediaVariant{
					"_thumb": {
						FilePath: "data/uploads/media/abc123def456789a_thumb.webp",
						URL:      "http://localhost:8080/uploads/media/abc123def456789a_thumb.webp",
						Width:    370,
						Height:   247,
					},
				}
				return []*media.Media{first, makeMedia(2)}
			}(),
			wantHasMore: false,
		},
		{
			name:           "paginated list returns nextCursor and hasMore",
			target:         "/api/v1/media?limit=10&cursor=" + appresponse.EncodeCursor(20),
			mockSearch:     "",
			mockDateFilter: "",
			mockLimit:      11,
			mockBeforeID:   20,
			mediaList: func() []*media.Media {
				items := make([]*media.Media, 0, 11)
				for id := 20; id >= 10; id-- {
					items = append(items, makeMedia(id))
				}
				return items
			}(),
			wantCount:      10,
			wantHasMore:    true,
			wantNextCursor: 11,
			mockTotal:      42,
			wantTotal:      42,
		},
		{
			name:           "search and date filter are forwarded",
			target:         "/api/v1/media?search=sunset&date_filter=today",
			mockSearch:     "sunset",
			mockDateFilter: "today",
			mockLimit:      51,
			mockBeforeID:   0,
			mediaList:      []*media.Media{},
			mockTotal:      3,
			wantTotal:      3,
			wantHasMore:    false,
		},
		{
			name:     "invalid cursor returns 400",
			target:   "/api/v1/media?cursor=not-a-cursor",
			wantCode: "invalid_cursor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := util.NewLogger(os.Stdout)

			mockService := handlersmocks.NewMockMediaServiceInterface(t)
			if tt.wantCode == "" {
				mockService.EXPECT().SearchMediaByCursor(
					mock.Anything,
					tt.mockSearch,
					tt.mockDateFilter,
					tt.mockLimit,
					tt.mockBeforeID,
				).Return(tt.mediaList, nil)
				mockService.EXPECT().Count(
					mock.Anything,
					tt.mockSearch,
					tt.mockDateFilter,
				).Return(tt.mockTotal, nil)
			}

			handler := handlers.NewMediaHandler(mockService, nil, logger)

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "1"))

			w := httptest.NewRecorder()
			handler.GetMedia(w, req)

			if tt.wantCode != "" {
				require.Equal(t, http.StatusBadRequest, w.Code)

				var errResp map[string]any
				require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
				errorInfo, ok := errResp["error"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, tt.wantCode, errorInfo["code"])
				return
			}

			require.Equal(t, http.StatusOK, w.Code)

			var resp map[string]any
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)

			mediaArr, ok := resp["data"].([]any)
			require.True(t, ok, "data must be a bare JSON array")
			wantCount := tt.wantCount
			if wantCount == 0 {
				wantCount = len(tt.mediaList)
			}
			require.Len(t, mediaArr, wantCount)

			meta, ok := resp["meta"].(map[string]any)
			require.True(t, ok)
			pagination, ok := meta["pagination"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tt.wantHasMore, pagination["hasMore"], "hasMore")
			if tt.wantTotal > 0 {
				total, ok := pagination["total"].(float64)
				require.True(t, ok, "total must be present")
				assert.Equal(t, float64(tt.wantTotal), total, "total")
			}

			if tt.wantHasMore {
				nextCursor, ok := pagination["nextCursor"].(string)
				require.True(t, ok)
				nextID, err := appresponse.DecodeCursor(nextCursor)
				require.NoError(t, err)
				assert.Equal(t, tt.wantNextCursor, nextID, "nextCursor id")
			}

			if len(tt.mediaList) == 0 {
				return
			}

			first, ok := mediaArr[0].(map[string]any)
			require.True(t, ok)

			variants1, ok := first["variants"].(map[string]any)
			require.True(t, ok)

			thumb, ok := variants1["_thumb"].(map[string]any)
			if tt.mediaList[0].OriginalFilename == "sunset.jpg" {
				require.True(t, ok)
				assert.Equal(t, "http://localhost:8080/uploads/media/abc123def456789a_thumb.webp", thumb["url"])
				assert.Equal(t, float64(370), thumb["width"])
				assert.Equal(t, float64(247), thumb["height"])
			}

			second, ok := mediaArr[1].(map[string]any)
			require.True(t, ok)

			variants2, ok := second["variants"].(map[string]any)
			require.True(t, ok)
			assert.Empty(t, variants2)
		})
	}
}

func TestMediaHandler_GenerateImage(t *testing.T) {
	tests := []struct {
		name              string
		setupImageGen     bool
		requestBody       string
		mockGenerateError error
		mockSaveError     error
		expectedStatus    int
		expectedCode      string
	}{
		{
			name:           "success - generates and saves image",
			setupImageGen:  true,
			requestBody:    `{"prompt":"A beautiful sunset"}`,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "error - no image gen service configured",
			setupImageGen:  false,
			requestBody:    `{"prompt":"A beautiful sunset"}`,
			expectedStatus: http.StatusServiceUnavailable,
			expectedCode:   "not_configured",
		},
		{
			name:           "error - empty prompt",
			setupImageGen:  true,
			requestBody:    `{"prompt":""}`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "invalid_prompt",
		},
		{
			name:           "error - missing prompt field",
			setupImageGen:  true,
			requestBody:    `{}`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "invalid_prompt",
		},
		{
			name:           "error - invalid JSON",
			setupImageGen:  true,
			requestBody:    `not json`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "invalid_request",
		},
		{
			name:              "error - generation fails",
			setupImageGen:     true,
			requestBody:       `{"prompt":"fail"}`,
			mockGenerateError: errors.New("API error"),
			expectedStatus:    http.StatusInternalServerError,
			expectedCode:      "generation_failed",
		},
		{
			name:          "error - save fails",
			setupImageGen: true,
			requestBody:   `{"prompt":"duplicate image"}`,
			mockSaveError: &media.DuplicateMediaError{Existing: &media.Media{ID: 99}},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := util.NewLogger(os.Stdout)

			mockService := handlersmocks.NewMockMediaServiceInterface(t)

			var imageGenService media.ImageGenerationService
			if tt.setupImageGen {
				mockImageGen := mediamocks.NewMockImageGenerationService(t)
				if tt.mockGenerateError != nil {
					mockImageGen.EXPECT().GenerateImage(
						mock.Anything,
						mock.Anything,
					).Return(([]byte)(nil), tt.mockGenerateError)
				} else if tt.requestBody == `{"prompt":"A beautiful sunset"}` || tt.requestBody == `{"prompt":"duplicate image"}` {
					mockImageGen.EXPECT().GenerateImage(
						mock.Anything,
						mock.Anything,
					).Return([]byte{0x89, 0x50, 0x4E, 0x47}, nil)
					mockService.EXPECT().GenerateFromBytes(
						mock.Anything,
						mock.Anything,
						mock.Anything,
						mock.Anything,
						mock.Anything,
					).Return(&media.Media{
						ID:               100,
						OriginalFilename: "ai-generated-20260605-120000.webp",
						URL:              "http://localhost:8080/uploads/media/abc123.webp",
						AltText:          "A beautiful sunset",
					}, tt.mockSaveError)
				}
				imageGenService = mockImageGen
			}

			handler := handlers.NewMediaHandler(mockService, imageGenService, logger)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/media/generate", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "1"))

			w := httptest.NewRecorder()
			handler.GenerateImage(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" && tt.expectedCode != "duplicate_media" {
				var resp map[string]any
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				errObj, ok := resp["error"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, tt.expectedCode, errObj["code"])
			}
		})
	}
}

// TestMediaHandler_RoleGate verifies media endpoints are gated by the caller's
// role capability when a role service is configured. A role without the media
// capability gets 403 on Upload, DeleteMedia, and GenerateImage; without a role
// service every authenticated user may (legacy behavior, covered by the
// existing tests).
func TestMediaHandler_RoleGate(t *testing.T) {
	roleService := roledomain.NewService()
	if err := roleService.Register(roledomain.Role{
		Name:      "Editor",
		PostTypes: []string{"article"},
		Publish:   true,
		Media:     false,
		Comments:  true,
	}); err != nil {
		t.Fatalf("failed to register role: %v", err)
	}

	roleServiceWithMedia := roledomain.NewService()
	if err := roleServiceWithMedia.Register(roledomain.Role{
		Name:      "Editor",
		PostTypes: []string{"article"},
		Publish:   true,
		Media:     true,
		Comments:  true,
	}); err != nil {
		t.Fatalf("failed to register role: %v", err)
	}

	tests := []struct {
		name           string
		endpoint       func(*testing.T, *handlers.MediaHandler) int
		withRoleSvc    bool
		roleSvc        *roledomain.Service
		role           string
		expectedStatus int
	}{
		{
			name: "upload - role cannot manage media",
			endpoint: func(t *testing.T, h *handlers.MediaHandler) int {
				imgData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
				body, contentType := createMultipartFormData(t, "image", "sunset.jpg", imgData)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/media/upload", body)
				req.Header.Set("Content-Type", contentType)
				req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "1"))
				req = req.WithContext(context.WithValue(req.Context(), middleware.RoleKey, "Editor"))
				w := httptest.NewRecorder()
				h.Upload(w, req)
				return w.Code
			},
			withRoleSvc:    true,
			roleSvc:        roleService,
			role:           "Editor",
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "upload - role can manage media",
			endpoint: func(t *testing.T, h *handlers.MediaHandler) int {
				imgData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
				body, contentType := createMultipartFormData(t, "image", "sunset.jpg", imgData)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/media/upload", body)
				req.Header.Set("Content-Type", contentType)
				req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "1"))
				req = req.WithContext(context.WithValue(req.Context(), middleware.RoleKey, "Editor"))
				w := httptest.NewRecorder()
				h.Upload(w, req)
				return w.Code
			},
			withRoleSvc:    true,
			roleSvc:        roleServiceWithMedia,
			role:           "Editor",
			expectedStatus: http.StatusCreated,
		},
		{
			name: "generate - role cannot manage media",
			endpoint: func(t *testing.T, h *handlers.MediaHandler) int {
				req := httptest.NewRequest(http.MethodPost, "/api/v1/media/generate", bytes.NewBufferString(`{"prompt":"a sunset"}`))
				req.Header.Set("Content-Type", "application/json")
				req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "1"))
				req = req.WithContext(context.WithValue(req.Context(), middleware.RoleKey, "Editor"))
				w := httptest.NewRecorder()
				h.GenerateImage(w, req)
				return w.Code
			},
			withRoleSvc:    true,
			roleSvc:        roleService,
			role:           "Editor",
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "delete - role cannot manage media",
			endpoint: func(t *testing.T, h *handlers.MediaHandler) int {
				req := httptest.NewRequest(http.MethodDelete, "/api/v1/media/1", nil)
				req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "1"))
				req = req.WithContext(context.WithValue(req.Context(), middleware.RoleKey, "Editor"))
				w := httptest.NewRecorder()
				h.DeleteMedia(w, req)
				return w.Code
			},
			withRoleSvc:    true,
			roleSvc:        roleService,
			role:           "Editor",
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "upload - no role service allows legacy",
			endpoint: func(t *testing.T, h *handlers.MediaHandler) int {
				imgData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
				body, contentType := createMultipartFormData(t, "image", "sunset.jpg", imgData)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/media/upload", body)
				req.Header.Set("Content-Type", contentType)
				req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "1"))
				w := httptest.NewRecorder()
				h.Upload(w, req)
				return w.Code
			},
			withRoleSvc:    false,
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockMediaServiceInterface(t)
			if tt.expectedStatus == http.StatusCreated {
				if tt.name == "upload - role can manage media" || tt.name == "upload - no role service allows legacy" {
					mockService.EXPECT().Upload(
						mock.Anything,
						mock.AnythingOfType("media.UploadRequest"),
					).Return(&media.Media{
						ID:               1,
						UserID:           1,
						Filename:         "abc.webp",
						OriginalFilename: "sunset.jpg",
						URL:              "http://localhost:8080/uploads/media/abc.webp",
						Hash:             "sha256hash",
					}, nil)
				}
			}

			var opts []handlers.MediaHandlerOption
			if tt.withRoleSvc {
				opts = append(opts, handlers.WithMediaRoleService(tt.roleSvc))
			}
			handler := handlers.NewMediaHandler(mockService, nil, util.NewLogger(os.Stdout), opts...)

			status := tt.endpoint(t, handler)
			require.Equal(t, tt.expectedStatus, status)
		})
	}
}