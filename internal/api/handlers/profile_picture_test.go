package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/domain/profilepicture"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockUserRepoForHandler implements repository.UserRepo partially for handler tests
type mockUserRepoForHandler struct {
	mock.Mock
}

func (m *mockUserRepoForHandler) GetUserByID(ctx context.Context, userID int) (*repository.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.User), args.Error(1)
}

func (m *mockUserRepoForHandler) UpdateAdminPasswordAndEmail(ctx context.Context, passwordHash, email, currentPasswordHash string) error {
	args := m.Called(ctx, passwordHash, email, currentPasswordHash)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) GetAdminUser(ctx context.Context) (*repository.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.User), args.Error(1)
}

func (m *mockUserRepoForHandler) CreateUser(ctx context.Context, user *repository.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) CheckUsernameExists(ctx context.Context, username string) (bool, error) {
	args := m.Called(ctx, username)
	return args.Bool(0), args.Error(1)
}

func (m *mockUserRepoForHandler) CheckEmailExists(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}

func (m *mockUserRepoForHandler) UpdateUserStatus(ctx context.Context, userID int, status string) error {
	args := m.Called(ctx, userID, status)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) UpdateUserStatusIfCurrentStatus(ctx context.Context, userID int, currentStatus string, newStatus string) error {
	args := m.Called(ctx, userID, currentStatus, newStatus)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) GetUserByEmail(ctx context.Context, email string) (*repository.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.User), args.Error(1)
}

func (m *mockUserRepoForHandler) GetUserByUsername(ctx context.Context, username string) (*repository.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.User), args.Error(1)
}

func (m *mockUserRepoForHandler) GetPendingUsers(ctx context.Context, limit int, offset int) ([]*repository.User, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.User), args.Error(1)
}

func (m *mockUserRepoForHandler) DeleteUser(ctx context.Context, userID int) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) SuspendUser(ctx context.Context, userID int) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) UnsuspendUser(ctx context.Context, userID int) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) SoftDeleteUser(ctx context.Context, userID int) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) GetAllUsers(ctx context.Context, status string, limit int, offset int) ([]*repository.User, error) {
	args := m.Called(ctx, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.User), args.Error(1)
}

func (m *mockUserRepoForHandler) GetUserStatus(ctx context.Context, userID int) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}

func (m *mockUserRepoForHandler) UpdateEmail(ctx context.Context, userID int, email string) error {
	args := m.Called(ctx, userID, email)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) UpdateName(ctx context.Context, userID int, name string) error {
	args := m.Called(ctx, userID, name)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) UpdatePassword(ctx context.Context, userID int, currentPasswordHash, newPasswordHash string) error {
	args := m.Called(ctx, userID, currentPasswordHash, newPasswordHash)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) UpdateLastLoginAt(ctx context.Context, userID int) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) UpdatePasswordByUserID(ctx context.Context, userID int, newPasswordHash string) error {
	args := m.Called(ctx, userID, newPasswordHash)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) UpdateProfile(ctx context.Context, userID int, name string, email string, role string, customFields map[string]any) error {
	args := m.Called(ctx, userID, name, email, role, customFields)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) UpdateCustomFields(ctx context.Context, userID int, customFields map[string]any) error {
	args := m.Called(ctx, userID, customFields)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) CheckEmailExistsForOtherUser(ctx context.Context, userID int, email string) (bool, error) {
	args := m.Called(ctx, userID, email)
	return args.Bool(0), args.Error(1)
}

func (m *mockUserRepoForHandler) UpdateProfilePicture(ctx context.Context, userID int, profilePicture string) error {
	args := m.Called(ctx, userID, profilePicture)
	return args.Error(0)
}

