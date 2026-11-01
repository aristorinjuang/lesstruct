package media_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	media "github.com/aristorinjuang/lesstruct/modules/media/domain"
	mediamocks "github.com/aristorinjuang/lesstruct/modules/media/domain/mocks"
	thumbnail "github.com/aristorinjuang/lesstruct/modules/media/domain/thumbnail"
	"github.com/deepteams/webp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func createTestImage(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	c := color.RGBA{255, 0, 0, 255}
	for y := range 100 {
		for x := range 100 {
			img.Set(x, y, c)
		}
	}

	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	require.NoError(t, err)
	return buf.Bytes()
}

func createTestFile(t *testing.T, data []byte) (*os.File, *multipart.FileHeader) {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "test-*.png")
	require.NoError(t, err)

	_, err = tmpFile.Write(data)
	require.NoError(t, err)

	_, err = tmpFile.Seek(0, 0)
	require.NoError(t, err)

	header := &multipart.FileHeader{
		Filename: filepath.Base(tmpFile.Name()),
		Size:     int64(len(data)),
	}

	return tmpFile, header
}

// stubStorage is an in-memory kernelstorage.Storage double. Save stores
// bytes under the filename and returns a URL-like key; errors are injected
// per test via saveErr/deleteErr (saveSucceed caps how many initial saves
// succeed before saveErr applies); Delete records removed paths.
type stubStorage struct {
	saved       map[string][]byte
	saveErr     error
	saveSucceed int
	deleteErr   error
	deleted     []string
	openErr     error
}

func newStubStorage() *stubStorage {
	return &stubStorage{
		saved: make(map[string][]byte),
	}
}

