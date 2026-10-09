package profilepicture_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/users/domain/profilepicture"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubUserRepo implements profilepicture.UserRepo with programmable results.
type stubUserRepo struct {
	picture   string
	getErr    error
	updated   string
	updateErr error
	cleared   bool
	clearErr  error
}

func (s *stubUserRepo) GetUserByID(_ context.Context, _ int) (string, error) {
	return s.picture, s.getErr
}

func (s *stubUserRepo) UpdateProfilePicture(_ context.Context, _ int, profilePicture string) error {
	s.updated = profilePicture
	return s.updateErr
}

func (s *stubUserRepo) DeleteProfilePicture(_ context.Context, _ int) error {
	s.cleared = true
	return s.clearErr
}

// stubStorage is an in-memory kernel storage for testing.
type stubStorage struct {
	files     map[string][]byte
	saveErr   error
	deleteErr error
	deleted   []string
}

func (s *stubStorage) Save(filename string, reader io.Reader) (string, error) {
	if s.saveErr != nil {
		return "", s.saveErr
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	if s.files == nil {
		s.files = map[string][]byte{}
	}
	s.files[filename] = data
	return filename, nil
}

func (s *stubStorage) Delete(filePath string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = append(s.deleted, filePath)
	delete(s.files, filePath)
	return nil
}

func (s *stubStorage) GetURL(filePath string) string {
	return "/uploads/profile_pictures/" + filePath
}

// createTestImage creates a PNG image of the given dimensions
func createTestImage(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
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
	return createMultipartFileWithType(t, imageData, "image/png")
}

// createMultipartFileWithType creates a multipart.File and FileHeader with an
// explicit part content type for testing.
func createMultipartFileWithType(t *testing.T, imageData []byte, contentType string) (multipart.File, *multipart.FileHeader) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="test.png"`)
	h.Set("Content-Type", contentType)
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

func testPrincipal() kernel.Principal {
	return kernel.Principal{ID: "1", Username: "testuser"}
}

// errorFile fails every read with a fixed error.
type errorFile struct{}

func (errorFile) Read(_ []byte) (int, error) {
	return 0, errors.New("forced read error")
}

func (errorFile) ReadAt(_ []byte, _ int64) (int, error) {
	return 0, errors.New("forced read error")
}

func (errorFile) Seek(_ int64, _ int) (int64, error) {
	return 0, nil
}

func (errorFile) Close() error {
	return nil
}

// failSeekFile reads normally but fails seeks.
type failSeekFile struct {
	*bytes.Reader
}

func (failSeekFile) Close() error {
	return nil
}

func (f failSeekFile) Seek(_ int64, _ int) (int64, error) {
	return 0, errors.New("forced seek error")
}

// testFileHeader builds a FileHeader for direct service calls.
func testFileHeader(size int64, contentType string) *multipart.FileHeader {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="test.png"`)
	h.Set("Content-Type", contentType)
	return &multipart.FileHeader{
		Filename: "test.png",
		Header:   h,
		Size:     size,
	}
}

func TestService_Upload_InvalidMimeType(t *testing.T) {
	storage := &stubStorage{}
	processor := profilepicture.NewProcessor()
	repo := &stubUserRepo{}

	imageData := createTestImage(t, 200, 200)
	file, _ := createMultipartFile(t, imageData)
	defer func() { _ = file.Close() }()
	header := testFileHeader(int64(len(imageData)), "text/plain")

	service := profilepicture.NewService(repo, storage, processor)
	_, err := service.Upload(context.Background(), testPrincipal(), file, header)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid MIME type")
}

