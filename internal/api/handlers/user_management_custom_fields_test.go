package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	repomocks "github.com/aristorinjuang/lesstruct/internal/repository/mocks"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/domain/user"
	usermocks "github.com/aristorinjuang/lesstruct/internal/domain/user/mocks"
	"github.com/aristorinjuang/lesstruct/internal/auth"
	emailmocks "github.com/aristorinjuang/lesstruct/internal/email/mocks"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupCustomFieldsTestRouter(
	t *testing.T,
	userRepo *repomocks.MockUserRepo,
	blockedEmailRepo *usermocks.MockBlockedEmailRepo,
	emailService *emailmocks.MockEmailService,
	softDeleteRepo *repomocks.MockSoftDeleteRepo,
) *chi.Mux {
	t.Helper()
	jwtManager := auth.NewJWTManager("test-secret-key-for-testing")
	logger := util.NewLogger(os.Stdout)

	service := user.NewUserManagementService(userRepo, blockedEmailRepo)
	handler := handlers.NewUserManagementHandler(
		service,
		nil,
		userRepo,
		softDeleteRepo,
		jwtManager,
		emailService,
		logger,
		nil,
	)

	r := chi.NewRouter()
	r.Put("/api/admin/users/{id}", handler.UpdateUser)
	return r
}

func TestUpdateUser_WithCustomFields(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := usermocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	router := setupCustomFieldsTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Test User", Role: "Contributor",
		}, nil).Once()

	userRepo.EXPECT().
		CheckEmailExistsForOtherUser(mock.Anything, 1, "new@example.com").
		Return(false, nil)

	// Name falls back to username since no name in request; email and role passed through
	userRepo.EXPECT().
		UpdateProfile(mock.Anything, 1, "testuser", "new@example.com", "Contributor", mock.Anything).
		Return(nil)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "new@example.com",
			Name: "Test User", Role: "Contributor", Status: "verified", CreatedAt: "2025-01-01",
			CustomFields: map[string]any{"job_title": "Engineer", "company": "Acme"},
		}, nil).Once()

	body, _ := json.Marshal(map[string]any{
		"email":        "new@example.com",
		"customFields": map[string]any{"job_title": "Engineer", "company": "Acme"},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var wrapper struct {
		Data handlers.UpdateUserProfileResponse `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &wrapper)
	require.NoError(t, err)
	require.NotNil(t, wrapper.Data.User)
	assert.Equal(t, "Engineer", wrapper.Data.User.CustomFields["job_title"])
	assert.Equal(t, "Acme", wrapper.Data.User.CustomFields["company"])
}

func TestUpdateUser_WithoutCustomFields(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	blockedEmailRepo := usermocks.NewMockBlockedEmailRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)

	router := setupCustomFieldsTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Test User", Role: "Contributor",
		}, nil).Once()

	// Name falls back to username since no name in request; no email check needed (empty email = use existing)
	userRepo.EXPECT().
		UpdateProfile(mock.Anything, 1, "testuser", "test@example.com", "Contributor", mock.Anything).
		Return(nil)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Test User", Role: "Contributor", Status: "verified", CreatedAt: "2025-01-01",
		}, nil).Once()

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var wrapper struct {
		Data handlers.UpdateUserProfileResponse `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &wrapper)
	require.NoError(t, err)
	assert.Nil(t, wrapper.Data.User.CustomFields)
}

func TestUpdateUserProfileService_WithCustomFields(t *testing.T) {
	mockUserRepo := usermocks.NewMockUserRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Test", Role: "Contributor",
		}, nil).Once()

	mockUserRepo.EXPECT().
		UpdateProfile(mock.Anything, 1, "Test", "test@example.com", "Contributor", mock.Anything).
		Return(nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Test", Role: "Contributor", Status: "verified", CreatedAt: "2025-01-01",
			CustomFields: map[string]any{"key": "value"},
		}, nil).Once()

	customFields := map[string]any{"key": "value"}
	updated, err := service.UpdateUserProfile(context.Background(), 1, "Test", "test@example.com", "Contributor", customFields)
	require.NoError(t, err)
	require.NotNil(t, updated.CustomFields)
	assert.Equal(t, "value", updated.CustomFields["key"])
}

func TestUpdateUserProfileService_NilCustomFields(t *testing.T) {
	mockUserRepo := usermocks.NewMockUserRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Test", Role: "Contributor",
		}, nil).Once()

	mockUserRepo.EXPECT().
		UpdateProfile(mock.Anything, 1, "testuser", "test@example.com", "Contributor", mock.Anything).
		Return(nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Test", Role: "Contributor", Status: "verified", CreatedAt: "2025-01-01",
		}, nil).Once()

	updated, err := service.UpdateUserProfile(context.Background(), 1, "", "", "", nil)
	require.NoError(t, err)
	assert.Nil(t, updated.CustomFields)
}
