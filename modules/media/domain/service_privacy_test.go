package media_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	media "github.com/aristorinjuang/lesstruct/modules/media/domain"
	mediamocks "github.com/aristorinjuang/lesstruct/modules/media/domain/mocks"
	thumbnail "github.com/aristorinjuang/lesstruct/modules/media/domain/thumbnail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// stubRedactor is a programmable media Redactor double.
type stubRedactor struct {
	redacted []byte
	err      error
	calls    int
}

func (s *stubRedactor) Redact(_ context.Context, imageBytes []byte) ([]byte, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.redacted != nil {
		return s.redacted, nil
	}
	return imageBytes, nil
}

func createRedactPNG(t *testing.T) []byte {
	t.Helper()
	return createRedactPNGWith(t, 0)
}

// createRedactPNGWith builds the same 64x64 gradient with the red channel
// XORed by base, so tests can mint byte-distinct raw/redacted fixtures.
func createRedactPNGWith(t *testing.T, base uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.Set(x, y, color.RGBA{uint8(x*4) ^ base, uint8(y * 4), 200, 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestServiceRedactedUsesBlurredHashAndVariants proves the redacted bytes —
// not the raw upload — drive hashing, the stored original, and every
// thumbnail variant. A raw-hash/raw-variant regression would ship
// unblurred pixels while these tests stay green if they shared fixtures.
func TestServiceRedactedUsesBlurredHashAndVariants(t *testing.T) {
	tests := []struct {
		name   string
		upload func(svc *media.Service, req media.UploadRequest) (*media.Media, error)
	}{
		{
			name: "Upload",
			upload: func(svc *media.Service, req media.UploadRequest) (*media.Media, error) {
				return svc.Upload(context.Background(), req)
			},
		},
		{
			name: "ForceUpload",
			upload: func(svc *media.Service, req media.UploadRequest) (*media.Media, error) {
				return svc.ForceUpload(context.Background(), req)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := createRedactPNGWith(t, 0)
			redacted := createRedactPNGWith(t, 0x5A)
			require.NotEqual(t, raw, redacted)
			redactor := &stubRedactor{redacted: redacted}
			expectedHash := sha256Hex(redacted)

			var gotHash string
			mockRepo := mediamocks.NewMockRepository(t)
			mockRepo.On("FindByHash", mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) { gotHash = args.String(1) }).
				Return((*media.Media)(nil), media.ErrMediaNotFound)
			var created *media.Media
			mockRepo.On("Create", mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) { created = args.Get(1).(*media.Media) }).
				Return(nil)

			thumbnailService := thumbnail.NewService()
			storage := newStubStorage()
			svc := media.NewService(mockRepo, storage, thumbnailService, media.WithRedactor(redactor))

			file, header := createTestFile(t, raw)
			defer func() { _ = file.Close() }()

			got, err := tt.upload(svc, media.UploadRequest{
				File:       file,
				FileHeader: header,
				UserID:     1,
				AltText:    "alt",
			})
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, expectedHash, gotHash)
			require.NotNil(t, created)
			assert.Equal(t, expectedHash, created.Hash)
			assert.Equal(t, expectedHash[:16]+".webp", created.Filename)

			processor := media.NewProcessor()
			wantMain, _, err := processor.ConvertToWebP(bytes.NewReader(redacted))
			require.NoError(t, err)
			assert.Equal(t, wantMain, storage.saved[created.Filename])

			for _, tc := range thumbnailService.GetAll() {
				wantVariant, _, err := processor.Resize(bytes.NewReader(redacted), tc.MaxWidth)
				require.NoError(t, err)
				assert.Equal(t, wantVariant, storage.saved[expectedHash[:16]+tc.Suffix+".webp"])
			}
			assert.Len(t, storage.saved, 1+len(thumbnailService.GetAll()))
		})
	}
}