func TestService_Upload_ReadError(t *testing.T) {
	storage := &stubStorage{}
	processor := profilepicture.NewProcessor()
	repo := &stubUserRepo{}

	header := testFileHeader(100, "image/png")

	service := profilepicture.NewService(repo, storage, processor)
	_, err := service.Upload(context.Background(), testPrincipal(), errorFile{}, header)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestService_Upload_InvalidContent(t *testing.T) {
	storage := &stubStorage{}
	processor := profilepicture.NewProcessor()
	repo := &stubUserRepo{}

	body := []byte("not an image")
	file, _ := createMultipartFileWithType(t, body, "image/png")
	defer func() { _ = file.Close() }()
	header := testFileHeader(int64(len(body)), "image/png")

	service := profilepicture.NewService(repo, storage, processor)
	_, err := service.Upload(context.Background(), testPrincipal(), file, header)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid file content")
}

func TestService_Upload_SeekError(t *testing.T) {
	storage := &stubStorage{}
	processor := profilepicture.NewProcessor()
	repo := &stubUserRepo{}

	imageData := createTestImage(t, 200, 200)
	file := failSeekFile{Reader: bytes.NewReader(imageData)}
	header := testFileHeader(int64(len(imageData)), "image/png")

	service := profilepicture.NewService(repo, storage, processor)
	_, err := service.Upload(context.Background(), testPrincipal(), file, header)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to reset file reader")
}

func TestService_Upload_ProcessError(t *testing.T) {
	storage := &stubStorage{}
	processor := profilepicture.NewProcessor()
	repo := &stubUserRepo{}

	// Truncated PNG: magic intact so content validation passes, decode fails.
	imageData := createTestImage(t, 200, 200)
	truncated := imageData[:200]
	file, _ := createMultipartFile(t, truncated)
	defer func() { _ = file.Close() }()
	header := testFileHeader(int64(len(truncated)), "image/png")

	service := profilepicture.NewService(repo, storage, processor)
	_, err := service.Upload(context.Background(), testPrincipal(), file, header)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to process image")
}

func TestService_Upload_SaveError(t *testing.T) {
	storage := &stubStorage{saveErr: assert.AnError}
	processor := profilepicture.NewProcessor()
	repo := &stubUserRepo{}

	imageData := createTestImage(t, 200, 200)
	file, header := createMultipartFile(t, imageData)
	defer func() { _ = file.Close() }()

	service := profilepicture.NewService(repo, storage, processor)
	_, err := service.Upload(context.Background(), testPrincipal(), file, header)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save file")
	assert.Empty(t, repo.updated, "DB must not be touched when save fails")
}

func TestService_Upload_UpdateErrorCleansUp(t *testing.T) {
	storage := &stubStorage{}
	processor := profilepicture.NewProcessor()
	repo := &stubUserRepo{updateErr: assert.AnError}

	imageData := createTestImage(t, 200, 200)
	file, header := createMultipartFile(t, imageData)
	defer func() { _ = file.Close() }()

	service := profilepicture.NewService(repo, storage, processor)
	_, err := service.Upload(context.Background(), testPrincipal(), file, header)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update profile picture in database")
	assert.Empty(t, storage.files, "new file must be cleaned up on DB failure")
}

func TestService_Upload_OldFileLookupErrorContinues(t *testing.T) {
	storage := &stubStorage{}
	processor := profilepicture.NewProcessor()
	repo := &stubUserRepo{getErr: assert.AnError}

	imageData := createTestImage(t, 200, 200)
	file, header := createMultipartFile(t, imageData)
	defer func() { _ = file.Close() }()

	service := profilepicture.NewService(repo, storage, processor)
	url, err := service.Upload(context.Background(), testPrincipal(), file, header)

	require.NoError(t, err, "old-file lookup failure must not block the upload")
	assert.NotEmpty(t, url)
}

func TestService_Upload_OldFileDeleteWarning(t *testing.T) {
	storage := &stubStorage{
		files:     map[string][]byte{"old_pic.webp": []byte("old")},
		deleteErr: assert.AnError,
	}
	processor := profilepicture.NewProcessor()
	repo := &stubUserRepo{picture: "old_pic.webp"}

	imageData := createTestImage(t, 200, 200)
	file, header := createMultipartFile(t, imageData)
	defer func() { _ = file.Close() }()

	service := profilepicture.NewService(repo, storage, processor)
	url, err := service.Upload(context.Background(), testPrincipal(), file, header)

	require.NoError(t, err, "old-file delete failure must not block the upload")
	assert.NotEmpty(t, url)
}

func TestService_Delete_StorageDeleteWarning(t *testing.T) {
	storage := &stubStorage{
		files:     map[string][]byte{"pic.webp": []byte("data")},
		deleteErr: assert.AnError,
	}
	processor := profilepicture.NewProcessor()
	repo := &stubUserRepo{picture: "pic.webp"}

	service := profilepicture.NewService(repo, storage, processor)
	err := service.Delete(context.Background(), testPrincipal())

	require.NoError(t, err, "file delete failure must not block the DB clear")
	assert.True(t, repo.cleared)
}

func TestService_Upload_Success(t *testing.T) {
	storage := &stubStorage{}
	processor := profilepicture.NewProcessor()

	repo := &stubUserRepo{}

	imageData := createTestImage(t, 200, 200)
	file, header := createMultipartFile(t, imageData)
	defer func() { _ = file.Close() }()

	service := profilepicture.NewService(repo, storage, processor)
	url, err := service.Upload(context.Background(), testPrincipal(), file, header)

	require.NoError(t, err)
	assert.NotEmpty(t, url)
	assert.Contains(t, url, "testuser_")
	assert.Contains(t, url, ".webp")

	// Verify file exists in storage
	assert.Equal(t, 1, len(storage.files))
	assert.True(t, strings.HasSuffix(repo.updated, ".webp"))
}

func TestService_Upload_ReUploadDeletesOldFile(t *testing.T) {
	storage := &stubStorage{}
	processor := profilepicture.NewProcessor()

	imageData := createTestImage(t, 200, 200)

	// First upload — no old picture.
	repo1 := &stubUserRepo{}

	service1 := profilepicture.NewService(repo1, storage, processor)
	file1, header1 := createMultipartFile(t, imageData)
	url1, err := service1.Upload(context.Background(), testPrincipal(), file1, header1)
	require.NoError(t, err)
	require.NotEmpty(t, url1)
	_ = file1.Close()

	// Small sleep to ensure different timestamp
	time.Sleep(1100 * time.Millisecond)

	// Get the first filename from URL
	parts1 := strings.Split(url1, "/")
	firstFilename := parts1[len(parts1)-1]

	// Verify one file exists in storage after first upload
	assert.Equal(t, 1, len(storage.files))

	// Second upload — has old picture.
	repo2 := &stubUserRepo{picture: firstFilename}

	service2 := profilepicture.NewService(repo2, storage, processor)
	file2, header2 := createMultipartFile(t, imageData)
	url2, err := service2.Upload(context.Background(), testPrincipal(), file2, header2)
	require.NoError(t, err)
	require.NotEmpty(t, url2)
	_ = file2.Close()

	assert.NotEqual(t, url1, url2)

	// Old file should be deleted, only one file remains
	assert.Equal(t, 1, len(storage.files))
	assert.Contains(t, storage.deleted, firstFilename)
}

func TestService_Upload_FileTooLarge(t *testing.T) {
	storage := &stubStorage{}
	processor := profilepicture.NewProcessor()

	repo := &stubUserRepo{}

	service := profilepicture.NewService(repo, storage, processor)

	// Create a multipart file and override its Size field
	imageData := createTestImage(t, 200, 200)
	file, header := createMultipartFile(t, imageData)
	defer func() { _ = file.Close() }()
	header.Size = profilepicture.ProfilePictureMaxFileSize + 1

	_, err := service.Upload(context.Background(), testPrincipal(), file, header)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file size exceeds limit")
}

func TestService_Delete_Success(t *testing.T) {
	storage := &stubStorage{
		files: map[string][]byte{"test_user_20260101120000.webp": []byte("fake data")},
	}
	processor := profilepicture.NewProcessor()

	repo := &stubUserRepo{picture: "test_user_20260101120000.webp"}

	service := profilepicture.NewService(repo, storage, processor)
	err := service.Delete(context.Background(), testPrincipal())

	require.NoError(t, err)
	assert.True(t, repo.cleared)
	assert.Empty(t, storage.files)
}

func TestService_Delete_NoExistingPicture(t *testing.T) {
	storage := &stubStorage{}
	processor := profilepicture.NewProcessor()

	repo := &stubUserRepo{}

	service := profilepicture.NewService(repo, storage, processor)
	err := service.Delete(context.Background(), testPrincipal())

	require.NoError(t, err)
	assert.False(t, repo.cleared)
}

func TestService_Delete_UserRepoError(t *testing.T) {
	storage := &stubStorage{}
	processor := profilepicture.NewProcessor()

	repo := &stubUserRepo{getErr: assert.AnError}

	service := profilepicture.NewService(repo, storage, processor)
	err := service.Delete(context.Background(), testPrincipal())

	require.Error(t, err)
}

func TestService_Delete_ClearDBError(t *testing.T) {
	storage := &stubStorage{
		files: map[string][]byte{"test_user_20260101120000.webp": []byte("fake data")},
	}
	processor := profilepicture.NewProcessor()

	repo := &stubUserRepo{
		picture:  "test_user_20260101120000.webp",
		clearErr: assert.AnError,
	}

	service := profilepicture.NewService(repo, storage, processor)
	err := service.Delete(context.Background(), testPrincipal())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to clear profile picture")
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

func TestService_InvalidPrincipal(t *testing.T) {
	svc := profilepicture.NewService(nil, nil, nil)
	bad := kernel.Principal{ID: "not-a-number", Username: "ghost"}

	tests := []struct {
		name    string
		call    func() error
		wantErr bool
	}{
		{
			name: "upload rejects non-decimal id",
			call: func() error {
				_, err := svc.Upload(context.Background(), bad, nil, nil)
				return err
			},
			wantErr: true,
		},
		{
			name: "delete rejects non-decimal id",
			call: func() error {
				return svc.Delete(context.Background(), bad)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, kernel.ErrInvalidPrincipalID)
				return
			}
			require.NoError(t, err)
		})
	}
}
