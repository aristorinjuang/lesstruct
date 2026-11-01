package media_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"testing"

	media "github.com/aristorinjuang/lesstruct/modules/media/domain"
	mediamocks "github.com/aristorinjuang/lesstruct/modules/media/domain/mocks"
	"github.com/deepteams/webp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// losslessWebPBytes encodes a noisy-alpha image as WebP, matching what the
// pipeline stores, so the re-encode path exercises the sanitize-only
// passthrough end to end (metadata-free input round-trips byte-identical).
func losslessWebPBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			a := uint8(128)
			if (x+y)%3 == 0 {
				a = 200
			}
			img.Set(x, y, color.RGBA{uint8((x * y) % 256), uint8((x + y) % 256), 128, a})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, webp.Encode(&buf, img, &webp.EncoderOptions{Quality: 80, Method: 4}))
	return buf.Bytes()
}

func TestService_ReencodeLibrary(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(*mediamocks.MockRepository, *stubStorage)
		wantProcessed   int
		wantSkipped     int
		wantErr         bool
		wantBytesShrink bool
	}{
		{
			name: "success - single record no variants rewritten in place",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				raw := losslessWebPBytes(t, 320, 200)
				storage.saved["abc123.webp"] = raw
				m := &media.Media{
					ID:       2,
					Filename: "abc123.webp",
					FilePath: "/uploads/media/abc123.webp",
					Hash:     "abc123hash00000000",
					Variants: map[string]media.MediaVariant{},
				}
				repo.On("FindAllByCursor", mock.Anything, 0, 100, 0).Return([]*media.Media{m}, nil).Once()
			},
			wantProcessed:   1,
			wantSkipped:     0,
			wantErr:         false,
			wantBytesShrink: true,
		},
		{
			name: "success - original plus variant rewritten",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				raw := losslessWebPBytes(t, 640, 360)
				variant := losslessWebPBytes(t, 64, 36)
				storage.saved["def456.webp"] = raw
				storage.saved["def456_thumb.webp"] = variant
				m := &media.Media{
					ID:       5,
					Filename: "def456.webp",
					FilePath: "/uploads/media/def456.webp",
					Hash:     "def456hash00000000",
					Variants: map[string]media.MediaVariant{
						"_thumb": {FilePath: "/uploads/media/def456_thumb.webp", URL: "/uploads/media/def456_thumb.webp", Width: 64, Height: 36},
					},
				}
				repo.On("FindAllByCursor", mock.Anything, 0, 100, 0).Return([]*media.Media{m}, nil).Once()
			},
			wantProcessed:   1,
			wantSkipped:     0,
			wantErr:         false,
			wantBytesShrink: false,
		},
		{
			name: "skipped - missing file counts as skipped and run continues",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				raw := losslessWebPBytes(t, 320, 200)
				storage.saved["good.webp"] = raw
				missing := &media.Media{
					ID:       9,
					Filename: "missing.webp",
					FilePath: "/uploads/media/missing.webp",
					Hash:     "missinghash0000000",
				}
				good := &media.Media{
					ID:       8,
					Filename: "good.webp",
					FilePath: "/uploads/media/good.webp",
					Hash:     "goodhash0000000000",
				}
				repo.On("FindAllByCursor", mock.Anything, 0, 100, 0).Return([]*media.Media{missing, good}, nil).Once()
			},
			wantProcessed:   1,
			wantSkipped:     1,
			wantErr:         false,
			wantBytesShrink: false,
		},
		{
			name: "skipped - corrupt bytes fail decode",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				storage.saved["bad.webp"] = []byte("not a webp file at all")
				m := &media.Media{
					ID:       3,
					Filename: "bad.webp",
					FilePath: "/uploads/media/bad.webp",
					Hash:     "badhash00000000000",
				}
				repo.On("FindAllByCursor", mock.Anything, 0, 100, 0).Return([]*media.Media{m}, nil).Once()
			},
			wantProcessed:   0,
			wantSkipped:     1,
			wantErr:         false,
			wantBytesShrink: false,
		},
		{
			name: "skipped - save failure counts as skipped",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				raw := losslessWebPBytes(t, 320, 200)
				storage.saved["savefail.webp"] = raw
				storage.saveErr = errors.New("disk full")
				m := &media.Media{
					ID:       4,
					Filename: "savefail.webp",
					FilePath: "/uploads/media/savefail.webp",
					Hash:     "savefailhash000000",
				}
				repo.On("FindAllByCursor", mock.Anything, 0, 100, 0).Return([]*media.Media{m}, nil).Once()
			},
			wantProcessed:   0,
			wantSkipped:     1,
			wantErr:         false,
			wantBytesShrink: false,
		},
		{
			name: "empty library - no records",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				repo.On("FindAllByCursor", mock.Anything, 0, 100, 0).Return([]*media.Media{}, nil).Once()
			},
			wantProcessed:   0,
			wantSkipped:     0,
			wantErr:         false,
			wantBytesShrink: false,
		},
		{
			name: "error - repository list failure",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				repo.On("FindAllByCursor", mock.Anything, 0, 100, 0).Return([]*media.Media(nil), errors.New("db down")).Once()
			},
			wantProcessed:   0,
			wantSkipped:     0,
			wantErr:         true,
			wantBytesShrink: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mediamocks.NewMockRepository(t)
			storage := newStubStorage()
			tt.setup(repo, storage)
			// Every processed record refreshes its stored file size; allow
			// it wherever the table drives a record to completion (the
			// dedicated test below pins the exact calls).
			repo.On("UpdateFileSize", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

			svc := media.NewService(repo, storage, nil)
			summary, err := svc.ReencodeLibrary(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantProcessed, summary.Processed)
			assert.Equal(t, tt.wantSkipped, summary.Skipped)
			if tt.wantProcessed > 0 {
				assert.Greater(t, summary.BytesBefore, int64(0))
				assert.Greater(t, summary.BytesAfter, int64(0))
			}
			if tt.wantBytesShrink {
				assert.LessOrEqual(t, summary.BytesAfter, summary.BytesBefore)
			}
		})
	}
}

