package media_test

import (
	"context"
	"errors"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"

	media "github.com/aristorinjuang/lesstruct/modules/media/domain"
	mediamocks "github.com/aristorinjuang/lesstruct/modules/media/domain/mocks"
	thumbnail "github.com/aristorinjuang/lesstruct/modules/media/domain/thumbnail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// failingSeekFile fails every Seek from failFrom onward, simulating a reader
// that breaks after the pipeline has already rewound it several times.
type failingSeekFile struct {
	*os.File
	failFrom int
	seeks    int
}

func (f *failingSeekFile) Seek(offset int64, whence int) (int64, error) {
	f.seeks++
	if f.seeks >= f.failFrom {
		return 0, errors.New("seek error")
	}
	return f.File.Seek(offset, whence)
}

// twoVariantThumbnailService loads a thumbnail service with two configs so
// rollback tests exercise a loop that already saved one variant.
func twoVariantThumbnailService(t *testing.T) *thumbnail.Service {
	t.Helper()

	path := filepath.Join(t.TempDir(), "thumbnails.toml")
	require.NoError(t, os.WriteFile(path, []byte(`[[thumbnail]]
max_width = 370
suffix = "_thumb"

[[thumbnail]]
max_width = 800
suffix = "_large"
`), 0o600))

	service := thumbnail.NewService()
	require.NoError(t, service.LoadConfigFromFile(path))
	return service
}

func TestService_Upload_ContentMismatch(t *testing.T) {
	tests := []struct {
		name            string
		data            []byte
		wantErrContains string
	}{
		{
			name:            "supported extension with text content",
			data:            []byte("this is definitely not an image"),
			wantErrContains: "file content does not match a supported image type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mediamocks.NewMockRepository(t)
			storage := newStubStorage()
			svc := media.NewService(repo, storage, nil)

			file, header := createTestFile(t, tt.data)
			defer func() { _ = file.Close() }()

			req := media.UploadRequest{
				File:       file,
				FileHeader: header,
				UserID:     1,
				AltText:    "Disguised file",
			}

			got, err := svc.Upload(context.Background(), req)

			require.Error(t, err)
			assert.Nil(t, got)
			assert.Contains(t, err.Error(), tt.wantErrContains)
		})
	}
}

func TestService_Uploads_VariantResizeFailure(t *testing.T) {
	tests := []struct {
		name     string
		force    bool
		failFrom int
	}{
		{
			name:     "upload rewinds into a variant resize failure",
			force:    false,
			failFrom: 7,
		},
		{
			name:     "force upload rewinds into a variant resize failure",
			force:    true,
			failFrom: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mediamocks.NewMockRepository(t)
			storage := newStubStorage()
			repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)

			svc := media.NewService(repo, storage, thumbnail.NewService())

			file, header := createTestFile(t, createTestImage(t))
			defer func() { _ = file.Close() }()
			wrapped := &failingSeekFile{File: file, failFrom: tt.failFrom}

			req := media.UploadRequest{
				File:       wrapped,
				FileHeader: header,
				UserID:     1,
				AltText:    "Variant failure",
			}

			var (
				got *media.Media
				err error
			)
			if tt.force {
				got, err = svc.ForceUpload(context.Background(), req)
			} else {
				got, err = svc.Upload(context.Background(), req)
			}

			require.Error(t, err)
			assert.Nil(t, got)
			assert.Contains(t, err.Error(), "failed to generate variant")
			assert.NotEmpty(t, storage.deleted, "the saved original must be rolled back")
		})
	}
}

func TestService_Upload_CreateFailureRollsBackVariants(t *testing.T) {
	repo := mediamocks.NewMockRepository(t)
	storage := newStubStorage()
	repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	repo.On("Create", mock.Anything, mock.Anything).Return(errors.New("insert failed"))

	svc := media.NewService(repo, storage, twoVariantThumbnailService(t))

	file, header := createTestFile(t, createTestImage(t))
	defer func() { _ = file.Close() }()

	got, err := svc.Upload(context.Background(), media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "Rollback variants",
	})

	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "failed to create media record")
	assert.Len(t, storage.deleted, 3, "the original and both variants must be rolled back")
}