func (s *stubStorage) Save(filename string, reader io.Reader) (string, error) {
	if s.saveErr != nil && len(s.saved) >= s.saveSucceed {
		return "", s.saveErr
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	s.saved[filename] = data
	return "/uploads/media/" + filename, nil
}

func (s *stubStorage) Delete(filePath string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = append(s.deleted, filePath)
	return nil
}

func (s *stubStorage) GetURL(filePath string) string {
	return filePath
}

func (s *stubStorage) Open(filePath string) (io.ReadCloser, error) {
	if s.openErr != nil {
		return nil, s.openErr
	}
	if data, ok := s.saved[filePath]; ok {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	base := filepath.Base(filePath)
	if data, ok := s.saved[base]; ok {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	trimmed := strings.TrimPrefix(filePath, "/uploads/media/")
	if data, ok := s.saved[trimmed]; ok {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return nil, io.ErrUnexpectedEOF
}

func testPrincipal(userID int, role string) kernel.Principal {
	roles := make([]string, 0, 1)
	if role != "" {
		roles = append(roles, role)
	}
	return kernel.Principal{
		ID:       strconv.Itoa(userID),
		Username: "testuser",
		Roles:    roles,
	}
}

func TestService_Upload(t *testing.T) {
	tests := []struct {
		name           string
		altText        string
		wantErr        bool
		expectErr      error
		expectDupMedia *media.Media
		setupMock      func(*mediamocks.MockRepository, *stubStorage)
	}{
		{
			name:    "successful upload",
			altText: "A test image",
			wantErr: false,
			setupMock: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
				repo.On("Create", mock.Anything, mock.Anything).Return(nil)
			},
		},
		{
			name:      "missing alt text",
			altText:   "",
			wantErr:   true,
			expectErr: media.ErrInvalidAltText,
			setupMock: func(repo *mediamocks.MockRepository, storage *stubStorage) {},
		},
		{
			name:      "whitespace only alt text",
			altText:   "   ",
			wantErr:   true,
			expectErr: media.ErrInvalidAltText,
			setupMock: func(repo *mediamocks.MockRepository, storage *stubStorage) {},
		},
		{
			name:           "duplicate upload",
			altText:        "Duplicate image",
			wantErr:        true,
			expectDupMedia: &media.Media{ID: 99, UserID: 1, OriginalFilename: "existing.png", Hash: "duplicate-hash"},
			setupMock: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				existing := &media.Media{ID: 99, UserID: 1, OriginalFilename: "existing.png", Hash: "duplicate-hash"}
				repo.On("FindByHash", mock.Anything, mock.Anything).Return(existing, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mediamocks.NewMockRepository(t)
			mockStorage := newStubStorage()
			tt.setupMock(mockRepo, mockStorage)

			svc := media.NewService(mockRepo, mockStorage, nil)

			imgData := createTestImage(t)
			file, header := createTestFile(t, imgData)
			defer func() { _ = file.Close() }()

			req := media.UploadRequest{
				File:       file,
				FileHeader: header,
				UserID:     1,
				AltText:    tt.altText,
			}

			got, err := svc.Upload(context.Background(), req)

			if (err != nil) != tt.wantErr {
				t.Errorf("Service.Upload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.expectDupMedia != nil {
				var dupErr *media.DuplicateMediaError
				require.True(t, errors.As(err, &dupErr), "expected DuplicateMediaError")
				assert.Equal(t, tt.expectDupMedia.OriginalFilename, dupErr.Existing.OriginalFilename)
			} else if tt.expectErr != nil && !errors.Is(err, tt.expectErr) {
				t.Errorf("Service.Upload() error = %v, expectErr %v", err, tt.expectErr)
			}

			if !tt.wantErr && got != nil {
				if got.AltText != strings.TrimSpace(tt.altText) {
					t.Errorf("Service.Upload() AltText = %v, want %v", got.AltText, strings.TrimSpace(tt.altText))
				}
				if !got.IsWebP {
					t.Errorf("Service.Upload() IsWebP = %v, want true", got.IsWebP)
				}
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetByID(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mediamocks.MockRepository)
		wantErr   bool
		expectErr error
	}{
		{
			name: "found",
			setupMock: func(repo *mediamocks.MockRepository) {
				mockMedia := &media.Media{
					ID:     1,
					UserID: 1,
					URL:    "/uploads/media/test.webp",
				}
				repo.On("FindByID", mock.Anything, 1).Return(mockMedia, nil)
			},
			wantErr: false,
		},
		{
			name: "not found",
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("FindByID", mock.Anything, 1).Return((*media.Media)(nil), media.ErrMediaNotFound)
			},
			wantErr:   true,
			expectErr: media.ErrMediaNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mediamocks.NewMockRepository(t)
			mockStorage := newStubStorage()
			tt.setupMock(mockRepo)

			svc := media.NewService(mockRepo, mockStorage, nil)

			got, err := svc.GetByID(context.Background(), 1)

			if (err != nil) != tt.wantErr {
				t.Errorf("Service.GetByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.expectErr != nil && !errors.Is(err, tt.expectErr) {
				t.Errorf("Service.GetByID() error = %v, expectErr %v", err, tt.expectErr)
			}

			if !tt.wantErr && got == nil {
				t.Errorf("Service.GetByID() returned nil media")
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_ListByCursor(t *testing.T) {
	tests := []struct {
		name        string
		userID      int
		limit       int
		beforeID    int
		setupMock   func(*mediamocks.MockRepository)
		expectedErr error
		expectedLen int
	}{
		{
			name:     "first page - beforeID 0 returns newest media",
			userID:   1,
			limit:    50,
			beforeID: 0,
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("ListByCursor", mock.Anything, 1, 50, 0).Return([]*media.Media{
					{ID: 3, UserID: 1},
					{ID: 2, UserID: 1},
					{ID: 1, UserID: 1},
				}, nil)
			},
			expectedErr: nil,
			expectedLen: 3,
		},
		{
			name:     "next page - beforeID filters to older media",
			userID:   1,
			limit:    50,
			beforeID: 2,
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("ListByCursor", mock.Anything, 1, 50, 2).Return([]*media.Media{
					{ID: 1, UserID: 1},
				}, nil)
			},
			expectedErr: nil,
			expectedLen: 1,
		},
		{
			name:     "empty result - no media for the caller",
			userID:   1,
			limit:    50,
			beforeID: 0,
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("ListByCursor", mock.Anything, 1, 50, 0).Return([]*media.Media{}, nil)
			},
			expectedErr: nil,
			expectedLen: 0,
		},
		{
			name:     "repository error - wrapped as failed to list media",
			userID:   1,
			limit:    50,
			beforeID: 0,
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("ListByCursor", mock.Anything, 1, 50, 0).Return([]*media.Media(nil), errors.New("database error"))
			},
			expectedErr: errors.New("failed to list media"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mediamocks.NewMockRepository(t)
			mockStorage := newStubStorage()
			tt.setupMock(mockRepo)

			svc := media.NewService(mockRepo, mockStorage, nil)
			result, err := svc.ListByCursor(context.Background(), tt.userID, tt.limit, tt.beforeID)

			if tt.expectedErr != nil {
				require.Error(t, err, "Service.ListByCursor() expected error, got nil")
				assert.Contains(t, err.Error(), tt.expectedErr.Error(), "Service.ListByCursor() error message")
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err, "Service.ListByCursor() unexpected error")
			require.Len(t, result, tt.expectedLen, "Service.ListByCursor() result length")
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_Delete(t *testing.T) {
	tests := []struct {
		name      string
		userID    int
		userRole  string
		setupMock func(*mediamocks.MockRepository, *stubStorage)
		wantErr   bool
		expectErr error
	}{
		{
			name:     "successful delete by owner",
			userID:   1,
			userRole: "Contributor",
			setupMock: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				mockMedia := &media.Media{
					ID:       1,
					UserID:   1,
					FilePath: "/uploads/media/test.webp",
				}
				repo.On("FindByID", mock.Anything, 1).Return(mockMedia, nil)
				repo.On("DeleteByOwner", mock.Anything, 1, 1).Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "successful delete by admin",
			userID:   2,
			userRole: "Admin",
			setupMock: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				mockMedia := &media.Media{
					ID:       1,
					UserID:   1,
					FilePath: "/uploads/media/test.webp",
				}
				repo.On("FindByID", mock.Anything, 1).Return(mockMedia, nil)
				repo.On("DeleteByID", mock.Anything, 1).Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "media not found",
			userID:   1,
			userRole: "Contributor",
			setupMock: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				repo.On("FindByID", mock.Anything, 1).Return((*media.Media)(nil), media.ErrMediaNotFound)
			},
			wantErr:   true,
			expectErr: media.ErrMediaNotFound,
		},
		{
			name:     "unauthorized delete - different user non-admin",
			userID:   2,
			userRole: "Contributor",
			setupMock: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				mockMedia := &media.Media{ID: 1, UserID: 1, FilePath: "/uploads/media/test.webp"}
				repo.On("FindByID", mock.Anything, 1).Return(mockMedia, nil)
			},
			wantErr:   true,
			expectErr: media.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mediamocks.NewMockRepository(t)
			mockStorage := newStubStorage()
			tt.setupMock(mockRepo, mockStorage)

			svc := media.NewService(mockRepo, mockStorage, nil)

			err := svc.Delete(context.Background(), 1, testPrincipal(tt.userID, tt.userRole))

			if (err != nil) != tt.wantErr {
				t.Errorf("Service.Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.expectErr != nil && !errors.Is(err, tt.expectErr) {
				t.Errorf("Service.Delete() error = %v, expectErr %v", err, tt.expectErr)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_Upload_StorageFailure(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()
	mockStorage.saveErr = errors.New("storage error")

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)

	svc := media.NewService(mockRepo, mockStorage, nil)

	imgData := createTestImage(t)
	file, header := createTestFile(t, imgData)
	defer func() { _ = file.Close() }()

	req := media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "Test image",
	}

	_, err := svc.Upload(context.Background(), req)

	require.Error(t, err)

	mockRepo.AssertExpectations(t)
}

func TestService_Upload_InvalidFileType(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	svc := media.NewService(mockRepo, mockStorage, nil)

	invalidData := []byte("not an image")
	tmpDir := t.TempDir()
	tmpFile, _ := os.CreateTemp(tmpDir, "test-*.txt")
	_, _ = tmpFile.Write(invalidData)
	_, _ = tmpFile.Seek(0, 0)

	header := &multipart.FileHeader{
		Filename: filepath.Base(tmpFile.Name()),
		Size:     int64(len(invalidData)),
	}

	req := media.UploadRequest{
		File:       tmpFile,
		FileHeader: header,
		UserID:     1,
		AltText:    "Test file",
	}

	_, err := svc.Upload(context.Background(), req)

	require.Error(t, err)
}

func TestService_Upload_FileTooLarge(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	svc := media.NewService(mockRepo, mockStorage, nil)

	largeData := make([]byte, 11*1024*1024)
	tmpDir := t.TempDir()
	tmpFile, _ := os.CreateTemp(tmpDir, "test-*.jpg")
	_, _ = tmpFile.Write(largeData)
	_, _ = tmpFile.Seek(0, 0)

	header := &multipart.FileHeader{
		Filename: "large.jpg",
		Size:     int64(len(largeData)),
	}

	req := media.UploadRequest{
		File:       tmpFile,
		FileHeader: header,
		UserID:     1,
		AltText:    "Large image",
	}

	_, err := svc.Upload(context.Background(), req)

	require.Error(t, err)
}

func TestService_Delete_StorageFailure(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()
	mockStorage.deleteErr = errors.New("storage error")

	mockMedia := &media.Media{
		ID:       1,
		UserID:   1,
		FilePath: "/uploads/media/test.webp",
	}

	mockRepo.On("FindByID", mock.Anything, 1).Return(mockMedia, nil)

	svc := media.NewService(mockRepo, mockStorage, nil)

	err := svc.Delete(context.Background(), 1, testPrincipal(1, "Contributor"))

	require.Error(t, err)

	mockRepo.AssertExpectations(t)
}

func TestService_Upload_InvalidExtension(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	svc := media.NewService(mockRepo, mockStorage, nil)

	imgData := createTestImage(t)
	tmpDir := t.TempDir()
	tmpFile, _ := os.CreateTemp(tmpDir, "test-*.pdf")
	_, _ = tmpFile.Write(imgData)
	_, _ = tmpFile.Seek(0, 0)

	header := &multipart.FileHeader{
		Filename: "test.pdf",
		Size:     int64(len(imgData)),
	}

	req := media.UploadRequest{
		File:       tmpFile,
		FileHeader: header,
		UserID:     1,
		AltText:    "Test PDF",
	}

	_, err := svc.Upload(context.Background(), req)

	require.Error(t, err)
}

func TestService_Upload_ZeroSizeFile(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	svc := media.NewService(mockRepo, mockStorage, nil)

	emptyData := []byte{}
	tmpDir := t.TempDir()
	tmpFile, _ := os.CreateTemp(tmpDir, "test-*.jpg")
	_, _ = tmpFile.Write(emptyData)
	_, _ = tmpFile.Seek(0, 0)

	header := &multipart.FileHeader{
		Filename: "empty.jpg",
		Size:     0,
	}

	req := media.UploadRequest{
		File:       tmpFile,
		FileHeader: header,
		UserID:     1,
		AltText:    "Empty file",
	}

	_, err := svc.Upload(context.Background(), req)

	require.Error(t, err)
}

func TestService_Upload_RepositoryCreateError(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("database error"))

	svc := media.NewService(mockRepo, mockStorage, nil)

	imgData := createTestImage(t)
	file, header := createTestFile(t, imgData)
	defer func() { _ = file.Close() }()

	req := media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "A test image",
	}

	_, err := svc.Upload(context.Background(), req)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create media record")
	require.Len(t, mockStorage.deleted, 1, "failed upload must clean up the saved file")

	mockRepo.AssertExpectations(t)
}

func TestService_Delete_RepositoryDeleteError(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	mockMedia := &media.Media{
		ID:       1,
		UserID:   1,
		FilePath: "/uploads/media/test.webp",
	}

	mockRepo.On("FindByID", mock.Anything, 1).Return(mockMedia, nil)
	mockRepo.On("DeleteByOwner", mock.Anything, 1, 1).Return(errors.New("database error"))

	svc := media.NewService(mockRepo, mockStorage, nil)

	err := svc.Delete(context.Background(), 1, testPrincipal(1, "Contributor"))

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to delete media")

	mockRepo.AssertExpectations(t)
}

func TestService_Upload_ConvertToWebPError(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)

	svc := media.NewService(mockRepo, mockStorage, nil)

	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00,
	}
	tmpDir := t.TempDir()
	tmpFile, _ := os.CreateTemp(tmpDir, "test-*.png")
	_, _ = tmpFile.Write(pngData)
	_, _ = tmpFile.Seek(0, 0)

	header := &multipart.FileHeader{
		Filename: "test.png",
		Size:     int64(len(pngData)),
	}

	req := media.UploadRequest{
		File:       tmpFile,
		FileHeader: header,
		UserID:     1,
		AltText:    "Test image",
	}

	_, err := svc.Upload(context.Background(), req)

	require.Error(t, err)

	mockRepo.AssertExpectations(t)
}

type mockFileThatFailsAfterValidation struct {
	*os.File
	readsBeforeFailure int
	readsCompleted     int
}

func (m *mockFileThatFailsAfterValidation) Read(p []byte) (n int, err error) {
	m.readsCompleted++
	if m.readsCompleted > m.readsBeforeFailure {
		return 0, errors.New("read error during hash")
	}
	n, err = m.File.Read(p)
	return n, err
}

func TestService_Upload_GenerateHashError(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	svc := media.NewService(mockRepo, mockStorage, nil)

	imgData := createTestImage(t)
	tmpDir := t.TempDir()
	tmpFile, _ := os.CreateTemp(tmpDir, "test-*.png")
	_, _ = tmpFile.Write(imgData)
	_, _ = tmpFile.Seek(0, 0)

	hashFile := &mockFileThatFailsAfterValidation{
		File:               tmpFile,
		readsBeforeFailure: 1,
	}

	header := &multipart.FileHeader{
		Filename: "test.png",
		Size:     int64(len(imgData)),
	}

	req := media.UploadRequest{
		File:       hashFile,
		FileHeader: header,
		UserID:     1,
		AltText:    "Test image",
	}

	_, err := svc.Upload(context.Background(), req)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to generate hash")
}

type mockFileWithSeekError struct {
	*os.File
}

func (m *mockFileWithSeekError) Seek(offset int64, whence int) (int64, error) {
	return 0, errors.New("seek error")
}

func TestService_Upload_SeekErrorAfterValidation(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	svc := media.NewService(mockRepo, mockStorage, nil)

	imgData := createTestImage(t)
	tmpDir := t.TempDir()
	tmpFile, _ := os.CreateTemp(tmpDir, "test-*.png")
	_, _ = tmpFile.Write(imgData)
	_, _ = tmpFile.Seek(0, 0)

	seekFile := &mockFileWithSeekError{File: tmpFile}

	header := &multipart.FileHeader{
		Filename: "test.png",
		Size:     int64(len(imgData)),
	}

	req := media.UploadRequest{
		File:       seekFile,
		FileHeader: header,
		UserID:     1,
		AltText:    "Test image",
	}

	_, err := svc.Upload(context.Background(), req)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to seek file")
}

type mockFileWithReadError struct {
	*os.File
	readShouldFail bool
}

func (m *mockFileWithReadError) Read(p []byte) (n int, err error) {
	if m.readShouldFail {
		return 0, errors.New("read error")
	}
	return m.File.Read(p)
}

func TestService_Upload_FileReadError(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	svc := media.NewService(mockRepo, mockStorage, nil)

	imgData := createTestImage(t)
	tmpDir := t.TempDir()
	tmpFile, _ := os.CreateTemp(tmpDir, "test-*.png")
	_, _ = tmpFile.Write(imgData)

	readFile := &mockFileWithReadError{File: tmpFile, readShouldFail: true}

	header := &multipart.FileHeader{
		Filename: "test.png",
		Size:     int64(len(imgData)),
	}

	req := media.UploadRequest{
		File:       readFile,
		FileHeader: header,
		UserID:     1,
		AltText:    "Test image",
	}

	_, err := svc.Upload(context.Background(), req)

	require.Error(t, err)
	require.Contains(t, err.Error(), "file validation failed")
}

func TestService_validateMagicNumbers_ShortData(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	svc := media.NewService(mockRepo, mockStorage, nil)

	shortData := []byte{0xFF, 0xD8}
	tmpDir := t.TempDir()
	tmpFile, _ := os.CreateTemp(tmpDir, "test-*.dat")
	_, _ = tmpFile.Write(shortData)
	_, _ = tmpFile.Seek(0, 0)

	header := &multipart.FileHeader{
		Filename: "test.dat",
		Size:     int64(len(shortData)),
	}

	req := media.UploadRequest{
		File:       tmpFile,
		FileHeader: header,
		UserID:     1,
		AltText:    "Test file",
	}

	_, err := svc.Upload(context.Background(), req)

	require.Error(t, err)
}

func TestService_Upload_AllSignatureTypes(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		ext  string
	}{
		{
			name: "png",
			data: createTestImage(t),
			ext:  ".png",
		},
		{
			name: "jpeg",
			data: createTestJPEGImage(t),
			ext:  ".jpg",
		},
		{
			name: "gif",
			data: createTestGIFImage(t),
			ext:  ".gif",
		},
		{
			name: "webp",
			data: createTestWebPImage(t),
			ext:  ".webp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mediamocks.NewMockRepository(t)
			mockStorage := newStubStorage()

			mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
			mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

			svc := media.NewService(mockRepo, mockStorage, nil)

			tmpDir := t.TempDir()
			tmpFile, _ := os.CreateTemp(tmpDir, "test-*"+tt.ext)
			_, _ = tmpFile.Write(tt.data)
			_, _ = tmpFile.Seek(0, 0)

			header := &multipart.FileHeader{
				Filename: "test" + tt.ext,
				Size:     int64(len(tt.data)),
			}

			req := media.UploadRequest{
				File:       tmpFile,
				FileHeader: header,
				UserID:     1,
				AltText:    "Test image",
			}

			got, err := svc.Upload(context.Background(), req)

			require.NoError(t, err)
			require.NotNil(t, got)

			mockRepo.AssertExpectations(t)
		})
	}
}

func createTestJPEGImage(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	c := color.RGBA{255, 0, 0, 255}
	for y := range 100 {
		for x := range 100 {
			img.Set(x, y, c)
		}
	}

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	require.NoError(t, err)
	return buf.Bytes()
}

func createTestGIFImage(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	c := color.RGBA{255, 0, 0, 255}
	for y := range 100 {
		for x := range 100 {
			img.Set(x, y, c)
		}
	}

	var buf bytes.Buffer
	err := gif.Encode(&buf, img, &gif.Options{})
	require.NoError(t, err)
	return buf.Bytes()
}

func createTestWebPImage(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	c := color.RGBA{255, 0, 0, 255}
	for y := range 100 {
		for x := range 100 {
			img.Set(x, y, c)
		}
	}

	var buf bytes.Buffer
	err := webp.Encode(&buf, img, &webp.Options{Quality: 80})
	require.NoError(t, err)
	return buf.Bytes()
}

func TestParseDateFilter(t *testing.T) {
	wednesday := time.Date(2026, time.October, 21, 15, 30, 0, 0, time.UTC)
	sunday := time.Date(2026, time.October, 25, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		filter    string
		now       time.Time
		wantValid bool
		expected  time.Time
	}{
		{
			name:      "today returns start of day",
			filter:    "today",
			now:       wednesday,
			wantValid: true,
			expected:  time.Date(2026, time.October, 21, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "this_week returns monday of the current week",
			filter:    "this_week",
			now:       wednesday,
			wantValid: true,
			expected:  time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "this_week on a sunday returns the preceding monday",
			filter:    "this_week",
			now:       sunday,
			wantValid: true,
			expected:  time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "this_month returns start of month",
			filter:    "this_month",
			now:       wednesday,
			wantValid: true,
			expected:  time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "invalid filter returns false",
			filter:    "invalid",
			now:       wednesday,
			wantValid: false,
			expected:  time.Time{},
		},
		{
			name:      "empty filter returns false",
			filter:    "",
			now:       wednesday,
			wantValid: false,
			expected:  time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := media.SetNowForTest(func() time.Time { return tt.now })
			defer restore()

			result, valid := media.ParseDateFilter(tt.filter)
			assert.Equal(t, tt.wantValid, valid)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDuplicateMediaError(t *testing.T) {
	tests := []struct {
		name     string
		existing *media.Media
		expected string
	}{
		{
			name:     "names the existing file",
			existing: &media.Media{ID: 42, UserID: 1, OriginalFilename: "sunset.jpg", Hash: "abc123"},
			expected: "media already exists: sunset.jpg",
		},
		{
			name:     "falls back when the existing file is unknown",
			existing: nil,
			expected: "media already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dupErr := &media.DuplicateMediaError{Existing: tt.existing}

			assert.Equal(t, tt.expected, dupErr.Error())
			assert.Equal(t, tt.existing, dupErr.Existing)

			var target *media.DuplicateMediaError
			require.True(t, errors.As(dupErr, &target))
			assert.Equal(t, tt.existing, target.Existing)
		})
	}
}

func TestService_ForceUpload(t *testing.T) {
	t.Run("successful force upload with no existing hash", func(t *testing.T) {
		mockRepo := mediamocks.NewMockRepository(t)
		mockStorage := newStubStorage()

		mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
		mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

		svc := media.NewService(mockRepo, mockStorage, nil)

		imgData := createTestImage(t)
		file, header := createTestFile(t, imgData)
		defer func() { _ = file.Close() }()

		req := media.UploadRequest{
			File:       file,
			FileHeader: header,
			UserID:     1,
			AltText:    "Force upload",
		}

		got, err := svc.ForceUpload(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Contains(t, got.OriginalFilename, "-2.png")
		assert.True(t, got.IsWebP)

		mockRepo.AssertExpectations(t)
	})

	t.Run("force upload with existing hash generates unique variant", func(t *testing.T) {
		mockRepo := mediamocks.NewMockRepository(t)
		mockStorage := newStubStorage()

		existing := &media.Media{ID: 1, Hash: "existing-hash", OriginalFilename: "existing.png"}

		mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return(existing, nil).Once()
		mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound).Once()
		mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

		svc := media.NewService(mockRepo, mockStorage, nil)

		imgData := createTestImage(t)
		file, header := createTestFile(t, imgData)
		defer func() { _ = file.Close() }()

		req := media.UploadRequest{
			File:       file,
			FileHeader: header,
			UserID:     1,
			AltText:    "Force upload duplicate",
		}

		got, err := svc.ForceUpload(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Contains(t, got.Hash, "_1")
		assert.Contains(t, got.OriginalFilename, "-2")

		mockRepo.AssertExpectations(t)
	})

	t.Run("missing alt text returns error", func(t *testing.T) {
		mockRepo := mediamocks.NewMockRepository(t)
		mockStorage := newStubStorage()

		svc := media.NewService(mockRepo, mockStorage, nil)

		imgData := createTestImage(t)
		file, header := createTestFile(t, imgData)
		defer func() { _ = file.Close() }()

		req := media.UploadRequest{
			File:       file,
			FileHeader: header,
			UserID:     1,
			AltText:    "",
		}

		_, err := svc.ForceUpload(context.Background(), req)

		require.Error(t, err)
		assert.True(t, errors.Is(err, media.ErrInvalidAltText))
	})

	t.Run("invalid file extension returns error", func(t *testing.T) {
		mockRepo := mediamocks.NewMockRepository(t)
		mockStorage := newStubStorage()

		svc := media.NewService(mockRepo, mockStorage, nil)

		invalidData := []byte("not an image")
		tmpDir := t.TempDir()
		tmpFile, _ := os.CreateTemp(tmpDir, "test-*.txt")
		_, _ = tmpFile.Write(invalidData)
		_, _ = tmpFile.Seek(0, 0)

		header := &multipart.FileHeader{
			Filename: filepath.Base(tmpFile.Name()),
			Size:     int64(len(invalidData)),
		}

		req := media.UploadRequest{
			File:       tmpFile,
			FileHeader: header,
			UserID:     1,
			AltText:    "Test file",
		}

		_, err := svc.ForceUpload(context.Background(), req)

		require.Error(t, err)
	})
}

func TestGenerateDuplicateFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sunset.jpg", "sunset-2.jpg"},
		{"photo.png", "photo-2.png"},
		{"image.gif", "image-2.gif"},
		{"noextension", "noextension-2"},
		{"multi.part.name.jpg", "multi.part.name-2.jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := media.GenerateDuplicateFilename(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestService_SearchMediaByCursor(t *testing.T) {
	admin := kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}}
	buyer := kernel.Principal{ID: "7", Username: "buyer", Roles: []string{"Commentator"}}

	tests := []struct {
		name        string
		principal   kernel.Principal
		search      string
		dateFilter  string
		limit       int
		beforeID    int
		setupMock   func(*mediamocks.MockRepository)
		expectedLen int
		expectedErr error
	}{
		{
			name:       "success - admin scope searches every user's media",
			principal:  admin,
			search:     "",
			dateFilter: "",
			limit:      50,
			beforeID:   0,
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("FindAllByCursor", mock.Anything, 0, 50, 0).Return([]*media.Media{}, nil)
			},
		},
		{
			name:       "success - non-admin scope is pinned to the caller's library",
			principal:  buyer,
			search:     "",
			dateFilter: "",
			limit:      50,
			beforeID:   0,
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("FindAllByCursor", mock.Anything, 7, 50, 0).Return([]*media.Media{}, nil)
			},
		},
		{
			name:       "search only calls FindAllByFilenameByCursor",
			principal:  admin,
			search:     "sunset",
			dateFilter: "",
			limit:      50,
			beforeID:   0,
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("FindAllByFilenameByCursor", mock.Anything, 0, "sunset", 50, 0).Return([]*media.Media{}, nil)
			},
		},
		{
			name:       "date filter only calls FindAllByDateRangeByCursor",
			principal:  buyer,
			search:     "",
			dateFilter: "today",
			limit:      50,
			beforeID:   0,
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("FindAllByDateRangeByCursor", mock.Anything, 7, mock.AnythingOfType("time.Time"), 50, 0).Return([]*media.Media{}, nil)
			},
		},
		{
			name:       "both filters calls FindAllByFilenameAndDateRangeByCursor",
			principal:  admin,
			search:     "sunset",
			dateFilter: "this_week",
			limit:      50,
			beforeID:   0,
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("FindAllByFilenameAndDateRangeByCursor", mock.Anything, 0, "sunset", mock.AnythingOfType("time.Time"), 50, 0).Return([]*media.Media{}, nil)
			},
		},
		{
			name:       "next page passes beforeID through",
			principal:  admin,
			search:     "",
			dateFilter: "",
			limit:      50,
			beforeID:   100,
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("FindAllByCursor", mock.Anything, 0, 50, 100).Return([]*media.Media{
					{ID: 99, UserID: 1},
					{ID: 98, UserID: 1},
				}, nil)
			},
			expectedLen: 2,
		},
		{
			name:       "whitespace search is treated as no search",
			principal:  admin,
			search:     "   ",
			dateFilter: "",
			limit:      50,
			beforeID:   0,
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("FindAllByCursor", mock.Anything, 0, 50, 0).Return([]*media.Media{}, nil)
			},
		},
		{
			name:       "repository error - wrapped as failed to search media",
			principal:  admin,
			search:     "",
			dateFilter: "",
			limit:      50,
			beforeID:   0,
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("FindAllByCursor", mock.Anything, 0, 50, 0).Return([]*media.Media(nil), errors.New("database error"))
			},
			expectedErr: errors.New("failed to search media"),
		},
		{
			name:        "error - principal without a usable user ID",
			principal:   kernel.Principal{ID: "not-a-number", Username: "ghost"},
			search:      "",
			dateFilter:  "",
			limit:       50,
			beforeID:    0,
			expectedErr: errors.New("invalid principal ID"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mediamocks.NewMockRepository(t)
			mockStorage := newStubStorage()
			if tt.setupMock != nil {
				tt.setupMock(mockRepo)
			}

			svc := media.NewService(mockRepo, mockStorage, nil)

			got, err := svc.SearchMediaByCursor(context.Background(), tt.principal, tt.search, tt.dateFilter, tt.limit, tt.beforeID)

			if tt.expectedErr != nil {
				require.Error(t, err, "Service.SearchMediaByCursor() expected error, got nil")
				assert.Contains(t, err.Error(), tt.expectedErr.Error(), "Service.SearchMediaByCursor() error message")
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, got)
			if tt.expectedLen > 0 {
				assert.Len(t, got, tt.expectedLen)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_Count(t *testing.T) {
	admin := kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}}
	buyer := kernel.Principal{ID: "7", Username: "buyer", Roles: []string{"Commentator"}}

	tests := []struct {
		name        string
		principal   kernel.Principal
		search      string
		dateFilter  string
		setupMock   func(*mediamocks.MockRepository)
		expected    int
		expectedErr error
	}{
		{
			name:       "success - admin counts the whole library",
			principal:  admin,
			search:     "",
			dateFilter: "",
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("Count", mock.Anything, 0, "", time.Time{}).Return(12, nil)
			},
			expected: 12,
		},
		{
			name:       "success - non-admin counts only their own media",
			principal:  buyer,
			search:     "",
			dateFilter: "",
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("Count", mock.Anything, 7, "", time.Time{}).Return(2, nil)
			},
			expected: 2,
		},
		{
			name:       "search only is forwarded",
			principal:  admin,
			search:     "sunset",
			dateFilter: "",
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("Count", mock.Anything, 0, "sunset", time.Time{}).Return(3, nil)
			},
			expected: 3,
		},
		{
			name:       "date filter only passes a non-zero since",
			principal:  admin,
			search:     "",
			dateFilter: "today",
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("Count", mock.Anything, 0, "", mock.AnythingOfType("time.Time")).Return(5, nil)
			},
			expected: 5,
		},
		{
			name:       "both filters are forwarded",
			principal:  admin,
			search:     "sunset",
			dateFilter: "this_week",
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("Count", mock.Anything, 0, "sunset", mock.AnythingOfType("time.Time")).Return(2, nil)
			},
			expected: 2,
		},
		{
			name:       "whitespace search is treated as no search",
			principal:  admin,
			search:     "   ",
			dateFilter: "",
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("Count", mock.Anything, 0, "", time.Time{}).Return(0, nil)
			},
			expected: 0,
		},
		{
			name:       "repository error - wrapped as failed to count media",
			principal:  admin,
			search:     "",
			dateFilter: "",
			setupMock: func(repo *mediamocks.MockRepository) {
				repo.On("Count", mock.Anything, 0, "", time.Time{}).Return(0, errors.New("database error"))
			},
			expectedErr: errors.New("failed to count media"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mediamocks.NewMockRepository(t)
			mockStorage := newStubStorage()
			tt.setupMock(mockRepo)

			svc := media.NewService(mockRepo, mockStorage, nil)

			got, err := svc.Count(context.Background(), tt.principal, tt.search, tt.dateFilter)

			if tt.expectedErr != nil {
				require.Error(t, err, "Service.Count() expected error, got nil")
				assert.Contains(t, err.Error(), tt.expectedErr.Error(), "Service.Count() error message")
				mockRepo.AssertExpectations(t)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
			mockRepo.AssertExpectations(t)
		})
	}
}
func TestService_Upload_WithThumbnailService(t *testing.T) {
	thumbSvc := thumbnail.NewService()

	imgData := createTestImage(t)
	file, header := createTestFile(t, imgData)
	defer func() { _ = file.Close() }()

	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	svc := media.NewService(mockRepo, mockStorage, thumbSvc)

	req := media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "A test image",
	}

	got, err := svc.Upload(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Variants)
	require.Contains(t, got.Variants, "_thumb")
	assert.Contains(t, got.Variants["_thumb"].FilePath, "_thumb.webp")
	assert.Contains(t, got.Variants["_thumb"].URL, "_thumb")
	assert.Greater(t, got.Variants["_thumb"].Width, 0)
	assert.Greater(t, got.Variants["_thumb"].Height, 0)

	mockRepo.AssertExpectations(t)
}