func TestService_ReencodeLibrary_CancelledContext(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*mediamocks.MockRepository, *stubStorage)
		wantErr bool
	}{
		{
			name: "error - cancelled context aborts before listing",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mediamocks.NewMockRepository(t)
			storage := newStubStorage()
			tt.setup(repo, storage)
			// Every processed record refreshes its stored file size; allow
			// it wherever the table drives a record to completion (the
			// dedicated test below pins the exact calls).
			repo.On("UpdateFileSize", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

			svc := media.NewService(repo, storage, nil)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, err := svc.ReencodeLibrary(ctx)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestService_ReencodeLibrary_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(*mediamocks.MockRepository, *stubStorage)
		wantProcessed int
		wantSkipped   int
		wantErr       bool
	}{
		{
			name: "skipped - nil record counts as skipped",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				repo.On("FindAllByCursor", mock.Anything, 0, 100, 0).Return([]*media.Media{nil}, nil).Once()
			},
			wantProcessed: 0,
			wantSkipped:   1,
			wantErr:       false,
		},
		{
			name: "success - empty filename falls back to base of file path",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				raw := losslessWebPBytes(t, 64, 64)
				storage.saved["fallback.webp"] = raw
				m := &media.Media{
					ID:       11,
					Filename: "",
					FilePath: "/uploads/media/fallback.webp",
					Hash:     "fallbackhash00000",
				}
				repo.On("FindAllByCursor", mock.Anything, 0, 100, 0).Return([]*media.Media{m}, nil).Once()
			},
			wantProcessed: 1,
			wantSkipped:   0,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mediamocks.NewMockRepository(t)
			storage := newStubStorage()
			tt.setup(repo, storage)
			// Every processed record refreshes its stored file size; allow
			// it wherever the table drives a record to completion (the
			// dedicated test below pins the exact calls).
			repo.On("UpdateFileSize", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

			svc := media.NewService(repo, storage, nil)
			summary, err := svc.ReencodeLibrary(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantProcessed, summary.Processed)
			assert.Equal(t, tt.wantSkipped, summary.Skipped)
		})
	}
}