func TestService_ForceUpload_Failures(t *testing.T) {
	imgData := createTestImage(t)
	truncatedPNG := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D,
	}

	tests := []struct {
		name            string
		altText         string
		fileData        []byte
		failHashRead    bool
		failSeekFrom    int
		cancelled       bool
		thumbs          bool
		setup           func(*mediamocks.MockRepository, *stubStorage)
		wantErr         error
		wantErrContains string
	}{
		{
			name:    "rejects blank alt text",
			altText: "   ",
			setup:   func(*mediamocks.MockRepository, *stubStorage) {},
			wantErr: media.ErrInvalidAltText,
		},
		{
			name:      "cancelled context stops the unique-hash probe",
			altText:   "Force upload",
			cancelled: true,
			setup:     func(*mediamocks.MockRepository, *stubStorage) {},
			wantErr:   context.Canceled,
		},
		{
			name:            "hash generation failure",
			altText:         "Force upload",
			failHashRead:    true,
			setup:           func(*mediamocks.MockRepository, *stubStorage) {},
			wantErrContains: "failed to generate hash",
		},
		{
			name:    "unique hash lookup failure",
			altText: "Force upload",
			setup: func(repo *mediamocks.MockRepository, _ *stubStorage) {
				repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), errors.New("database down"))
			},
			wantErrContains: "failed to generate unique hash",
		},
		{
			name:     "webp conversion failure",
			altText:  "Force upload",
			fileData: truncatedPNG,
			setup: func(repo *mediamocks.MockRepository, _ *stubStorage) {
				repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
			},
			wantErrContains: "failed to convert to WebP",
		},
		{
			name:    "main save failure",
			altText: "Force upload",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
				storage.saveErr = errors.New("disk full")
			},
			wantErrContains: "failed to save file",
		},
		{
			name:         "variant resize failure",
			altText:      "Force upload",
			failSeekFrom: 7,
			thumbs:       true,
			setup: func(repo *mediamocks.MockRepository, _ *stubStorage) {
				repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
			},
			wantErrContains: "failed to generate variant",
		},
		{
			name:    "variant save failure",
			altText: "Force upload",
			thumbs:  true,
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
				storage.saveErr = errors.New("disk full")
				storage.saveSucceed = 1
			},
			wantErrContains: "failed to save variant",
		},
		{
			name:    "record insert failure",
			altText: "Force upload",
			setup: func(repo *mediamocks.MockRepository, _ *stubStorage) {
				repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
				repo.On("Create", mock.Anything, mock.Anything).Return(errors.New("insert failed"))
			},
			wantErrContains: "failed to create media record",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fileData == nil {
				tt.fileData = imgData
			}

			repo := mediamocks.NewMockRepository(t)
			storage := newStubStorage()
			tt.setup(repo, storage)

			var thumbs *thumbnail.Service
			if tt.thumbs {
				thumbs = thumbnail.NewService()
			}
			svc := media.NewService(repo, storage, thumbs)

			rawFile, header := createTestFile(t, tt.fileData)
			defer func() { _ = rawFile.Close() }()

			var file multipart.File = rawFile
			if tt.failHashRead {
				file = &mockFileThatFailsAfterValidation{File: rawFile, readsBeforeFailure: 1}
			}
			if tt.failSeekFrom > 0 {
				file = &failingSeekFile{File: rawFile, failFrom: tt.failSeekFrom}
			}

			ctx := context.Background()
			if tt.cancelled {
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}

			got, err := svc.ForceUpload(ctx, media.UploadRequest{
				File:       file,
				FileHeader: header,
				UserID:     1,
				AltText:    tt.altText,
			})

			assert.Nil(t, got)
			require.Error(t, err)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.Contains(t, err.Error(), tt.wantErrContains)
		})
	}
}

