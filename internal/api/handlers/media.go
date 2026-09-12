package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/domain/media"
	roledomain "github.com/aristorinjuang/lesstruct/internal/domain/role"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

const (
	mediaSearchParam     = "search"
	mediaDateFilterParam = "date_filter"
	maxAIPromptLength    = 1000
	// maxGenerateMultipartBytes caps AI image generation requests carrying
	// reference images (3 x 10MB references plus form overhead).
	maxGenerateMultipartBytes = 32 << 20
)

// generateImageRequest is the JSON body for the GenerateImage endpoint.
type generateImageRequest struct {
	Prompt string `json:"prompt"`
	OG     bool   `json:"og"`
}

// generateImageParams carries a decoded GenerateImage request.
type generateImageParams struct {
	prompt     string
	references []media.ImageReference
	og         bool
}

func handleMediaError(w http.ResponseWriter, err error) {
	if dupErr, ok := errors.AsType[*media.DuplicateMediaError](err); ok {
		sendSuccessResponse(w, http.StatusOK, map[string]any{
			"existingMedia": dupErr.Existing,
			"duplicate":     true,
		})
		return
	}

	statusCode := http.StatusInternalServerError
	code := "internal_error"
	message := "An internal error occurred"

	switch {
	case errors.Is(err, media.ErrInvalidFile), errors.Is(err, media.ErrInvalidMimeType), errors.Is(err, media.ErrInvalidFileContent):
		statusCode = http.StatusBadRequest
		code = "invalid_file"
		message = "Invalid file type. Please upload an image file (JPG, PNG, GIF, WebP)"
	case errors.Is(err, media.ErrFileTooLarge):
		statusCode = http.StatusBadRequest
		code = "file_too_large"
		message = "File size exceeds 10MB limit. Please upload a smaller image"
	case errors.Is(err, media.ErrInvalidAltText):
		statusCode = http.StatusBadRequest
		code = "invalid_alt_text"
		message = "Alt text is required and must be less than 500 characters"
	case errors.Is(err, media.ErrDuplicateMedia):
		statusCode = http.StatusConflict
		code = "duplicate_media"
		message = "Media already exists"
	case errors.Is(err, media.ErrMediaNotFound):
		statusCode = http.StatusNotFound
		code = "media_not_found"
		message = "Media not found"
	case errors.Is(err, media.ErrUnauthorized):
		statusCode = http.StatusForbidden
		code = "forbidden"
		message = "You are not authorized to perform this action"
	}

	sendErrorResponse(w, statusCode, code, message, nil)
}

type MediaServiceInterface interface {
	Upload(ctx context.Context, req media.UploadRequest) (*media.Media, error)
	ForceUpload(ctx context.Context, req media.UploadRequest) (*media.Media, error)
	GenerateFromBytes(ctx context.Context, imageBytes []byte, userID int, altText string, originalFilename string) (*media.Media, error)
	GetByID(ctx context.Context, id int) (*media.Media, error)
	Delete(ctx context.Context, id int, userID int, userRole string) error
	SearchMediaByCursor(ctx context.Context, search string, dateFilter string, limit int, beforeID int) ([]*media.Media, error)
	Count(ctx context.Context, search string, dateFilter string) (int, error)
}

// MediaHandlerOption configures a MediaHandler. A role service makes the media
// capabilities config-driven; without one every authenticated user may upload,
// generate, and delete media (the legacy behavior).
type MediaHandlerOption func(*MediaHandler)

// WithMediaRoleService attaches the config-driven role registry so upload, generate,
// and delete are gated by the caller's CanMedia capability.
func WithMediaRoleService(rs *roledomain.Service) MediaHandlerOption {
	return func(h *MediaHandler) {
		h.roleService = rs
	}
}

type MediaHandler struct {
	mediaService    MediaServiceInterface
	imageGenService media.ImageGenerationService
	logger          *util.Logger
	roleService     *roledomain.Service
}

// mediaAllowed reports whether the caller's role may manage media. Without a
// role service every authenticated user may, matching the legacy behavior.
func (h *MediaHandler) mediaAllowed(r *http.Request) bool {
	if h.roleService == nil {
		return true
	}
	role, ok := middleware.GetRole(r)
	if !ok {
		return false
	}
	return h.roleService.CanMedia(role)
}