func TestService_Upload_VariantFilenamePattern(t *testing.T) {
	thumbSvc := thumbnail.NewService()

	imgData := createTestImage(t)
	file, header := createTestFile(t, imgData)
	defer func() { _ = file.Close() }()

	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	svc := media.NewService(mockRepo, mockStorage, thumbSvc)

	req := media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "A test image",
	}

	got, err := svc.Upload(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, got)

	variant := got.Variants["_thumb"]
	assert.Contains(t, variant.FilePath, "_thumb.webp")
}

func TestService_Upload_VariantsNilWhenThumbnailServiceNil(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	svc := media.NewService(mockRepo, mockStorage, nil)

	imgData := createTestImage(t)
	file, header := createTestFile(t, imgData)
	defer func() { _ = file.Close() }()

	req := media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "A test image",
	}

	got, err := svc.Upload(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Empty(t, got.Variants)
}

func TestService_ForceUpload_WithThumbnailService(t *testing.T) {
	thumbSvc := thumbnail.NewService()

	imgData := createTestImage(t)
	file, header := createTestFile(t, imgData)
	defer func() { _ = file.Close() }()

	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	svc := media.NewService(mockRepo, mockStorage, thumbSvc)

	req := media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "Force upload with variants",
	}

	got, err := svc.ForceUpload(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Variants)
	require.Contains(t, got.Variants, "_thumb")
	assert.Greater(t, got.Variants["_thumb"].Width, 0)

	mockRepo.AssertExpectations(t)
}