func (m *mockUserRepoForHandler) DeleteProfilePicture(ctx context.Context, userID int) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func createProfilePictureMultipartForm(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}

	var imgBuf bytes.Buffer
	err := png.Encode(&imgBuf, img)
	require.NoError(t, err)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="avatar.png"`)
	h.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(h)
	require.NoError(t, err)
	_, err = part.Write(imgBuf.Bytes())
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return &body, writer.FormDataContentType()
}

func setupProfilePictureTestRouter(
	t *testing.T,
	handler *handlers.ProfilePictureHandler,
) *chi.Mux {
	t.Helper()

	router := chi.NewRouter()
	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), middleware.UserIDKey, "1")
			ctx = context.WithValue(ctx, middleware.UsernameKey, "testuser")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	router.Route("/api/profile", func(r chi.Router) {
		r.With(authMiddleware).Put("/picture", handler.UploadProfilePicture)
		r.With(authMiddleware).Delete("/picture", handler.DeleteProfilePicture)
	})

	router.Route("/api/admin/users/{id}", func(r chi.Router) {
		r.With(authMiddleware).Put("/picture", handler.AdminUploadUserPicture)
		r.With(authMiddleware).Delete("/picture", handler.AdminDeleteUserPicture)
	})

	return router
}

func TestProfilePictureHandler_DeleteProfilePicture_Success(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)

	// Pre-save a file
	_, err := storage.Save("testuser_20260101000000.webp", bytes.NewReader([]byte("fake webp")))
	require.NoError(t, err)

	// Create adapter — use real repo adapter with mock
	mockRepo := new(mockUserRepoForHandler)
	mockRepo.On("GetUserByID", mock.Anything, 1).Return(&repository.User{
		ID:             1,
		Username:       "testuser",
		ProfilePicture: "testuser_20260101000000.webp",
	}, nil)
	mockRepo.On("DeleteProfilePicture", mock.Anything, 1).Return(nil)

	adapter := profilepicture.NewRepoAdapter(mockRepo)
	service := profilepicture.NewService(adapter, storage, profilepicture.NewProcessor())
	logger := util.NewLogger(os.Stdout)
	handler := handlers.NewProfilePictureHandler(service, mockRepo, logger)

	router := setupProfilePictureTestRouter(t, handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/profile/picture", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	require.Equal(t, "Profile picture deleted successfully", data["message"])
}

func TestProfilePictureHandler_DeleteProfilePicture_NoPicture(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)
	processor := profilepicture.NewProcessor()

	mockRepo := new(mockUserRepoForHandler)
	mockRepo.On("GetUserByID", mock.Anything, 1).Return(&repository.User{
		ID:             1,
		Username:       "testuser",
		ProfilePicture: "",
	}, nil)

	adapter := profilepicture.NewRepoAdapter(mockRepo)
	service := profilepicture.NewService(adapter, storage, processor)
	logger := util.NewLogger(os.Stdout)
	handler := handlers.NewProfilePictureHandler(service, mockRepo, logger)

	router := setupProfilePictureTestRouter(t, handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/profile/picture", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestProfilePictureHandler_UploadProfilePicture_Success(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)
	processor := profilepicture.NewProcessor()

	mockRepo := new(mockUserRepoForHandler)
	mockRepo.On("GetUserByID", mock.Anything, 1).Return(&repository.User{
		ID:             1,
		Username:       "testuser",
		ProfilePicture: "",
	}, nil)
	mockRepo.On("UpdateProfilePicture", mock.Anything, 1, mock.AnythingOfType("string")).Return(nil)

	adapter := profilepicture.NewRepoAdapter(mockRepo)
	service := profilepicture.NewService(adapter, storage, processor)
	logger := util.NewLogger(os.Stdout)
	handler := handlers.NewProfilePictureHandler(service, mockRepo, logger)

	router := setupProfilePictureTestRouter(t, handler)

	body, contentType := createProfilePictureMultipartForm(t)
	req := httptest.NewRequest(http.MethodPut, "/api/profile/picture", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	require.Contains(t, data["profilePicture"], "testuser_")
}

func TestProfilePictureHandler_AdminUploadUserPicture_Success(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)
	processor := profilepicture.NewProcessor()

	mockRepo := new(mockUserRepoForHandler)
	mockRepo.On("GetUserByID", mock.Anything, 5).Return(&repository.User{
		ID:             5,
		Username:       "targetuser",
		ProfilePicture: "",
	}, nil)
	mockRepo.On("UpdateProfilePicture", mock.Anything, 5, mock.AnythingOfType("string")).Return(nil)

	adapter := profilepicture.NewRepoAdapter(mockRepo)
	service := profilepicture.NewService(adapter, storage, processor)
	logger := util.NewLogger(os.Stdout)
	handler := handlers.NewProfilePictureHandler(service, mockRepo, logger)

	router := setupProfilePictureTestRouter(t, handler)

	body, contentType := createProfilePictureMultipartForm(t)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/5/picture", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	require.Contains(t, data["profilePicture"], "targetuser_")
}

func TestProfilePictureHandler_AdminDeleteUserPicture_Success(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)

	// Pre-save
	_, err := storage.Save("targetuser_20260101000000.webp", bytes.NewReader([]byte("fake webp")))
	require.NoError(t, err)

	mockRepo := new(mockUserRepoForHandler)
	mockRepo.On("GetUserByID", mock.Anything, 5).Return(&repository.User{
		ID:             5,
		Username:       "targetuser",
		ProfilePicture: "targetuser_20260101000000.webp",
	}, nil)
	mockRepo.On("DeleteProfilePicture", mock.Anything, 5).Return(nil)

	adapter := profilepicture.NewRepoAdapter(mockRepo)
	service := profilepicture.NewService(adapter, storage, profilepicture.NewProcessor())
	logger := util.NewLogger(os.Stdout)
	handler := handlers.NewProfilePictureHandler(service, mockRepo, logger)

	router := setupProfilePictureTestRouter(t, handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/5/picture", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}
