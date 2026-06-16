package profilepicture_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/profilepicture"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockUserRepo implements profilepicture.UserRepo for testing
type mockUserRepo struct {
	mock.Mock
}

func (m *mockUserRepo) GetUserByID(ctx context.Context, userID int) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}

func (m *mockUserRepo) UpdateProfilePicture(ctx context.Context, userID int, profilePicture string) error {
	args := m.Called(ctx, userID, profilePicture)
	return args.Error(0)
}

func (m *mockUserRepo) DeleteProfilePicture(ctx context.Context, userID int) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// createTestImage creates a PNG image of the given dimensions
func createTestImage(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	require.NoError(t, err)
	return buf.Bytes()
}

// createMultipartFile creates a multipart.File and FileHeader for testing
func createMultipartFile(t *testing.T, imageData []byte) (multipart.File, *multipart.FileHeader) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="test.png"`)
	h.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(h)
	require.NoError(t, err)
	_, err = part.Write(imageData)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	err = req.ParseMultipartForm(profilepicture.ProfilePictureMaxFileSize)
	require.NoError(t, err)

	file, header, err := req.FormFile("image")
	require.NoError(t, err)
	return file, header
}

func TestService_Upload_Success(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)
	processor := profilepicture.NewProcessor()

	mockRepo := new(mockUserRepo)
	mockRepo.On("GetUserByID", mock.Anything, 1).Return("", nil)
	mockRepo.On("UpdateProfilePicture", mock.Anything, 1, mock.MatchedBy(func(s string) bool {
		return strings.HasSuffix(s, ".webp")
	})).Return(nil)

	imageData := createTestImage(t, 200, 200)
	file, header := createMultipartFile(t, imageData)
	defer func() { _ = file.Close() }()

	service := profilepicture.NewService(mockRepo, storage, processor)
	url, err := service.Upload(context.Background(), 1, "testuser", file, header)

	require.NoError(t, err)
	assert.NotEmpty(t, url)
	assert.Contains(t, url, "testuser_")
	assert.Contains(t, url, ".webp")

	// Verify file exists on disk
	entries, err := os.ReadDir(baseDir)
	require.NoError(t, err)
	assert.Equal(t, 1, len(entries))
	mockRepo.AssertExpectations(t)
}

func TestService_Upload_ReUploadDeletesOldFile(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)
	processor := profilepicture.NewProcessor()

	imageData := createTestImage(t, 200, 200)

	// First upload — no old picture. Use a fresh mock.
	mockRepo1 := new(mockUserRepo)
	mockRepo1.On("GetUserByID", mock.Anything, 1).Return("", nil).Once()
	mockRepo1.On("UpdateProfilePicture", mock.Anything, 1, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "testuser_") && strings.HasSuffix(s, ".webp")
	})).Return(nil).Once()

	service1 := profilepicture.NewService(mockRepo1, storage, processor)
	file1, header1 := createMultipartFile(t, imageData)
	url1, err := service1.Upload(context.Background(), 1, "testuser", file1, header1)
	require.NoError(t, err)
	require.NotEmpty(t, url1)
	_ = file1.Close()
	mockRepo1.AssertExpectations(t)

	// Small sleep to ensure different timestamp
	time.Sleep(1100 * time.Millisecond)

	// Get the first filename from URL
	parts1 := strings.Split(url1, "/")
	firstFilename := parts1[len(parts1)-1]

	// Verify one file exists on disk after first upload
	entries, err := os.ReadDir(baseDir)
	require.NoError(t, err)
	assert.Equal(t, 1, len(entries))

	// Second upload — has old picture. Use a new mock.
	mockRepo2 := new(mockUserRepo)
	mockRepo2.On("GetUserByID", mock.Anything, 1).Return(firstFilename, nil).Once()
	mockRepo2.On("UpdateProfilePicture", mock.Anything, 1, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "testuser_") && strings.HasSuffix(s, ".webp")
	})).Return(nil).Once()

	service2 := profilepicture.NewService(mockRepo2, storage, processor)
	file2, header2 := createMultipartFile(t, imageData)
	url2, err := service2.Upload(context.Background(), 1, "testuser", file2, header2)
	require.NoError(t, err)
	require.NotEmpty(t, url2)
	_ = file2.Close()

	assert.NotEqual(t, url1, url2)

	// Old file should be deleted, only one file remains
	entries, err = os.ReadDir(baseDir)
	require.NoError(t, err)
	assert.Equal(t, 1, len(entries))
	mockRepo2.AssertExpectations(t)
}

func TestService_Upload_FileTooLarge(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)
	processor := profilepicture.NewProcessor()

	mockRepo := new(mockUserRepo)

	service := profilepicture.NewService(mockRepo, storage, processor)

	// Create a multipart file and override its Size field
	imageData := createTestImage(t, 200, 200)
	file, header := createMultipartFile(t, imageData)
	defer func() { _ = file.Close() }()
	header.Size = profilepicture.ProfilePictureMaxFileSize + 1

	_, err := service.Upload(context.Background(), 1, "testuser", file, header)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file size exceeds limit")
}

func TestService_Delete_Success(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)
	processor := profilepicture.NewProcessor()

	mockRepo := new(mockUserRepo)
	mockRepo.On("GetUserByID", mock.Anything, 1).Return("test_user_20260101120000.webp", nil)
	mockRepo.On("DeleteProfilePicture", mock.Anything, 1).Return(nil)

	// Pre-create a file so the delete doesn't error
	_, err := storage.Save("test_user_20260101120000.webp", bytes.NewReader([]byte("fake data")))
	require.NoError(t, err)

	service := profilepicture.NewService(mockRepo, storage, processor)
	err = service.Delete(context.Background(), 1)

	require.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestService_Delete_NoExistingPicture(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)
	processor := profilepicture.NewProcessor()

	mockRepo := new(mockUserRepo)
	mockRepo.On("GetUserByID", mock.Anything, 1).Return("", nil)

	service := profilepicture.NewService(mockRepo, storage, processor)
	err := service.Delete(context.Background(), 1)

	require.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestService_Delete_UserRepoError(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)
	processor := profilepicture.NewProcessor()

	mockRepo := new(mockUserRepo)
	mockRepo.On("GetUserByID", mock.Anything, 1).Return("", assert.AnError)

	service := profilepicture.NewService(mockRepo, storage, processor)
	err := service.Delete(context.Background(), 1)

	require.Error(t, err)
	mockRepo.AssertExpectations(t)
}

func TestService_Delete_ClearDBError(t *testing.T) {
	baseDir := t.TempDir()
	storage := profilepicture.NewLocalStorage(baseDir, "localhost", 8080)
	processor := profilepicture.NewProcessor()

	mockRepo := new(mockUserRepo)
	mockRepo.On("GetUserByID", mock.Anything, 1).Return("test_user_20260101120000.webp", nil)
	mockRepo.On("DeleteProfilePicture", mock.Anything, 1).Return(assert.AnError)

	// Pre-create a file so delete works
	_, err := storage.Save("test_user_20260101120000.webp", bytes.NewReader([]byte("fake data")))
	require.NoError(t, err)

	service := profilepicture.NewService(mockRepo, storage, processor)
	err = service.Delete(context.Background(), 1)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to clear profile picture")
	mockRepo.AssertExpectations(t)
}

func TestSanitizeUsernameForFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal username",
			input:    "testuser",
			expected: "testuser",
		},
		{
			name:     "username with dashes and underscores",
			input:    "test-user_123",
			expected: "test-user_123",
		},
		{
			name:     "path traversal attempt",
			input:    "../etc/passwd",
			expected: "etcpasswd",
		},
		{
			name:     "empty username",
			input:    "",
			expected: "user",
		},
		{
			name:     "special characters only",
			input:    "!@#$%",
			expected: "user",
		},
		{
			name:     "spaces removed",
			input:    "hello world",
			expected: "helloworld",
		},
		{
			name:     "null bytes removed",
			input:    "user\x00name",
			expected: "username",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := profilepicture.SanitizeUsernameForFilename(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
