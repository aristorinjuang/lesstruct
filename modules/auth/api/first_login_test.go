package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/auth/api"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStatus(t *testing.T) {
	service := authdomain.NewFirstLoginService(stubPolicy{})
	store := &stubUserStore{
		admin: &userdomain.User{
			ID:           1,
			Username:     "admin",
			PasswordHash: "hashed:admin",
			Role:         "Admin",
		},
	}
	handler := api.NewFirstLoginHandler(service, store, stubPolicy{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/first-login", nil)
	w := httptest.NewRecorder()

	handler.GetStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Status")

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "Failed to decode response")
	require.NotNil(t, resp["data"], "Response data should not be nil")

	// Parse the data field
	dataBytes, _ := json.Marshal(resp["data"])
	var status api.FirstLoginStatus
	require.NoError(t, json.Unmarshal(dataBytes, &status), "Failed to parse status response")

	assert.False(t, status.FirstLoginComplete, "FirstLoginComplete should be false initially")
}

func TestGetStatus_AfterComplete(t *testing.T) {
	service := authdomain.NewFirstLoginService(stubPolicy{})

	// Admin password has been changed from default -> setup is complete
	store := &stubUserStore{
		admin: &userdomain.User{
			ID:           1,
			Username:     "admin",
			PasswordHash: "hashed:ChangedPassword456!",
			Role:         "Admin",
		},
	}
	handler := api.NewFirstLoginHandler(service, store, stubPolicy{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/first-login", nil)
	w := httptest.NewRecorder()

	handler.GetStatus(w, req)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "Failed to decode response")

	dataBytes, _ := json.Marshal(resp["data"])
	var status api.FirstLoginStatus
	require.NoError(t, json.Unmarshal(dataBytes, &status), "Failed to parse status response")

	assert.True(t, status.FirstLoginComplete, "FirstLoginComplete should be true when admin password has been changed")
}
