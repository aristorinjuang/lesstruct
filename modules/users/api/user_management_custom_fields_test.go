package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aristorinjuang/lesstruct/modules/users/api"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

func setupCustomFieldsTestRouter(
	t *testing.T,
	userRepo *stubUserRepo,
	blockedEmailRepo *stubBlockedRepo,
	emailService *stubMailer,
	softDeleteRepo *stubSoftDelete,
) *chi.Mux {
	t.Helper()

	service := userdomain.NewUserManagementService(userRepo, blockedEmailRepo, stubPolicy{}, true)
	handler := api.NewUserManagementHandler(
		service,
		nil,
		userRepo,
		softDeleteRepo,
		emailService,
		nil,
		nil,
	)

	r := chi.NewRouter()
	r.Put("/api/admin/users/{id}", handler.UpdateUser)
	return r
}

func TestUpdateUser_WithCustomFields(t *testing.T) {
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			1: {
				ID: 1, Username: "testuser", Email: "test@example.com",
				Name: "Test User", Role: "Contributor",
				Status: "verified", CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	router := setupCustomFieldsTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

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
		Data api.UpdateUserProfileResponse `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &wrapper)
	require.NoError(t, err)
	require.NotNil(t, wrapper.Data.User)
	assert.Equal(t, "Engineer", wrapper.Data.User.CustomFields["job_title"])
	assert.Equal(t, "Acme", wrapper.Data.User.CustomFields["company"])
}

func TestUpdateUser_WithoutCustomFields(t *testing.T) {
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			1: {
				ID: 1, Username: "testuser", Email: "test@example.com",
				Name: "Test User", Role: "Contributor",
				Status: "verified", CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	router := setupCustomFieldsTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var wrapper struct {
		Data api.UpdateUserProfileResponse `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &wrapper)
	require.NoError(t, err)
	assert.Nil(t, wrapper.Data.User.CustomFields)
}

func TestUpdateUserProfileService_WithCustomFields(t *testing.T) {
	repo := &stubUserRepo{
		users: map[int]*userdomain.User{
			1: {
				ID: 1, Username: "testuser", Email: "test@example.com",
				Name: "Test", Role: "Contributor",
				Status: "verified", CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	service := userdomain.NewUserManagementService(repo, nil, stubPolicy{}, true)

	customFields := map[string]any{"key": "value"}
	updated, err := service.UpdateUserProfile(context.Background(), 1, "Test", "test@example.com", "Contributor", customFields)
	require.NoError(t, err)
	require.NotNil(t, updated.CustomFields)
	assert.Equal(t, "value", updated.CustomFields["key"])
}

func TestUpdateUserProfileService_NilCustomFields(t *testing.T) {
	repo := &stubUserRepo{
		users: map[int]*userdomain.User{
			1: {
				ID: 1, Username: "testuser", Email: "test@example.com",
				Name: "Test", Role: "Contributor",
				Status: "verified", CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	service := userdomain.NewUserManagementService(repo, nil, stubPolicy{}, true)

	updated, err := service.UpdateUserProfile(context.Background(), 1, "", "", "", nil)
	require.NoError(t, err)
	assert.Nil(t, updated.CustomFields)
}
