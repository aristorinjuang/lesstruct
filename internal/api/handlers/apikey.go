package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

// CreateAPIKeyRequest is the payload for POST /api/admin/api-keys.
type CreateAPIKeyRequest struct {
	Name  string `json:"name"`
	Scope string `json:"scope,omitempty"` // TODO(story-future): per-key scopes deferred — accepted but not persisted
}

// CreateAPIKeyResponse is returned on successful key creation. The full key
// string is included exactly once; the client must store it.
type CreateAPIKeyResponse struct {
	Key        string    `json:"key"`
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	KeyPrefix  string    `json:"keyPrefix"`
	CreatedAt  time.Time `json:"createdAt"`
}

// APIKeyListItem is a safe (masked) list entry. It MUST NOT contain keyHash,
// the secret, or lastUsedIp (privacy-sensitive). Prefix is the masked display
// form "lesstruct_<keyID>••••" produced via apikey.DisplayPrefix.
type APIKeyListItem struct {
	ID         int        `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	RevokedAt  *time.Time `json:"revokedAt"`
}

// RevokeAPIKeyResponse is returned on successful (or idempotent) revoke.
type RevokeAPIKeyResponse struct {
	ID        int        `json:"id"`
	RevokedAt *time.Time `json:"revokedAt"`
}

// APIKeyService is the narrow interface the handler depends on (avoids
// hard-coupling to *apikey.Service; mirrors the dashboard ServiceInterface pattern).
type APIKeyService interface {
	// Existing (Story 1.1):
	Create(ctx context.Context, userID int, name string) (string, *apikey.APIKey, error)
	// New (Story 1.2):
	List(ctx context.Context, userID int) ([]*apikey.APIKey, error)
	Revoke(ctx context.Context, id, userID int) (*apikey.APIKey, error)
}

// APIKeyHandler handles browser-realm API key lifecycle endpoints.
type APIKeyHandler struct {
	apiKeyService APIKeyService
	logger        *util.Logger
}

// CreateAPIKey handles POST /api/admin/api-keys.
func (h *APIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		// A non-numeric userID in the auth context is a server-side invariant
		// violation (the auth middleware only stores numeric IDs).
		h.logger.Error("Non-numeric userID %q in auth context: %v", userIDStr, err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Invalid authenticated session", nil)
		return
	}

	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body", nil)
		return
	}

	keyString, key, err := h.apiKeyService.Create(r.Context(), userID, req.Name)
	if err != nil {
		if errors.Is(err, apikey.ErrInvalidKeyName) {
			response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		if errors.Is(err, apikey.ErrDuplicateKeyName) {
			response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
			return
		}

		h.logger.Error("Failed to create API key for user %d: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create API key", nil)
		return
	}

	// Defensive guard: the APIKeyService interface contract returns a non-nil key
	// on success, but a faulty/mock implementation could return (keyString, nil, nil).
	if key == nil {
		h.logger.Error("API key service returned nil entity for user %d", userID)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create API key", nil)
		return
	}

	// NEVER log the full key string — log only the public keyID and userID.
	h.logger.Info("API key created: user=%d keyID=%s", userID, key.KeyID)

	resp := CreateAPIKeyResponse{
		Key:       keyString,
		ID:        key.ID,
		Name:      key.Name,
		KeyPrefix: apikey.KeyPrefix + key.KeyID,
		CreatedAt: key.CreatedAt,
	}
	response.Success(w, resp)
}

// ListAPIKeys handles GET /api/admin/api-keys. Returns the caller's keys as
// masked list entries (prefix + metadata only). The secret, key_hash, and
// last_used_ip are NEVER included in the response.
func (h *APIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		// A non-numeric userID in the auth context is a server-side invariant
		// violation (the auth middleware only stores numeric IDs).
		h.logger.Error("Non-numeric userID %q in auth context: %v", userIDStr, err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Invalid authenticated session", nil)
		return
	}

	keys, err := h.apiKeyService.List(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to list API keys for user %d: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list API keys", nil)
		return
	}

	items := make([]APIKeyListItem, 0, len(keys))
	for _, k := range keys {
		items = append(items, APIKeyListItem{
			ID:         k.ID,
			Name:       k.Name,
			Prefix:     apikey.DisplayPrefix(k.KeyID),
			CreatedAt:  k.CreatedAt,
			LastUsedAt: k.LastUsedAt,
			ExpiresAt:  k.ExpiresAt,
			RevokedAt:  k.RevokedAt,
		})
	}
	response.Success(w, items)
}

// RevokeAPIKey handles DELETE /api/admin/api-keys/:id. Soft-deletes the key
// (sets revoked_at). Idempotent: revoking an already-revoked key returns 200.
// Cross-user or nonexistent ids return 404 NOT_FOUND (identical response — no
// cross-user disclosure).
func (h *APIKeyHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		h.logger.Error("Non-numeric userID %q in auth context: %v", userIDStr, err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Invalid authenticated session", nil)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid API key ID", nil)
		return
	}

	key, err := h.apiKeyService.Revoke(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, apikey.ErrKeyNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "API key not found", nil)
			return
		}
		h.logger.Error("Failed to revoke API key %d for user %d: %v", id, userID, err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to revoke API key", nil)
		return
	}

	// NEVER log the secret or key_hash — log only the public keyID and userID.
	h.logger.Info("API key revoked: user=%d keyID=%s", userID, key.KeyID)

	response.Success(w, RevokeAPIKeyResponse{
		ID:        key.ID,
		RevokedAt: key.RevokedAt,
	})
}

// NewAPIKeyHandler constructs a new APIKeyHandler.
func NewAPIKeyHandler(
	apiKeyService APIKeyService,
	logger *util.Logger,
) *APIKeyHandler {
	return &APIKeyHandler{
		apiKeyService: apiKeyService,
		logger:        logger,
	}
}