func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID", nil)
		return
	}

	if !h.mediaAllowed(r) {
		sendErrorResponse(w, http.StatusForbidden, "forbidden", "Your role cannot manage media", nil)
		return
	}

	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		h.logger.Error("Failed to parse multipart form: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to parse form data", nil)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		h.logger.Error("Failed to get form file: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Image file is required", nil)
		return
	}
	defer func() { _ = file.Close() }()

	altText := r.FormValue("alt_text")

	req := media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     userID,
		AltText:    altText,
	}

	var uploadedMedia *media.Media
	if r.URL.Query().Get("force") == "true" {
		uploadedMedia, err = h.mediaService.ForceUpload(r.Context(), req)
	} else {
		uploadedMedia, err = h.mediaService.Upload(r.Context(), req)
	}
	if err != nil {
		h.logger.Error("Failed to upload media: %v", err)
		handleMediaError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusCreated, uploadedMedia)
}

// GetMedia handles GET /api/v1/media (browser admin realm). It returns ALL media (admins
// manage all site media) in newest-first (id DESC) order using opaque keyset (cursor)
// pagination — the SAME contract as the agent media list (bare data array +
// meta.pagination). It reuses the response package's cursor helpers and SuccessList/
// Pagination/ListMeta types so both auth realms share one envelope.
func (h *MediaHandler) GetMedia(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetUserID(r); !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	limit := response.ParseListLimit(r)
	beforeID, err := response.DecodeCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_cursor", "Invalid cursor", nil)
		return
	}

	search := r.URL.Query().Get(mediaSearchParam)
	dateFilter := r.URL.Query().Get(mediaDateFilterParam)

	mediaList, err := h.mediaService.SearchMediaByCursor(
		r.Context(),
		search,
		dateFilter,
		limit+1,
		beforeID,
	)
	if err != nil {
		h.logger.Error("Failed to get media: %v", err)
		handleMediaError(w, err)
		return
	}

	hasMore := len(mediaList) > limit
	if hasMore {
		mediaList = mediaList[:limit]
	}
	nextCursor := ""
	if hasMore && len(mediaList) > 0 {
		nextCursor = response.EncodeCursor(mediaList[len(mediaList)-1].ID)
	}

	total, err := h.mediaService.Count(r.Context(), search, dateFilter)
	if err != nil {
		h.logger.Error("Failed to count media: %v", err)
		handleMediaError(w, err)
		return
	}

	response.SuccessList(
		w,
		mediaList,
		response.ListMeta{
			Pagination: response.Pagination{
				NextCursor: nextCursor,
				HasMore:    hasMore,
				Total:      &total,
			},
		},
	)
}

func (h *MediaHandler) GetMediaByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetUserID(r); !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_media_id", "Invalid media ID", nil)
		return
	}

	mediaItem, err := h.mediaService.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get media: %v", err)
		handleMediaError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusOK, mediaItem)
}

func (h *MediaHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID", nil)
		return
	}

	role, _ := middleware.GetRole(r)

	if !h.mediaAllowed(r) {
		sendErrorResponse(w, http.StatusForbidden, "forbidden", "Your role cannot manage media", nil)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_media_id", "Invalid media ID", nil)
		return
	}

	if err := h.mediaService.Delete(r.Context(), id, userID, role); err != nil {
		h.logger.Error("Failed to delete media: %v", err)
		handleMediaError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *MediaHandler) GenerateImage(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID", nil)
		return
	}

	if !h.mediaAllowed(r) {
		sendErrorResponse(w, http.StatusForbidden, "forbidden", "Your role cannot manage media", nil)
		return
	}

	if h.imageGenService == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "not_configured", "AI image generation is not configured", nil)
		return
	}

	params, ok := h.parseGenerateImageRequest(w, r)
	if !ok {
		return
	}

	prompt := strings.TrimSpace(params.prompt)
	if prompt == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_prompt", "Prompt is required", nil)
		return
	}
	if utf8.RuneCountInString(prompt) > maxAIPromptLength {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_prompt", "Prompt must be less than 1000 characters", nil)
		return
	}

	if len(params.references) > 0 && !h.imageGenService.SupportsImageReferences() {
		sendErrorResponse(w, http.StatusBadRequest, "references_not_supported", "The configured AI model does not support reference images", nil)
		return
	}

	prompt = media.BuildGenerationPrompt(prompt, params.og)

	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Minute)
	defer cancel()
	imageBytes, err := h.imageGenService.GenerateImage(ctx, prompt, params.references)
	if err != nil {
		h.logger.Error("Failed to generate image: %v", err)
		if errors.Is(err, media.ErrImageReferencesNotSupported) {
			sendErrorResponse(w, http.StatusBadRequest, "references_not_supported", "The configured AI model does not support reference images", nil)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "generation_failed", "Failed to generate image", nil)
		return
	}

	if params.og {
		imageBytes, err = media.NewProcessor().CropToOpenGraph(imageBytes)
		if err != nil {
			h.logger.Error("Failed to crop generated image to Open Graph size: %v", err)
			sendErrorResponse(w, http.StatusInternalServerError, "generation_failed", "Failed to generate image", nil)
			return
		}
	}

	stamp := time.Now().Format("20060102-150405")
	originalFilename := fmt.Sprintf("ai-generated-%s.webp", stamp)
	if params.og {
		originalFilename = fmt.Sprintf("ai-generated-og-%s.webp", stamp)
	}

	generatedMedia, err := h.mediaService.GenerateFromBytes(ctx, imageBytes, userID, prompt, originalFilename)
	if err != nil {
		h.logger.Error("Failed to save generated image: %v", err)
		handleMediaError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusCreated, generatedMedia)
}