func TestServiceUploadRedactedSuccess(t *testing.T) {
	raw := createRedactPNG(t)
	redacted := createRedactPNG(t)
	redactor := &stubRedactor{redacted: redacted}

	mockRepo := mediamocks.NewMockRepository(t)
	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	storage := newStubStorage()
	svc := media.NewService(mockRepo, storage, thumbnail.NewService(), media.WithRedactor(redactor))

	file, header := createTestFile(t, raw)
	defer func() { _ = file.Close() }()

	got, err := svc.Upload(context.Background(), media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "alt",
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 1, redactor.calls)
	assert.NotEmpty(t, storage.saved)
}

func TestServiceUploadRedactedFailure(t *testing.T) {
	raw := createRedactPNG(t)
	redactor := &stubRedactor{err: errors.New("scan boom")}

	mockRepo := mediamocks.NewMockRepository(t)
	storage := newStubStorage()
	svc := media.NewService(mockRepo, storage, nil, media.WithRedactor(redactor))

	file, header := createTestFile(t, raw)
	defer func() { _ = file.Close() }()

	_, err := svc.Upload(context.Background(), media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "alt",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, media.ErrPrivacyScanFailed)
	assert.Empty(t, storage.saved)
	mockRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestServiceForceUploadRedacted(t *testing.T) {
	raw := createRedactPNG(t)
	redacted := createRedactPNG(t)
	redactor := &stubRedactor{redacted: redacted}

	mockRepo := mediamocks.NewMockRepository(t)
	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	storage := newStubStorage()
	svc := media.NewService(mockRepo, storage, thumbnail.NewService(), media.WithRedactor(redactor))

	file, header := createTestFile(t, raw)
	defer func() { _ = file.Close() }()

	got, err := svc.ForceUpload(context.Background(), media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "alt",
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 1, redactor.calls)
}

func TestServiceForceUploadRedactedFailure(t *testing.T) {
	raw := createRedactPNG(t)
	redactor := &stubRedactor{err: errors.New("scan boom")}

	mockRepo := mediamocks.NewMockRepository(t)
	storage := newStubStorage()
	svc := media.NewService(mockRepo, storage, nil, media.WithRedactor(redactor))

	file, header := createTestFile(t, raw)
	defer func() { _ = file.Close() }()

	_, err := svc.ForceUpload(context.Background(), media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "alt",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, media.ErrPrivacyScanFailed)
	assert.Empty(t, storage.saved)
}

func TestWithRedactorNilKeepsLegacy(t *testing.T) {
	mockRepo := mediamocks.NewMockRepository(t)
	storage := newStubStorage()
	svc := media.NewService(mockRepo, storage, nil, media.WithRedactor(nil))
	require.NotNil(t, svc)
}

// failSecondReadFile passes validation (first 512 bytes valid) then fails
// the redaction ReadAll.
type failSecondReadFile struct {
	data  []byte
	reads int
}

func (f *failSecondReadFile) Read(p []byte) (int, error) {
	f.reads++
	if f.reads == 1 {
		n := copy(p, f.data[:512])
		return n, nil
	}
	return 0, errors.New("forced second-read error")
}

func (f *failSecondReadFile) Seek(_ int64, _ int) (int64, error) {
	return 0, nil
}

func (f *failSecondReadFile) ReadAt(p []byte, _ int64) (int, error) {
	return 0, errors.New("forced readat error")
}

func (failSecondReadFile) Close() error {
	return nil
}

func TestServiceUploadRedactedReadError(t *testing.T) {
	raw := createRedactPNG(t)
	padded := append(append([]byte(nil), raw...), make([]byte, 512)...)
	redactor := &stubRedactor{redacted: raw}

	mockRepo := mediamocks.NewMockRepository(t)
	storage := newStubStorage()
	svc := media.NewService(mockRepo, storage, nil, media.WithRedactor(redactor))

	// Valid header from a real file; the failing file supplies valid bytes
	// for validation then errors on the redaction ReadAll.
	tmpFile, header := createTestFile(t, raw)
	_ = tmpFile.Close()
	file := &failSecondReadFile{data: padded}

	_, err := svc.Upload(context.Background(), media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "alt",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestServiceUploadRedactedDuplicate(t *testing.T) {
	raw := createRedactPNG(t)
	redactor := &stubRedactor{redacted: raw}

	mockRepo := mediamocks.NewMockRepository(t)
	existing := &media.Media{ID: 99, OriginalFilename: "existing.png"}
	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return(existing, nil)

	storage := newStubStorage()
	svc := media.NewService(mockRepo, storage, nil, media.WithRedactor(redactor))

	file, header := createTestFile(t, raw)
	defer func() { _ = file.Close() }()

	_, err := svc.Upload(context.Background(), media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "alt",
	})
	require.Error(t, err)
	var dupErr *media.DuplicateMediaError
	require.True(t, errors.As(err, &dupErr))
}

func TestServiceUploadRedactedConvertError(t *testing.T) {
	raw := createRedactPNG(t)
	redactor := &stubRedactor{redacted: []byte("not an image")}

	mockRepo := mediamocks.NewMockRepository(t)
	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)

	storage := newStubStorage()
	svc := media.NewService(mockRepo, storage, nil, media.WithRedactor(redactor))

	file, header := createTestFile(t, raw)
	defer func() { _ = file.Close() }()

	_, err := svc.Upload(context.Background(), media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "alt",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to convert to WebP")
}

func TestServiceUploadRedactedPersistError(t *testing.T) {
	raw := createRedactPNG(t)
	redactor := &stubRedactor{redacted: raw}

	mockRepo := mediamocks.NewMockRepository(t)
	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db boom"))

	storage := newStubStorage()
	svc := media.NewService(mockRepo, storage, nil, media.WithRedactor(redactor))

	file, header := createTestFile(t, raw)
	defer func() { _ = file.Close() }()

	_, err := svc.Upload(context.Background(), media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "alt",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create media record")
}

func TestServiceForceUploadRedactedUniqueHashError(t *testing.T) {
	raw := createRedactPNG(t)
	redactor := &stubRedactor{redacted: raw}

	mockRepo := mediamocks.NewMockRepository(t)
	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), errors.New("db boom"))

	storage := newStubStorage()
	svc := media.NewService(mockRepo, storage, nil, media.WithRedactor(redactor))

	file, header := createTestFile(t, raw)
	defer func() { _ = file.Close() }()

	_, err := svc.ForceUpload(context.Background(), media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "alt",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate unique hash")
}

func TestServiceForceUploadRedactedConvertError(t *testing.T) {
	raw := createRedactPNG(t)
	redactor := &stubRedactor{redacted: []byte("not an image")}

	mockRepo := mediamocks.NewMockRepository(t)
	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)

	storage := newStubStorage()
	svc := media.NewService(mockRepo, storage, nil, media.WithRedactor(redactor))

	file, header := createTestFile(t, raw)
	defer func() { _ = file.Close() }()

	_, err := svc.ForceUpload(context.Background(), media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "alt",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to convert to WebP")
}

func TestServiceForceUploadRedactedReadError(t *testing.T) {
	raw := createRedactPNG(t)
	padded := append(append([]byte(nil), raw...), make([]byte, 512)...)
	redactor := &stubRedactor{redacted: raw}

	mockRepo := mediamocks.NewMockRepository(t)
	storage := newStubStorage()
	svc := media.NewService(mockRepo, storage, nil, media.WithRedactor(redactor))

	tmpFile, header := createTestFile(t, raw)
	_ = tmpFile.Close()
	file := &failSecondReadFile{data: padded}

	_, err := svc.ForceUpload(context.Background(), media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "alt",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestServiceForceUploadRedactedPersistError(t *testing.T) {
	raw := createRedactPNG(t)
	redactor := &stubRedactor{redacted: raw}

	mockRepo := mediamocks.NewMockRepository(t)
	mockRepo.On("FindByHash", mock.Anything, mock.Anything).Return((*media.Media)(nil), media.ErrMediaNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db boom"))

	storage := newStubStorage()
	svc := media.NewService(mockRepo, storage, nil, media.WithRedactor(redactor))

	file, header := createTestFile(t, raw)
	defer func() { _ = file.Close() }()

	_, err := svc.ForceUpload(context.Background(), media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     1,
		AltText:    "alt",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create media record")
}
