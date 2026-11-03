package profilepicture_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	"github.com/aristorinjuang/lesstruct/modules/users/domain/profilepicture"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubProfileRedactor is a programmable profile-picture Redactor double.
type stubProfileRedactor struct {
	redacted []byte
	err      error
	calls    int
}

func (s *stubProfileRedactor) Redact(_ context.Context, imageBytes []byte) ([]byte, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.redacted != nil {
		return s.redacted, nil
	}
	return imageBytes, nil
}

// createInvertedTestImage builds the same gradient with inverted channels,
// so tests can mint a byte-distinct redacted fixture.
func createInvertedTestImage(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{uint8(255 - x%256), uint8(255 - y%256), 128, 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// TestServiceUploadUsesRedactedBytes proves the redacted bytes — not the raw
// upload — flow into the crop/convert output. A pass-through regression
// would store the raw crop while the old identical-fixture test stayed
// green.
func TestServiceUploadUsesRedactedBytes(t *testing.T) {
	storage := &stubStorage{}
	repo := &stubUserRepo{}
	raw := createTestImage(t, 200, 200)
	redacted := createInvertedTestImage(t, 200, 200)
	require.NotEqual(t, raw, redacted)
	redactor := &stubProfileRedactor{redacted: redacted}

	service := profilepicture.NewService(repo, storage, profilepicture.NewProcessor(), profilepicture.WithRedactor(redactor))
	file, header := createMultipartFile(t, raw)
	defer func() { _ = file.Close() }()

	url, err := service.Upload(context.Background(), testPrincipal(), file, header)
	require.NoError(t, err)
	assert.NotEmpty(t, url)
	assert.Equal(t, 1, redactor.calls)

	want, err := profilepicture.NewProcessor().CropAndConvertToWebP(bytes.NewReader(redacted), profilepicture.ProfilePictureSize)
	require.NoError(t, err)
	require.Len(t, storage.files, 1)
	for _, got := range storage.files {
		assert.Equal(t, want, got)
	}
}

func TestServiceUploadRedactedFailure(t *testing.T) {
	storage := &stubStorage{}
	repo := &stubUserRepo{}
	raw := createTestImage(t, 200, 200)
	redactor := &stubProfileRedactor{err: errors.New("scan boom")}

	service := profilepicture.NewService(repo, storage, profilepicture.NewProcessor(), profilepicture.WithRedactor(redactor))
	file, header := createMultipartFile(t, raw)
	defer func() { _ = file.Close() }()

	_, err := service.Upload(context.Background(), testPrincipal(), file, header)
	require.Error(t, err)
	assert.ErrorIs(t, err, mediadomain.ErrPrivacyScanFailed)
	assert.Empty(t, storage.files)
	assert.Equal(t, 1, redactor.calls)
}

func TestWithRedactorNilKeepsLegacy(t *testing.T) {
	service := profilepicture.NewService(&stubUserRepo{}, &stubStorage{}, profilepicture.NewProcessor(), profilepicture.WithRedactor(nil))
	require.NotNil(t, service)
}

// failSecondRead fails the ReadAll in the redaction path while passing
// validation: the first Read returns valid bytes, Seek resets, the next
// Read fails.
type failSecondRead struct {
	data      []byte
	reads     int
	seekCalls int
}

func (f *failSecondRead) Read(p []byte) (int, error) {
	f.reads++
	if f.reads == 1 {
		n := copy(p, f.data[:512])
		return n, nil
	}
	return 0, errors.New("forced second-read error")
}

func (f *failSecondRead) Seek(_ int64, _ int) (int64, error) {
	f.seekCalls++
	return 0, nil
}

func (failSecondRead) Close() error {
	return nil
}

func (f *failSecondRead) ReadAt(p []byte, _ int64) (int, error) {
	return 0, errors.New("forced readat error")
}

func TestServiceUploadRedactedReadError(t *testing.T) {
	storage := &stubStorage{}
	repo := &stubUserRepo{}
	raw := createTestImage(t, 200, 200)
	// Pad to ensure at least 512 bytes for the first Read.
	padded := append(append([]byte(nil), raw...), make([]byte, 512)...)
	redactor := &stubProfileRedactor{redacted: raw}

	service := profilepicture.NewService(repo, storage, profilepicture.NewProcessor(), profilepicture.WithRedactor(redactor))
	header := testFileHeader(int64(len(padded)), "image/png")

	_, err := service.Upload(context.Background(), testPrincipal(), &failSecondRead{data: padded}, header)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
	assert.Equal(t, 0, redactor.calls)
	assert.Empty(t, storage.files)
}
