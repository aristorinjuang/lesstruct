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
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/aristorinjuang/lesstruct/internal/domain/media/mocks"
	"github.com/aristorinjuang/lesstruct/internal/domain/thumbnail"
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

func TestService_Upload(t *testing.T) {
	tests := []struct {
		name           string
		altText        string
		wantErr        bool
		expectErr      error
		expectDupMedia *media.Media
		setupMock      func(*mocks.MockRepository, *mocks.MockStorage)
	}{
		{
			name:    "successful upload",
			altText: "A test image",
			wantErr: false,
			setupMock: func(repo *mocks.MockRepository, storage *mocks.MockStorage) {
				repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
				repo.On("Create", mock.Anything, mock.Anything).Return(nil)
				storage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test.webp", nil)
				storage.On("GetURL", mock.Anything).Return("http://localhost:8080/uploads/media/test.webp")
			},
		},
		{
			name:      "missing alt text",
			altText:   "",
			wantErr:   true,
			expectErr: media.ErrInvalidAltText,
			setupMock: func(repo *mocks.MockRepository, storage *mocks.MockStorage) {},
		},
		{
			name:      "whitespace only alt text",
			altText:   "   ",
			wantErr:   true,
			expectErr: media.ErrInvalidAltText,
			setupMock: func(repo *mocks.MockRepository, storage *mocks.MockStorage) {},
		},
		{
			name:           "duplicate upload",
			altText:        "Duplicate image",
			wantErr:        true,
			expectDupMedia: &media.Media{ID: 99, UserID: 1, OriginalFilename: "existing.png", Hash: "duplicate-hash"},
			setupMock: func(repo *mocks.MockRepository, storage *mocks.MockStorage) {
				existing := &media.Media{ID: 99, UserID: 1, OriginalFilename: "existing.png", Hash: "duplicate-hash"}
				repo.On("FindByHash", mock.Anything, mock.Anything).Return(existing, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockStorage := mocks.NewMockStorage(t)
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
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestService_GetByID(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mocks.MockRepository)
		wantErr   bool
		expectErr error
	}{
		{
			name: "found",
			setupMock: func(repo *mocks.MockRepository) {
				mockMedia := &media.Media{
					ID:     1,
					UserID: 1,
					URL:    "http://localhost:8080/uploads/media/test.webp",
				}
				repo.On("FindByID", mock.Anything, 1).Return(mockMedia, nil)
			},
			wantErr: false,
		},
		{
			name: "not found",
			setupMock: func(repo *mocks.MockRepository) {
				repo.On("FindByID", mock.Anything, 1).Return((*media.Media)(nil), media.ErrMediaNotFound)
			},
			wantErr:   true,
			expectErr: media.ErrMediaNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockStorage := mocks.NewMockStorage(t)
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

func TestService_GetAll(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

	mockMedia := &media.Media{
		ID:     1,
		UserID: 1,
		URL:    "http://localhost:8080/uploads/media/test.webp",
	}

	mockRepo.On("FindAll", mock.Anything, 10, 0).Return([]*media.Media{mockMedia}, nil)

	svc := media.NewService(mockRepo, mockStorage, nil)

	got, err := svc.GetAll(context.Background(), 10, 0)

	require.NoError(t, err)
	require.NotEmpty(t, got)

	mockRepo.AssertExpectations(t)
}

func TestService_GetAll_Error(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

	mockRepo.On("FindAll", mock.Anything, 10, 0).Return([]*media.Media(nil), errors.New("database error"))

	svc := media.NewService(mockRepo, mockStorage, nil)

	_, err := svc.GetAll(context.Background(), 10, 0)

	require.Error(t, err)

	mockRepo.AssertExpectations(t)
}

func TestService_ListByCursor(t *testing.T) {
	tests := []struct {
		name        string
		userID      int
		limit       int
		beforeID    int
		setupMock   func(*mocks.MockRepository)
		expectedErr error
		expectedLen int
	}{
		{
			name:     "first page - beforeID 0 returns newest media",
			userID:   1,
			limit:    50,
			beforeID: 0,
			setupMock: func(repo *mocks.MockRepository) {
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
			setupMock: func(repo *mocks.MockRepository) {
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
			setupMock: func(repo *mocks.MockRepository) {
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
			setupMock: func(repo *mocks.MockRepository) {
				repo.On("ListByCursor", mock.Anything, 1, 50, 0).Return([]*media.Media(nil), errors.New("database error"))
			},
			expectedErr: errors.New("failed to list media"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockStorage := mocks.NewMockStorage(t)
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
		setupMock func(*mocks.MockRepository, *mocks.MockStorage)
		wantErr   bool
		expectErr error
	}{
		{
			name:     "successful delete by owner",
			userID:   1,
			userRole: "Contributor",
			setupMock: func(repo *mocks.MockRepository, storage *mocks.MockStorage) {
				mockMedia := &media.Media{
					ID:       1,
					UserID:   1,
					FilePath: "/uploads/media/test.webp",
				}
				repo.On("FindByID", mock.Anything, 1).Return(mockMedia, nil)
				storage.On("Delete", "/uploads/media/test.webp").Return(nil)
				repo.On("DeleteByOwner", mock.Anything, 1, 1).Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "successful delete by admin",
			userID:   2,
			userRole: "Admin",
			setupMock: func(repo *mocks.MockRepository, storage *mocks.MockStorage) {
				mockMedia := &media.Media{
					ID:       1,
					UserID:   1,
					FilePath: "/uploads/media/test.webp",
				}
				repo.On("FindByID", mock.Anything, 1).Return(mockMedia, nil)
				storage.On("Delete", "/uploads/media/test.webp").Return(nil)
				repo.On("DeleteByID", mock.Anything, 1).Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "media not found",
			userID:   1,
			userRole: "Contributor",
			setupMock: func(repo *mocks.MockRepository, storage *mocks.MockStorage) {
				repo.On("FindByID", mock.Anything, 1).Return((*media.Media)(nil), media.ErrMediaNotFound)
			},
			wantErr:   true,
			expectErr: media.ErrMediaNotFound,
		},
		{
			name:     "unauthorized delete - different user non-admin",
			userID:   2,
			userRole: "Contributor",
			setupMock: func(repo *mocks.MockRepository, storage *mocks.MockStorage) {
				mockMedia := &media.Media{ID: 1, UserID: 1, FilePath: "/uploads/media/test.webp"}
				repo.On("FindByID", mock.Anything, 1).Return(mockMedia, nil)
			},
			wantErr:   true,
			expectErr: media.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockStorage := mocks.NewMockStorage(t)
			tt.setupMock(mockRepo, mockStorage)

			svc := media.NewService(mockRepo, mockStorage, nil)

			err := svc.Delete(context.Background(), 1, tt.userID, tt.userRole)

			if (err != nil) != tt.wantErr {
				t.Errorf("Service.Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.expectErr != nil && !errors.Is(err, tt.expectErr) {
				t.Errorf("Service.Delete() error = %v, expectErr %v", err, tt.expectErr)
			}

			mockRepo.AssertExpectations(t)
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestService_Upload_StorageFailure(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockStorage.On("Save", mock.Anything, mock.Anything).Return("", errors.New("storage error"))

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
	mockStorage.AssertExpectations(t)
}

func TestService_Upload_InvalidFileType(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

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
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

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
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

	mockMedia := &media.Media{
		ID:       1,
		UserID:   1,
		FilePath: "/uploads/media/test.webp",
	}

	mockRepo.On("FindByID", mock.Anything, 1).Return(mockMedia, nil)
	mockStorage.On("Delete", "/uploads/media/test.webp").Return(errors.New("storage error"))

	svc := media.NewService(mockRepo, mockStorage, nil)

	err := svc.Delete(context.Background(), 1, 1, "Contributor")

	require.Error(t, err)

	mockRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestService_Upload_InvalidExtension(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

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
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

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
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("database error"))
	mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test.webp", nil)
	mockStorage.On("GetURL", mock.Anything).Return("http://localhost:8080/uploads/media/test.webp")
	mockStorage.On("Delete", "/uploads/media/test.webp").Return(nil)

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

	mockRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestService_Delete_RepositoryDeleteError(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

	mockMedia := &media.Media{
		ID:       1,
		UserID:   1,
		FilePath: "/uploads/media/test.webp",
	}

	mockRepo.On("FindByID", mock.Anything, 1).Return(mockMedia, nil)
	mockStorage.On("Delete", "/uploads/media/test.webp").Return(nil)
	mockRepo.On("DeleteByOwner", mock.Anything, 1, 1).Return(errors.New("database error"))

	svc := media.NewService(mockRepo, mockStorage, nil)

	err := svc.Delete(context.Background(), 1, 1, "Contributor")

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to delete media")

	mockRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestService_Upload_ConvertToWebPError(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

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
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

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
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

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
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

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
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

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
			mockRepo := mocks.NewMockRepository(t)
			mockStorage := mocks.NewMockStorage(t)

			mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
			mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
			mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test.webp", nil)
			mockStorage.On("GetURL", mock.Anything).Return("http://localhost:8080/uploads/media/test.webp")

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
			mockStorage.AssertExpectations(t)
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
	tests := []struct {
		name      string
		filter    string
		wantValid bool
		checkFunc func(t *testing.T, result time.Time)
	}{
		{
			name:      "today returns start of day",
			filter:    "today",
			wantValid: true,
			checkFunc: func(t *testing.T, result time.Time) {
				now := time.Now()
				assert.Equal(t, now.Year(), result.Year())
				assert.Equal(t, now.Month(), result.Month())
				assert.Equal(t, now.Day(), result.Day())
				assert.Equal(t, 0, result.Hour())
				assert.Equal(t, 0, result.Minute())
			},
		},
		{
			name:      "this_week returns start of week",
			filter:    "this_week",
			wantValid: true,
			checkFunc: func(t *testing.T, result time.Time) {
				assert.Equal(t, time.Monday, result.Weekday())
				assert.Equal(t, 0, result.Hour())
			},
		},
		{
			name:      "this_month returns start of month",
			filter:    "this_month",
			wantValid: true,
			checkFunc: func(t *testing.T, result time.Time) {
				now := time.Now()
				assert.Equal(t, 1, result.Day())
				assert.Equal(t, now.Year(), result.Year())
				assert.Equal(t, now.Month(), result.Month())
			},
		},
		{
			name:      "invalid filter returns false",
			filter:    "invalid",
			wantValid: false,
			checkFunc: func(t *testing.T, result time.Time) {
				assert.True(t, result.IsZero())
			},
		},
		{
			name:      "empty filter returns false",
			filter:    "",
			wantValid: false,
			checkFunc: func(t *testing.T, result time.Time) {
				assert.True(t, result.IsZero())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, valid := media.ParseDateFilter(tt.filter)
			assert.Equal(t, tt.wantValid, valid)
			if tt.wantValid {
				tt.checkFunc(t, result)
			}
		})
	}
}

func TestDuplicateMediaError(t *testing.T) {
	existing := &media.Media{
		ID:               42,
		UserID:           1,
		OriginalFilename: "sunset.jpg",
		Hash:             "abc123",
	}

	dupErr := &media.DuplicateMediaError{Existing: existing}

	require.Contains(t, dupErr.Error(), "media already exists: sunset.jpg")
	assert.Equal(t, existing, dupErr.Existing)

	var target *media.DuplicateMediaError
	require.True(t, errors.As(dupErr, &target))
	assert.Equal(t, "sunset.jpg", target.Existing.OriginalFilename)
}

func TestService_ForceUpload(t *testing.T) {
	t.Run("successful force upload with no existing hash", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		mockStorage := mocks.NewMockStorage(t)

		mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
		mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
		mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test.webp", nil)
		mockStorage.On("GetURL", mock.Anything).Return("http://localhost:8080/uploads/media/test.webp")

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
		mockStorage.AssertExpectations(t)
	})

	t.Run("force upload with existing hash generates unique variant", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		mockStorage := mocks.NewMockStorage(t)

		existing := &media.Media{ID: 1, Hash: "existing-hash", OriginalFilename: "existing.png"}

		mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return(existing, nil).Once()
		mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound).Once()
		mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
		mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test.webp", nil)
		mockStorage.On("GetURL", mock.Anything).Return("http://localhost:8080/uploads/media/test.webp")

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
		mockStorage.AssertExpectations(t)
	})

	t.Run("missing alt text returns error", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		mockStorage := mocks.NewMockStorage(t)

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
		mockRepo := mocks.NewMockRepository(t)
		mockStorage := mocks.NewMockStorage(t)

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

func TestService_SearchMedia(t *testing.T) {
	tests := []struct {
		name       string
		search     string
		dateFilter string
		setupMock  func(*mocks.MockRepository)
	}{
		{
			name:       "no filters falls back to FindAll",
			search:     "",
			dateFilter: "",
			setupMock: func(repo *mocks.MockRepository) {
				repo.On("FindAll", mock.Anything, 100, 0).Return([]*media.Media{}, nil)
			},
		},
		{
			name:       "search only calls FindAllByFilename",
			search:     "sunset",
			dateFilter: "",
			setupMock: func(repo *mocks.MockRepository) {
				repo.On("FindAllByFilename", mock.Anything, "sunset", 100, 0).Return([]*media.Media{}, nil)
			},
		},
		{
			name:       "date filter only calls FindAllByDateRange",
			search:     "",
			dateFilter: "today",
			setupMock: func(repo *mocks.MockRepository) {
				repo.On("FindAllByDateRange", mock.Anything, mock.AnythingOfType("time.Time"), 100, 0).Return([]*media.Media{}, nil)
			},
		},
		{
			name:       "both filters calls FindAllByFilenameAndDateRange",
			search:     "sunset",
			dateFilter: "this_week",
			setupMock: func(repo *mocks.MockRepository) {
				repo.On("FindAllByFilenameAndDateRange", mock.Anything, "sunset", mock.AnythingOfType("time.Time"), 100, 0).Return([]*media.Media{}, nil)
			},
		},
		{
			name:       "whitespace search is treated as no search",
			search:     "   ",
			dateFilter: "",
			setupMock: func(repo *mocks.MockRepository) {
				repo.On("FindAll", mock.Anything, 100, 0).Return([]*media.Media{}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockStorage := mocks.NewMockStorage(t)
			tt.setupMock(mockRepo)

			svc := media.NewService(mockRepo, mockStorage, nil)

			got, err := svc.SearchMedia(context.Background(), tt.search, tt.dateFilter, 100, 0)

			require.NoError(t, err)
			assert.NotNil(t, got)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_Upload_WithThumbnailService(t *testing.T) {
	thumbSvc := thumbnail.NewService()

	imgData := createTestImage(t)
	file, header := createTestFile(t, imgData)
	defer func() { _ = file.Close() }()

	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test.webp", nil).Once()
	mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test_thumb.webp", nil).Once()
	mockStorage.On("GetURL", "/uploads/media/test.webp").Return("http://localhost:8080/uploads/media/test.webp")
	mockStorage.On("GetURL", "/uploads/media/test_thumb.webp").Return("http://localhost:8080/uploads/media/test_thumb.webp")

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
	assert.Equal(t, "/uploads/media/test_thumb.webp", got.Variants["_thumb"].FilePath)
	assert.Equal(t, "http://localhost:8080/uploads/media/test_thumb.webp", got.Variants["_thumb"].URL)
	assert.Greater(t, got.Variants["_thumb"].Width, 0)
	assert.Greater(t, got.Variants["_thumb"].Height, 0)

	mockRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestService_Upload_VariantFilenamePattern(t *testing.T) {
	thumbSvc := thumbnail.NewService()

	imgData := createTestImage(t)
	file, header := createTestFile(t, imgData)
	defer func() { _ = file.Close() }()

	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test.webp", nil).Once()
	mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test_thumb.webp", nil).Once()
	mockStorage.On("GetURL", "/uploads/media/test.webp").Return("http://localhost:8080/uploads/media/test.webp")
	mockStorage.On("GetURL", "/uploads/media/test_thumb.webp").Return("http://localhost:8080/uploads/media/test_thumb.webp")

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
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test.webp", nil)
	mockStorage.On("GetURL", mock.Anything).Return("http://localhost:8080/uploads/media/test.webp")

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

	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test.webp", nil).Once()
	mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test_thumb.webp", nil).Once()
	mockStorage.On("GetURL", "/uploads/media/test.webp").Return("http://localhost:8080/uploads/media/test.webp")
	mockStorage.On("GetURL", "/uploads/media/test_thumb.webp").Return("http://localhost:8080/uploads/media/test_thumb.webp")

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
	mockStorage.AssertExpectations(t)
}

func TestService_Upload_VariantCleanupOnSaveFailure(t *testing.T) {
	thumbSvc := thumbnail.NewService()

	imgData := createTestImage(t)
	file, header := createTestFile(t, imgData)
	defer func() { _ = file.Close() }()

	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/orig.webp", nil).Once()
	mockStorage.On("Save", mock.Anything, mock.Anything).Return("", errors.New("disk full")).Once()
	mockStorage.On("Delete", "/uploads/media/orig.webp").Return(nil)

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

	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test.webp", nil).Once()
	mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test_thumb.webp", nil).Once()
	mockStorage.On("GetURL", "/uploads/media/test.webp").Return("http://localhost:8080/uploads/media/test.webp")
	mockStorage.On("GetURL", "/uploads/media/test_thumb.webp").Return("http://localhost:8080/uploads/media/test_thumb.webp")

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
	assert.Equal(t, 370, got.Variants["_thumb"].Width)
	assert.Equal(t, 277, got.Variants["_thumb"].Height)
}
func TestService_ForceUpload_VariantsNilWhenThumbnailServiceNil(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	mockStorage := mocks.NewMockStorage(t)

	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	mockStorage.On("Save", mock.Anything, mock.Anything).Return("/uploads/media/test.webp", nil)
	mockStorage.On("GetURL", mock.Anything).Return("http://localhost:8080/uploads/media/test.webp")

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