func TestService_Upload_VariantCleanupOnSaveFailure(t *testing.T) {
	thumbSvc := thumbnail.NewService()

	imgData := createTestImage(t)
	file, header := createTestFile(t, imgData)
	defer func() { _ = file.Close() }()

	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()
	mockStorage.saveErr = errors.New("disk full")
	mockStorage.saveSucceed = 1

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)

	svc := media.NewService(mockRepo, mockStorage, thumbSvc)

	req := media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "Test cleanup",
	}

	_, err := svc.Upload(context.Background(), req)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to save variant")
	require.Len(t, mockStorage.deleted, 1, "failed variant must clean up the saved original")
}

func TestService_Upload_VariantDimensionsCorrect(t *testing.T) {
	thumbSvc := thumbnail.NewService()

	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	c := color.RGBA{255, 0, 0, 255}
	for y := range 600 {
		for x := range 800 {
			img.Set(x, y, c)
		}
	}

	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	require.NoError(t, err)
	imgData := buf.Bytes()

	file, header := createTestFile(t, imgData)
	defer func() { _ = file.Close() }()

	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	svc := media.NewService(mockRepo, mockStorage, thumbSvc)

	req := media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "Large image for resize test",
	}

	got, err := svc.Upload(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Contains(t, got.Variants, "_thumb")
	assert.Equal(t, 64, got.Variants["_thumb"].Width)
	assert.Equal(t, 48, got.Variants["_thumb"].Height)
	require.Contains(t, got.Variants, "_medium")
	assert.Equal(t, 280, got.Variants["_medium"].Width)
	assert.Equal(t, 210, got.Variants["_medium"].Height)
	require.Contains(t, got.Variants, "_large")
	assert.Equal(t, 616, got.Variants["_large"].Width)
	assert.Equal(t, 462, got.Variants["_large"].Height)
}
func TestService_ForceUpload_VariantsNilWhenThumbnailServiceNil(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	mockStorage := newStubStorage()

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	svc := media.NewService(mockRepo, mockStorage, nil)

	imgData := createTestImage(t)
	file, header := createTestFile(t, imgData)
	defer func() { _ = file.Close() }()

	req := media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "Force upload no variants",
	}

	got, err := svc.ForceUpload(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Empty(t, got.Variants)
}

func TestService_InvalidPrincipal(t *testing.T) {
	svc := media.NewService(nil, nil, nil)
	bad := kernel.Principal{ID: "not-a-number", Username: "ghost"}

	tests := []struct {
		name    string
		call    func() error
		wantErr bool
	}{
		{
			name: "delete rejects non-decimal id",
			call: func() error {
				return svc.Delete(context.Background(), 1, bad)
			},
			wantErr: true,
		},
		{
			name: "generate from bytes rejects non-decimal id",
			call: func() error {
				_, err := svc.GenerateFromBytes(context.Background(), nil, bad, "alt", "photo.webp")
				return err
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
