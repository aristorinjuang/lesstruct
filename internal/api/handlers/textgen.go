package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

const maxTextGenPromptLength = 50000

// textGenEnhanceRequest is the JSON body for the Enhance endpoint.
type textGenEnhanceRequest struct {
	Content string `json:"content"`
}

// textGenTranslateRequest is the JSON body for the Translate endpoint.
type textGenTranslateRequest struct {
	Content    string `json:"content"`
	SourceLang string `json:"sourceLang"`
	TargetLang string `json:"targetLang"`
}

// textGenResponse is the JSON body for text generation responses.
type textGenResponse struct {
	Content string `json:"content"`
}

// TextGenerationService defines the interface for AI text generation.
type TextGenerationService interface {
	EnhanceText(ctx context.Context, content string) (string, error)
	TranslateText(ctx context.Context, content, sourceLang, targetLang string) (string, error)
}

// TextGenHandler handles AI text generation requests.
type TextGenHandler struct {
	textGenService TextGenerationService
	logger         *util.Logger
}

// Enhance handles the "Enhance with AI" endpoint.
func (h *TextGenHandler) Enhance(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetUserID(r); !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	var req textGenEnhanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode enhance request body: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content", "Content is required", nil)
		return
	}
	if utf8.RuneCountInString(content) > maxTextGenPromptLength {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content", "Content is too long", nil)
		return
	}

	// Validate that the content is valid JSON
	var parsed any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content", "Content must be valid JSON", nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Minute)
	defer cancel()

	enhanced, err := h.textGenService.EnhanceText(ctx, content)
	if err != nil {
		h.logger.Error("Failed to enhance text: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "generation_failed", "Failed to enhance content", nil)
		return
	}

	sendSuccessResponse(w, http.StatusOK, &textGenResponse{Content: enhanced})
}

// Translate handles the "Translate with AI" endpoint.
func (h *TextGenHandler) Translate(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetUserID(r); !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	var req textGenTranslateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode translate request body: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content", "Content is required", nil)
		return
	}
	if utf8.RuneCountInString(content) > maxTextGenPromptLength {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content", "Content is too long", nil)
		return
	}

	sourceLang := strings.TrimSpace(req.SourceLang)
	if sourceLang == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_source_lang", "Source language is required", nil)
		return
	}

	targetLang := strings.TrimSpace(req.TargetLang)
	if targetLang == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_target_lang", "Target language is required", nil)
		return
	}

	// Validate that the content is valid JSON
	var parsed any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content", "Content must be valid JSON", nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Minute)
	defer cancel()

	translated, err := h.textGenService.TranslateText(ctx, content, sourceLang, targetLang)
	if err != nil {
		h.logger.Error("Failed to translate text: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "generation_failed", "Failed to translate content", nil)
		return
	}

	sendSuccessResponse(w, http.StatusOK, &textGenResponse{Content: translated})
}

// NewTextGenHandler creates a new TextGenHandler.
func NewTextGenHandler(textGenService TextGenerationService, logger *util.Logger) *TextGenHandler {
	return &TextGenHandler{
		textGenService: textGenService,
		logger:         logger,
	}
}
