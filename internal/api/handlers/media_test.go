package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	handlersmocks "github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/domain/media"
	mediamocks "github.com/aristorinjuang/lesstruct/internal/domain/media/mocks"
	"os"

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
				CreatedAt:        "2026-05-27T10:00:00Z",
				UpdatedAt:        "2026-05-27T10:00:00Z",
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

func TestMediaHandler_GetMedia_Variants(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "list with mixed variants",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := util.NewLogger(os.Stdout)

			mediaList := []*media.Media{
				{
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
					Variants: map[string]media.MediaVariant{
						"_thumb": {
							FilePath: "data/uploads/media/abc123def456789a_thumb.webp",
							URL:      "http://localhost:8080/uploads/media/abc123def456789a_thumb.webp",
							Width:    370,
							Height:   247,
						},
					},
					UploadedBy: "Test User",
					CreatedAt:  "2026-05-27T10:00:00Z",
					UpdatedAt:  "2026-05-27T10:00:00Z",
				},
				{
					ID:               2,
					UserID:           1,
					Filename:         "def456abc7890.webp",
					OriginalFilename: "mountain.jpg",
					MimeType:         media.MimeTypeWebP,
					FileSize:         30000,
					Width:            1024,
					Height:           768,
					AltText:          "A mountain view",
					IsWebP:           true,
					FilePath:         "data/uploads/media/def456abc7890.webp",
					URL:              "http://localhost:8080/uploads/media/def456abc7890.webp",
					Hash:             "def456abc7890",
					Variants:         map[string]media.MediaVariant{},
					UploadedBy:       "Test User 2",
					CreatedAt:        "2026-05-26T10:00:00Z",
					UpdatedAt:        "2026-05-26T10:00:00Z",
				},
			}

			mockService := handlersmocks.NewMockMediaServiceInterface(t)
			mockService.EXPECT().SearchMedia(
				mock.Anything,
				"",
				"",
				100,
				0,
			).Return(mediaList, nil)

			handler := handlers.NewMediaHandler(mockService, nil, logger)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/media", nil)
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "1"))

			w := httptest.NewRecorder()
			handler.GetMedia(w, req)

			require.Equal(t, http.StatusOK, w.Code)

			var resp map[string]any
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)

			data, ok := resp["data"].(map[string]any)
			require.True(t, ok)

			mediaArr, ok := data["media"].([]any)
			require.True(t, ok)
			require.Len(t, mediaArr, 2)

			first, ok := mediaArr[0].(map[string]any)
			require.True(t, ok)

			variants1, ok := first["variants"].(map[string]any)
			require.True(t, ok)

			thumb, ok := variants1["_thumb"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "http://localhost:8080/uploads/media/abc123def456789a_thumb.webp", thumb["url"])
			assert.Equal(t, float64(370), thumb["width"])
			assert.Equal(t, float64(247), thumb["height"])

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