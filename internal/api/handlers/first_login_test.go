package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	appresponse "github.com/aristorinjuang/lesstruct/internal/api/response"
	authdomain "github.com/aristorinjuang/lesstruct/internal/domain/auth"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	repomocks "github.com/aristorinjuang/lesstruct/internal/repository/mocks"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
)

func TestGetStatus(t *testing.T) {
	defaultHash := "$2a$12$testdefaultpasswordhash"
	service := authdomain.NewFirstLoginService(defaultHash)

	mockRepo := repomocks.NewMockUserRepo(t)
	mockRepo.EXPECT().
		GetAdminUser(mock.Anything).
		Return(&repository.User{
			ID:           1,
			Username:     "admin",
			PasswordHash: defaultHash,
			Role:         "Admin",
		}, nil)

	logger := util.NewLogger(os.Stdout)
	handler := handlers.NewFirstLoginHandler(service, mockRepo, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/first-login", nil)
	w := httptest.NewRecorder()

	handler.GetStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Status")

	var resp appresponse.Response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Failed to decode response")

	assert.NotNil(t, resp.Data, "Response data should not be nil")

	// Parse the data field
	dataBytes, _ := json.Marshal(resp.Data)
	var status handlers.FirstLoginStatus
	err = json.Unmarshal(dataBytes, &status)
	require.NoError(t, err, "Failed to parse status response")

	assert.False(t, status.FirstLoginComplete, "FirstLoginComplete should be false initially")
}

func TestGetStatus_AfterComplete(t *testing.T) {
	defaultHash := "$2a$12$testdefaultpasswordhash"
	service := authdomain.NewFirstLoginService(defaultHash)

	// Admin password has been changed from default -> setup is complete
	changedHash := "$2a$12$changedpasswordhash"
	mockRepo := repomocks.NewMockUserRepo(t)
	mockRepo.EXPECT().
		GetAdminUser(mock.Anything).
		Return(&repository.User{
			ID:           1,
			Username:     "admin",
			PasswordHash: changedHash,
			Role:         "Admin",
		}, nil)

	logger := util.NewLogger(os.Stdout)
	handler := handlers.NewFirstLoginHandler(service, mockRepo, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/first-login", nil)
	w := httptest.NewRecorder()

	handler.GetStatus(w, req)

	var resp appresponse.Response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Failed to decode response")

	dataBytes, _ := json.Marshal(resp.Data)
	var status handlers.FirstLoginStatus
	err = json.Unmarshal(dataBytes, &status)
	require.NoError(t, err, "Failed to parse status response")

	assert.True(t, status.FirstLoginComplete, "FirstLoginComplete should be true when admin password has been changed")
}