// parseGenerateImageRequest decodes the prompt, optional reference images, and
// the Open Graph flag from a GenerateImage request. Reference images and the
// flag require multipart/form-data with a "prompt" field, repeated "references"
// files, and an optional "og" field; the legacy JSON body carries a prompt and
// an optional og flag only. It reports whether parsing succeeded, writing the
// error response itself on failure.
func (h *MediaHandler) parseGenerateImageRequest(
	w http.ResponseWriter,
	r *http.Request,
) (generateImageParams, bool) {
	contentType, _, _ := strings.Cut(r.Header.Get("Content-Type"), ";")
	if strings.TrimSpace(contentType) == "multipart/form-data" {
		return h.parseGenerateMultipartRequest(w, r)
	}

	var req generateImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return generateImageParams{}, false
	}
	return generateImageParams{prompt: req.Prompt, og: req.OG}, true
}

// parseGenerateMultipartRequest decodes the prompt, reference images, and Open
// Graph flag from a multipart GenerateImage request.
func (h *MediaHandler) parseGenerateMultipartRequest(
	w http.ResponseWriter,
	r *http.Request,
) (generateImageParams, bool) {
	if err := r.ParseMultipartForm(maxGenerateMultipartBytes); err != nil {
		h.logger.Error("Failed to parse multipart form: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to parse form data", nil)
		return generateImageParams{}, false
	}

	var formFiles []*multipart.FileHeader
	if r.MultipartForm != nil {
		formFiles = r.MultipartForm.File["references"]
	}
	if len(formFiles) > media.MaxImageReferences {
		sendErrorResponse(
			w,
			http.StatusBadRequest,
			"invalid_references",
			fmt.Sprintf("Up to %d reference images are allowed", media.MaxImageReferences),
			nil,
		)
		return generateImageParams{}, false
	}

	references := make([]media.ImageReference, 0, len(formFiles))
	for _, header := range formFiles {
		reference, ok := h.readImageReference(w, header)
		if !ok {
			return generateImageParams{}, false
		}
		references = append(references, reference)
	}

	og := false
	if raw := r.FormValue("og"); raw != "" {
		var err error
		og, err = strconv.ParseBool(raw)
		if err != nil {
			sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid og value. Use true or false", nil)
			return generateImageParams{}, false
		}
	}

	return generateImageParams{prompt: r.FormValue("prompt"), references: references, og: og}, true
}

// readImageReference reads and validates a single reference image upload,
// reporting success and writing the error response itself on failure.
func (h *MediaHandler) readImageReference(
	w http.ResponseWriter,
	header *multipart.FileHeader,
) (media.ImageReference, bool) {
	file, err := header.Open()
	if err != nil {
		h.logger.Error("Failed to open reference image: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to read reference image", nil)
		return media.ImageReference{}, false
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, media.MaxFileSize+1))
	if err != nil {
		h.logger.Error("Failed to read reference image: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to read reference image", nil)
		return media.ImageReference{}, false
	}

	reference, err := media.NewImageReference(data)
	if err != nil {
		h.logger.Error("Invalid reference image: %v", err)
		if errors.Is(err, media.ErrFileTooLarge) {
			sendErrorResponse(w, http.StatusBadRequest, "file_too_large", "Reference image exceeds 10MB limit. Please use a smaller image", nil)
		} else {
			sendErrorResponse(w, http.StatusBadRequest, "invalid_file", "Invalid reference image. Please use an image file (JPG, PNG, WebP)", nil)
		}
		return media.ImageReference{}, false
	}

	return reference, true
}

func NewMediaHandler(
	mediaService MediaServiceInterface,
	imageGenService media.ImageGenerationService,
	logger *util.Logger,
	opts ...MediaHandlerOption,
) *MediaHandler {
	h := &MediaHandler{
		mediaService:    mediaService,
		imageGenService: imageGenService,
		logger:          logger,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}