func TestService_GenerateFromBytes(t *testing.T) {
	imgData := createTestImage(t)
	truncatedPNG := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D,
	}
	existing := &media.Media{ID: 9, OriginalFilename: "existing.webp", Hash: "existing-hash"}

	tests := []struct {
		name            string
		imageBytes      []byte
		altText         string
		thumbs          bool
		setup           func(*mediamocks.MockRepository, *stubStorage)
		wantDuplicate   *media.Media
		wantErr         error
		wantErrContains string
		check           func(*testing.T, *media.Media)
	}{
		{
			name:       "success without thumbnails",
			imageBytes: imgData,
			altText:    "  AI generated  ",
			setup: func(repo *mediamocks.MockRepository, _ *stubStorage) {
				repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
				repo.On("Create", mock.Anything, mock.Anything).Return(nil)
			},
			check: func(t *testing.T, got *media.Media) {
				assert.Equal(t, 7, got.UserID)
				assert.Equal(t, "AI generated", got.AltText)
				assert.Equal(t, "generated_image.png", got.OriginalFilename)
				assert.Equal(t, media.MimeTypeWebP, got.MimeType)
				assert.True(t, got.IsWebP)
				assert.NotEmpty(t, got.Filename)
				assert.NotEmpty(t, got.FilePath)
				assert.NotEmpty(t, got.Hash)
				assert.Empty(t, got.Variants)
			},
		},
		{
			name:       "success with thumbnails",
			imageBytes: imgData,
			altText:    "AI generated",
			thumbs:     true,
			setup: func(repo *mediamocks.MockRepository, _ *stubStorage) {
				repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
				repo.On("Create", mock.Anything, mock.Anything).Return(nil)
			},
			check: func(t *testing.T, got *media.Media) {
				require.Len(t, got.Variants, 2)
				assert.Contains(t, got.Variants, "_thumb")
				assert.Contains(t, got.Variants, "_large")
			},
		},
		{
			name:       "rejects blank alt text",
			imageBytes: imgData,
			altText:    "   ",
			setup:      func(*mediamocks.MockRepository, *stubStorage) {},
			wantErr:    media.ErrInvalidAltText,
		},
		{
			name:            "rejects unreadable image bytes",
			imageBytes:      truncatedPNG,
			altText:         "AI generated",
			setup:           func(*mediamocks.MockRepository, *stubStorage) {},
			wantErrContains: "failed to convert to WebP",
		},
		{
			name:       "rejects a duplicate payload",
			imageBytes: imgData,
			altText:    "AI generated",
			setup: func(repo *mediamocks.MockRepository, _ *stubStorage) {
				repo.On("FindByHash", mock.Anything, mock.Anything).Return(existing, nil)
			},
			wantDuplicate: existing,
		},
		{
			name:       "main save failure",
			imageBytes: imgData,
			altText:    "AI generated",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
				storage.saveErr = errors.New("disk full")
			},
			wantErrContains: "failed to save file",
		},
		{
			name:       "variant save failure",
			imageBytes: imgData,
			altText:    "AI generated",
			thumbs:     true,
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
				storage.saveErr = errors.New("disk full")
				storage.saveSucceed = 1
			},
			wantErrContains: "failed to save variant",
		},
		{
			name:       "record insert failure",
			imageBytes: imgData,
			altText:    "AI generated",
			setup: func(repo *mediamocks.MockRepository, _ *stubStorage) {
				repo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
				repo.On("Create", mock.Anything, mock.Anything).Return(errors.New("insert failed"))
			},
			wantErrContains: "failed to create media record",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mediamocks.NewMockRepository(t)
			storage := newStubStorage()
			tt.setup(repo, storage)

			var thumbs *thumbnail.Service
			if tt.thumbs {
				thumbs = twoVariantThumbnailService(t)
			}
			svc := media.NewService(repo, storage, thumbs)

			got, err := svc.GenerateFromBytes(
				context.Background(),
				tt.imageBytes,
				testPrincipal(7, "Contributor"),
				tt.altText,
				"generated image.png",
			)

			if tt.wantErr != nil {
				assert.Nil(t, got)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			if tt.wantDuplicate != nil {
				var dupErr *media.DuplicateMediaError
				require.ErrorAs(t, err, &dupErr)
				assert.Equal(t, tt.wantDuplicate, dupErr.Existing)
				return
			}
			if tt.wantErrContains != "" {
				assert.Nil(t, got)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			tt.check(t, got)
		})
	}
}

func TestService_Delete_AdminAndRoleless(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		setup   func(*mediamocks.MockRepository, *stubStorage)
		wantErr string
	}{
		{
			name: "administrator storage failure",
			role: "Admin",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				storage.deleteErr = errors.New("disk error")
				repo.On("FindByID", mock.Anything, 1).Return(&media.Media{ID: 1, UserID: 1, FilePath: "/uploads/media/x.webp"}, nil)
			},
			wantErr: "failed to delete file",
		},
		{
			name: "administrator repository failure",
			role: "Admin",
			setup: func(repo *mediamocks.MockRepository, _ *stubStorage) {
				repo.On("FindByID", mock.Anything, 1).Return(&media.Media{ID: 1, UserID: 1, FilePath: "/uploads/media/x.webp"}, nil)
				repo.On("DeleteByID", mock.Anything, 1).Return(errors.New("database error"))
			},
			wantErr: "failed to delete media",
		},
		{
			name: "roleless owner deletes own media",
			role: "",
			setup: func(repo *mediamocks.MockRepository, _ *stubStorage) {
				repo.On("FindByID", mock.Anything, 1).Return(&media.Media{ID: 1, UserID: 1, FilePath: "/uploads/media/x.webp"}, nil)
				repo.On("DeleteByOwner", mock.Anything, 1, 1).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mediamocks.NewMockRepository(t)
			storage := newStubStorage()
			tt.setup(repo, storage)

			svc := media.NewService(repo, storage, nil)
			err := svc.Delete(context.Background(), 1, testPrincipal(1, tt.role))

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