func TestService_ReencodeLibrary_UpdatesFileSize(t *testing.T) {
	raw := losslessWebPBytes(t, 320, 200)

	t.Run("success - stored size follows the rewritten bytes", func(t *testing.T) {
		repo := mediamocks.NewMockRepository(t)
		storage := newStubStorage()
		storage.saved["sized.webp"] = raw
		m := &media.Media{
			ID:       7,
			Filename: "sized.webp",
			FilePath: "/uploads/media/sized.webp",
			Hash:     "sizedhash0000000000",
		}
		repo.On("FindAllByCursor", mock.Anything, 0, 100, 0).Return([]*media.Media{m}, nil).Once()
		var gotSize int64 = -1
		repo.On("UpdateFileSize", mock.Anything, 7, mock.Anything).
			Run(func(args mock.Arguments) { gotSize = args.Get(2).(int64) }).
			Return(nil).
			Once()

		svc := media.NewService(repo, storage, nil)
		summary, err := svc.ReencodeLibrary(context.Background())

		require.NoError(t, err)
		assert.Equal(t, 1, summary.Processed)
		assert.Equal(t, 0, summary.Skipped)
		require.GreaterOrEqual(t, gotSize, int64(0), "size update must fire with the rewritten byte count")
		assert.Equal(t, int64(len(storage.saved["sized.webp"])), gotSize, "stored file size follows the rewritten bytes")
	})

	t.Run("error - file size update failure skips the record", func(t *testing.T) {
		repo := mediamocks.NewMockRepository(t)
		storage := newStubStorage()
		storage.saved["nosize.webp"] = raw
		m := &media.Media{
			ID:       8,
			Filename: "nosize.webp",
			FilePath: "/uploads/media/nosize.webp",
			Hash:     "nosizehash00000000",
		}
		repo.On("FindAllByCursor", mock.Anything, 0, 100, 0).Return([]*media.Media{m}, nil).Once()
		repo.On("UpdateFileSize", mock.Anything, 8, mock.Anything).Return(errors.New("db down")).Once()

		svc := media.NewService(repo, storage, nil)
		summary, err := svc.ReencodeLibrary(context.Background())

		require.NoError(t, err)
		assert.Equal(t, 0, summary.Processed)
		assert.Equal(t, 1, summary.Skipped)
	})
}

func TestService_ReencodeLibrary_Paging(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(*mediamocks.MockRepository, *stubStorage)
		wantProcessed int
		wantSkipped   int
		wantErr       bool
	}{
		{
			name: "multi-page - cursor advances past a full page",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				raw := losslessWebPBytes(t, 32, 32)
				storage.saved["page.webp"] = raw
				var page1 []*media.Media
				for id := 200; id >= 101; id-- {
					page1 = append(page1, &media.Media{
						ID:       id,
						Filename: "page.webp",
						FilePath: "/uploads/media/page.webp",
						Hash:     "paginghash0000000000",
					})
				}
				page2 := []*media.Media{{
					ID:       50,
					Filename: "page.webp",
					FilePath: "/uploads/media/page.webp",
					Hash:     "paginghash0000000000",
				}}
				repo.On("FindAllByCursor", mock.Anything, 0, 100, 0).Return(page1, nil).Once()
				repo.On("FindAllByCursor", mock.Anything, 0, 100, 101).Return(page2, nil).Once()
				repo.On("UpdateFileSize", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
			},
			wantProcessed: 101,
			wantSkipped:   0,
			wantErr:       false,
		},
		{
			name: "trailing nil - cursor advances past trailing nils",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				raw := losslessWebPBytes(t, 32, 32)
				storage.saved["trail.webp"] = raw
				var page1 []*media.Media
				for id := 200; id >= 102; id-- {
					page1 = append(page1, &media.Media{
						ID:       id,
						Filename: "trail.webp",
						FilePath: "/uploads/media/trail.webp",
						Hash:     "trailhash0000000000",
					})
				}
				page1 = append(page1, nil)
				repo.On("FindAllByCursor", mock.Anything, 0, 100, 0).Return(page1, nil).Once()
				repo.On("FindAllByCursor", mock.Anything, 0, 100, 102).Return([]*media.Media{}, nil).Once()
				repo.On("UpdateFileSize", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
			},
			wantProcessed: 99,
			wantSkipped:   1,
			wantErr:       false,
		},
		{
			name: "all-nil batch - stops instead of looping",
			setup: func(repo *mediamocks.MockRepository, storage *stubStorage) {
				repo.On("FindAllByCursor", mock.Anything, 0, 100, 0).Return([]*media.Media{nil, nil}, nil).Once()
			},
			wantProcessed: 0,
			wantSkipped:   2,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mediamocks.NewMockRepository(t)
			storage := newStubStorage()
			repo.On("UpdateFileSize", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
			tt.setup(repo, storage)

			svc := media.NewService(repo, storage, nil)
			summary, err := svc.ReencodeLibrary(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantProcessed, summary.Processed)
			assert.Equal(t, tt.wantSkipped, summary.Skipped)
		})
	}
}
