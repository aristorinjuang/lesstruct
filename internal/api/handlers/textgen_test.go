package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	handlersmocks "github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func validTipTapContentJSON() string {
	return `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello world"}]}]}`
}

func buildEnhanceBody(content string) []byte {
	body := map[string]string{"content": content}
	b, _ := json.Marshal(body)
	return b
}

func buildEnhanceBodyWithFormat(content, format string) []byte {
	body := map[string]string{"content": content, "format": format}
	b, _ := json.Marshal(body)
	return b
}

func buildEnhanceBodyWithExisting(content, format, existingHTML string) []byte {
	body := map[string]string{"content": content, "format": format, "existingHtml": existingHTML}
	b, _ := json.Marshal(body)
	return b
}

func buildTranslateBody(content, sourceLang, targetLang string) []byte {
	body := map[string]string{
		"content":    content,
		"sourceLang": sourceLang,
		"targetLang": targetLang,
	}
	b, _ := json.Marshal(body)
	return b
}

func buildTranslateBodyWithFormat(content, sourceLang, targetLang, format string) []byte {
	body := map[string]string{
		"content":    content,
		"sourceLang": sourceLang,
		"targetLang": targetLang,
		"format":     format,
	}
	b, _ := json.Marshal(body)
	return b
}

func TestTextGenHandler_Enhance(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    []byte
		setupService   func(s *handlersmocks.MockTextGenerationService)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:        "successful enhance tiptap",
			requestBody: buildEnhanceBody(validTipTapContentJSON()),
			setupService: func(s *handlersmocks.MockTextGenerationService) {
				s.EXPECT().EnhanceText(
					mock.Anything,
					validTipTapContentJSON(),
					"tiptap",
					"",
				).Return(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Enhanced hello world"}]}]}`, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				require.True(t, ok, "expected data field")
				require.NotNil(t, data["content"])
			},
		},
		{
			name:        "successful enhance html",
			requestBody: buildEnhanceBodyWithFormat("<p>Hello</p>", "html"),
			setupService: func(s *handlersmocks.MockTextGenerationService) {
				s.EXPECT().EnhanceText(
					mock.Anything,
					"<p>Hello</p>",
					"html",
					"",
				).Return("<style>.ls-test{color:red}</style><section class='ls-test'>Hello</section>", nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				require.True(t, ok, "expected data field")
				require.NotNil(t, data["content"])
			},
		},
		{
			name:           "invalid request body",
			requestBody:    []byte(`not json`),
			setupService:   func(s *handlersmocks.MockTextGenerationService) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok, "expected error field")
				require.Equal(t, "invalid_request", err["code"])
			},
		},
		{
			name:           "empty content",
			requestBody:    buildEnhanceBody(""),
			setupService:   func(s *handlersmocks.MockTextGenerationService) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok, "expected error field")
				require.Equal(t, "invalid_content", err["code"])
			},
		},
		{
			name:           "whitespace only content",
			requestBody:    buildEnhanceBody("   "),
			setupService:   func(s *handlersmocks.MockTextGenerationService) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok, "expected error field")
				require.Equal(t, "invalid_content", err["code"])
			},
		},
		{
			name:        "content not valid json for tiptap",
			requestBody: buildEnhanceBody("this is not json"),
			setupService: func(s *handlersmocks.MockTextGenerationService) {
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok, "expected error field")
				require.Equal(t, "invalid_content", err["code"])
			},
		},
		{
			name:        "service returns error",
			requestBody: buildEnhanceBody(validTipTapContentJSON()),
			setupService: func(s *handlersmocks.MockTextGenerationService) {
				s.EXPECT().EnhanceText(
					mock.Anything,
					validTipTapContentJSON(),
					"tiptap",
					"",
				).Return("", context.DeadlineExceeded)
			},
			expectedStatus: http.StatusInternalServerError,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok, "expected error field")
				require.Equal(t, "generation_failed", err["code"])
			},
		},
		{
			name:        "content too long",
			requestBody: buildEnhanceBody(strings.Repeat("a", 50001)),
			setupService: func(s *handlersmocks.MockTextGenerationService) {
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok, "expected error field")
				require.Equal(t, "invalid_content", err["code"])
			},
		},
		{
			name:           "invalid format",
			requestBody:    buildEnhanceBodyWithFormat("content", "xml"),
			setupService:   func(s *handlersmocks.MockTextGenerationService) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok, "expected error field")
				require.Equal(t, "invalid_format", err["code"])
			},
		},
		{
			name:        "html format allows non-json content",
			requestBody: buildEnhanceBodyWithFormat("A hero section with a gradient background", "html"),
			setupService: func(s *handlersmocks.MockTextGenerationService) {
				s.EXPECT().EnhanceText(
					mock.Anything,
					"A hero section with a gradient background",
					"html",
					"",
				).Return("<style>.ls-hero{background:linear-gradient(135deg,#fafafa,#f0f0f5)}</style><section class='ls-hero'>Hello</section>", nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				require.True(t, ok, "expected data field")
				require.NotNil(t, data["content"])
			},
		},
		{
			name:        "html format with existing html for refinement",
			requestBody: buildEnhanceBodyWithExisting("Make the colors warmer", "html", "<section class='ls-test'>Hello</section>"),
			setupService: func(s *handlersmocks.MockTextGenerationService) {
				s.EXPECT().EnhanceText(
					mock.Anything,
					"User instruction:\nMake the colors warmer\n\nExisting HTML to refine — modify it according to the instruction, keep what works, output the complete updated fragment:\n<section class='ls-test'>Hello</section>",
					"html",
					"",
				).Return("<style>.ls-test{color:orange}</style><section class='ls-test'>Hello</section>", nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				require.True(t, ok, "expected data field")
				require.NotNil(t, data["content"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockTextGenerationService(t)
			tt.setupService(mockService)

			handler := handlers.NewTextGenHandler(mockService, nil, util.NewLogger(os.Stdout))

			req := httptest.NewRequest(http.MethodPost, "/api/v1/text/enhance", bytes.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.Enhance(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			var resp map[string]any
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}

func TestTextGenHandler_Enhance_Unauthorized(t *testing.T) {
	mockService := handlersmocks.NewMockTextGenerationService(t)

	handler := handlers.NewTextGenHandler(mockService, nil, util.NewLogger(os.Stdout))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/text/enhance", bytes.NewReader(buildEnhanceBody(validTipTapContentJSON())))
	req.Header.Set("Content-Type", "application/json")

	// No user ID in context

	w := httptest.NewRecorder()
	handler.Enhance(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]any
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	errField, ok := resp["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "unauthorized", errField["code"])
}

func TestTextGenHandler_Translate(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    []byte
		setupService   func(s *handlersmocks.MockTextGenerationService)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:        "successful translate tiptap",
			requestBody: buildTranslateBody(validTipTapContentJSON(), "en", "fr"),
			setupService: func(s *handlersmocks.MockTextGenerationService) {
				s.EXPECT().TranslateText(
					mock.Anything,
					validTipTapContentJSON(),
					"en",
					"fr",
					"tiptap",
				).Return(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Bonjour le monde"}]}]}`, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				require.True(t, ok, "expected data field")
				require.NotNil(t, data["content"])
			},
		},
		{
			name:        "successful translate html",
			requestBody: buildTranslateBodyWithFormat("<section>Hello world</section>", "en", "fr", "html"),
			setupService: func(s *handlersmocks.MockTextGenerationService) {
				s.EXPECT().TranslateText(
					mock.Anything,
					"<section>Hello world</section>",
					"en",
					"fr",
					"html",
				).Return("<section>Bonjour le monde</section>", nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				require.True(t, ok, "expected data field")
				require.NotNil(t, data["content"])
			},
		},
		{
			name:           "invalid request body",
			requestBody:    []byte(`not json`),
			setupService:   func(s *handlersmocks.MockTextGenerationService) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok, "expected error field")
				require.Equal(t, "invalid_request", err["code"])
			},
		},
		{
			name:           "empty content",
			requestBody:    buildTranslateBody("", "en", "fr"),
			setupService:   func(s *handlersmocks.MockTextGenerationService) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok, "expected error field")
				require.Equal(t, "invalid_content", err["code"])
			},
		},
		{
			name:           "empty source language",
			requestBody:    buildTranslateBody(validTipTapContentJSON(), "", "fr"),
			setupService:   func(s *handlersmocks.MockTextGenerationService) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok, "expected error field")
				require.Equal(t, "invalid_source_lang", err["code"])
			},
		},
		{
			name:           "empty target language",
			requestBody:    buildTranslateBody(validTipTapContentJSON(), "en", ""),
			setupService:   func(s *handlersmocks.MockTextGenerationService) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok, "expected error field")
				require.Equal(t, "invalid_target_lang", err["code"])
			},
		},
		{
			name:        "service returns error",
			requestBody: buildTranslateBody(validTipTapContentJSON(), "en", "fr"),
			setupService: func(s *handlersmocks.MockTextGenerationService) {
				s.EXPECT().TranslateText(
					mock.Anything,
					validTipTapContentJSON(),
					"en",
					"fr",
					"tiptap",
				).Return("", context.DeadlineExceeded)
			},
			expectedStatus: http.StatusInternalServerError,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok, "expected error field")
				require.Equal(t, "generation_failed", err["code"])
			},
		},
		{
			name:        "content too long",
			requestBody: buildTranslateBody(strings.Repeat("a", 50001), "en", "fr"),
			setupService: func(s *handlersmocks.MockTextGenerationService) {
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok, "expected error field")
				require.Equal(t, "invalid_content", err["code"])
			},
		},
		{
			name:           "invalid format",
			requestBody:    buildTranslateBodyWithFormat("content", "en", "fr", "xml"),
			setupService:   func(s *handlersmocks.MockTextGenerationService) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				require.True(t, ok, "expected error field")
				require.Equal(t, "invalid_format", err["code"])
			},
		},
		{
			name:        "html format allows non-json content",
			requestBody: buildTranslateBodyWithFormat("<section>Hello world</section>", "en", "fr", "html"),
			setupService: func(s *handlersmocks.MockTextGenerationService) {
				s.EXPECT().TranslateText(
					mock.Anything,
					"<section>Hello world</section>",
					"en",
					"fr",
					"html",
				).Return("<section>Bonjour le monde</section>", nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				require.True(t, ok, "expected data field")
				require.NotNil(t, data["content"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockTextGenerationService(t)
			tt.setupService(mockService)

			handler := handlers.NewTextGenHandler(mockService, nil, util.NewLogger(os.Stdout))

			req := httptest.NewRequest(http.MethodPost, "/api/v1/text/translate", bytes.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.Translate(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			var resp map[string]any
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}

func TestTextGenHandler_Translate_Unauthorized(t *testing.T) {
	mockService := handlersmocks.NewMockTextGenerationService(t)

	handler := handlers.NewTextGenHandler(mockService, nil, util.NewLogger(os.Stdout))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/text/translate", bytes.NewReader(buildTranslateBody(validTipTapContentJSON(), "en", "fr")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.Translate(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]any
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	errField, ok := resp["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "unauthorized", errField["code"])
}

func TestNewTextGenHandler(t *testing.T) {
	mockService := handlersmocks.NewMockTextGenerationService(t)
	logger := util.NewLogger(os.Stdout)

	handler := handlers.NewTextGenHandler(mockService, nil, logger)
	require.NotNil(t, handler)
}
