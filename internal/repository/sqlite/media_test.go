package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
	appsqlite "github.com/aristorinjuang/lesstruct/internal/repository/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// closeMediaTestDB is a helper function that explicitly ignores the error from Close()
// to satisfy errcheck linter for defer statements.
func closeMediaTestDB(db *sql.DB) {
	_ = db.Close()
}

func setupMediaTestDB(t *testing.T) *sql.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := "file:" + filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err, "Failed to open test database")

	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL,
			name TEXT
		);

		CREATE TABLE content_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			slug TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT,
			status TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			post_type TEXT DEFAULT 'post',
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			language TEXT NOT NULL DEFAULT 'en',
			translation_group_id INTEGER DEFAULT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id),
			UNIQUE(slug, language)
		);

		CREATE TABLE media_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			filename TEXT NOT NULL,
			original_filename TEXT NOT NULL,
			mime_type TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			width INTEGER,
			height INTEGER,
			alt_text TEXT,
			is_webp BOOLEAN DEFAULT TRUE,
			file_path TEXT NOT NULL,
			url TEXT NOT NULL,
			hash TEXT UNIQUE NOT NULL,
			variants TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);

		CREATE INDEX idx_media_files_user_id ON media_files(user_id);
		CREATE INDEX idx_media_files_hash ON media_files(hash);

		INSERT INTO users (username, password_hash, role, name) VALUES ('testuser', 'hash', 'user', 'Test User');
		INSERT INTO content_items (user_id, title, slug, content, status, post_type) VALUES (1, 'Test Content', 'test-content', 'Test', 'draft', 'post');
	`)
	require.NoError(t, err, "Failed to create test tables")

	return db
}

func TestMediaRepository_Create(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	repo := appsqlite.NewMediaRepository(db)

	media := &mediadomain.Media{
		UserID:           1,
		Filename:         "test.webp",
		OriginalFilename: "test.jpg",
		MimeType:         mediadomain.MimeTypeWebP,
		FileSize:         1000,
		Width:            100,
		Height:           100,
		AltText:          "A test image",
		IsWebP:           true,
		FilePath:         "/uploads/media/test.webp",
		URL:              "http://localhost:8080/uploads/media/test.webp",
		Hash:             "test-hash-123",
	}

	ctx := context.Background()
	err := repo.Create(ctx, media)
	require.NoError(t, err, "Create() failed")

	assert.NotZero(t, media.ID, "Create() did not set ID")
	assert.NotEmpty(t, media.CreatedAt, "Create() did not set CreatedAt")
	assert.NotEmpty(t, media.UpdatedAt, "Create() did not set UpdatedAt")
}

func TestMediaRepository_FindByID(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	repo := appsqlite.NewMediaRepository(db)

	media := &mediadomain.Media{
		UserID:           1,
		Filename:         "test.webp",
		OriginalFilename: "test.jpg",
		MimeType:         mediadomain.MimeTypeWebP,
		FileSize:         1000,
		Width:            100,
		Height:           100,
		AltText:          "A test image",
		IsWebP:           true,
		FilePath:         "/uploads/media/test.webp",
		URL:              "http://localhost:8080/uploads/media/test.webp",
		Hash:             "test-hash-123",
	}

	ctx := context.Background()
	_ = repo.Create(ctx, media)

	found, err := repo.FindByID(ctx, media.ID)
	require.NoError(t, err, "FindByID() failed")

	assert.Equal(t, media.ID, found.ID, "FindByID() ID mismatch")
	assert.Equal(t, media.OriginalFilename, found.OriginalFilename, "FindByID() OriginalFilename mismatch")
	assert.Equal(t, "Test User", found.UploadedBy, "FindByID() UploadedBy mismatch")
}

func TestMediaRepository_FindByID_NotFound(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	repo := appsqlite.NewMediaRepository(db)

	ctx := context.Background()
	_, err := repo.FindByID(ctx, 999)

	assert.ErrorIs(t, err, mediadomain.ErrMediaNotFound, "FindByID() expected ErrMediaNotFound")
}

func TestMediaRepository_FindByHash(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	repo := appsqlite.NewMediaRepository(db)

	media := &mediadomain.Media{
		UserID:           1,
		Filename:         "test.webp",
		OriginalFilename: "test.jpg",
		MimeType:         mediadomain.MimeTypeWebP,
		FileSize:         1000,
		Width:            100,
		Height:           100,
		AltText:          "A test image",
		IsWebP:           true,
		FilePath:         "/uploads/media/test.webp",
		URL:              "http://localhost:8080/uploads/media/test.webp",
		Hash:             "test-hash-456",
	}

	ctx := context.Background()
	_ = repo.Create(ctx, media)

	found, err := repo.FindByHash(ctx, "test-hash-456")
	require.NoError(t, err, "FindByHash() failed")

	assert.Equal(t, media.Hash, found.Hash, "FindByHash() Hash mismatch")
	assert.Equal(t, "Test User", found.UploadedBy, "FindByHash() UploadedBy mismatch")
}

func TestMediaRepository_FindByHash_NotFound(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	repo := appsqlite.NewMediaRepository(db)

	ctx := context.Background()
	_, err := repo.FindByHash(ctx, "nonexistent")

	assert.ErrorIs(t, err, mediadomain.ErrMediaNotFound, "FindByHash() expected ErrMediaNotFound")
}

func TestMediaRepository_FindAll(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	repo := appsqlite.NewMediaRepository(db)

	media1 := &mediadomain.Media{
		UserID:           1,
		Filename:         "test1.webp",
		OriginalFilename: "test1.jpg",
		MimeType:         mediadomain.MimeTypeWebP,
		FileSize:         1000,
		Width:            100,
		Height:           100,
		AltText:          "Test 1",
		IsWebP:           true,
		FilePath:         "/uploads/media/test1.webp",
		URL:              "http://localhost:8080/uploads/media/test1.webp",
		Hash:             "hash-1",
	}

	media2 := &mediadomain.Media{
		UserID:           1,
		Filename:         "test2.webp",
		OriginalFilename: "test2.jpg",
		MimeType:         mediadomain.MimeTypeWebP,
		FileSize:         1000,
		Width:            100,
		Height:           100,
		AltText:          "Test 2",
		IsWebP:           true,
		FilePath:         "/uploads/media/test2.webp",
		URL:              "http://localhost:8080/uploads/media/test2.webp",
		Hash:             "hash-2",
	}

	ctx := context.Background()
	_ = repo.Create(ctx, media1)
	_ = repo.Create(ctx, media2)

	found, err := repo.FindAll(ctx, 10, 0)
	require.NoError(t, err, "FindAll() failed")

	assert.Len(t, found, 2, "FindAll() expected 2 results")
	for _, m := range found {
		assert.Equal(t, "Test User", m.UploadedBy, "FindAll() should populate UploadedBy")
	}
}

func TestMediaRepository_ListByCursor(t *testing.T) {
	// seedMedia inserts num media rows for userID (IDs auto-assigned sequentially) plus
	// one row for a different user (user 2) so scoping is verifiable. Returns nothing —
	// the inserted IDs are deterministic (1..num for userID, num+1 for user 2).
	seedMedia := func(t *testing.T, repo *appsqlite.MediaRepository, userID, num int) {
		t.Helper()
		ctx := context.Background()
		for i := 1; i <= num; i++ {
			m := &mediadomain.Media{
				UserID:           userID,
				Filename:         "test" + strconv.Itoa(i) + ".webp",
				OriginalFilename: "test" + strconv.Itoa(i) + ".jpg",
				MimeType:         mediadomain.MimeTypeWebP,
				FileSize:         1000,
				Width:            100,
				Height:           100,
				AltText:          "Test " + strconv.Itoa(i),
				IsWebP:           true,
				FilePath:         "/uploads/media/test" + strconv.Itoa(i) + ".webp",
				URL:              "http://localhost:8080/uploads/media/test" + strconv.Itoa(i) + ".webp",
				Hash:             "hash-" + strconv.Itoa(i),
			}
			require.NoError(t, repo.Create(ctx, m), "seed Create failed")
		}
		// A different user's media must be excluded by the user_id scoping.
		other := &mediadomain.Media{
			UserID:           userID + 1,
			Filename:         "other.webp",
			OriginalFilename: "other.jpg",
			MimeType:         mediadomain.MimeTypeWebP,
			FileSize:         1000,
			Width:            100,
			Height:           100,
			AltText:          "Other",
			IsWebP:           true,
			FilePath:         "/uploads/media/other.webp",
			URL:              "http://localhost:8080/uploads/media/other.webp",
			Hash:             "hash-other",
		}
		require.NoError(t, repo.Create(ctx, other), "seed other Create failed")
	}

	tests := []struct {
		name     string
		userID   int
		limit    int
		beforeID int
		seedN    int
		wantIDs  []int
	}{
		{
			name:     "first page returns newest-first scoped to caller",
			userID:   1,
			limit:    10,
			beforeID: 0,
			seedN:    3,
			wantIDs:  []int{3, 2, 1},
		},
		{
			name:     "beforeID filters to older rows",
			userID:   1,
			limit:    10,
			beforeID: 2,
			seedN:    3,
			wantIDs:  []int{1},
		},
		{
			name:     "limit is honored",
			userID:   1,
			limit:    2,
			beforeID: 0,
			seedN:    3,
			wantIDs:  []int{3, 2},
		},
		{
			name:     "empty set when beforeID below minimum id",
			userID:   1,
			limit:    10,
			beforeID: 1,
			seedN:    3,
			wantIDs:  []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupMediaTestDB(t)
			defer closeMediaTestDB(db)
			repo := appsqlite.NewMediaRepository(db)
			seedMedia(t, repo, tt.userID, tt.seedN)

			got, err := repo.ListByCursor(context.Background(), tt.userID, tt.limit, tt.beforeID)
			require.NoError(t, err, "ListByCursor() unexpected error")

			gotIDs := make([]int, 0, len(got))
			for _, m := range got {
				gotIDs = append(gotIDs, m.ID)
			}
			assert.Equal(t, tt.wantIDs, gotIDs, "ListByCursor() result order/IDs (must exclude other user's media)")
		})
	}
}

func TestMediaRepository_DeleteByOwner(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	repo := appsqlite.NewMediaRepository(db)

	media := &mediadomain.Media{
		UserID:           1,
		Filename:         "test.webp",
		OriginalFilename: "test.jpg",
		MimeType:         mediadomain.MimeTypeWebP,
		FileSize:         1000,
		Width:            100,
		Height:           100,
		AltText:          "Test",
		IsWebP:           true,
		FilePath:         "/uploads/media/test.webp",
		URL:              "http://localhost:8080/uploads/media/test.webp",
		Hash:             "hash-delete",
	}

	ctx := context.Background()
	_ = repo.Create(ctx, media)

	err := repo.DeleteByOwner(ctx, media.ID, 1)
	require.NoError(t, err, "DeleteByOwner() failed")

	_, err = repo.FindByID(ctx, media.ID)
	assert.ErrorIs(t, err, mediadomain.ErrMediaNotFound, "DeleteByOwner() media still exists")
}

func TestMediaRepository_DeleteByOwner_Unauthorized(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	repo := appsqlite.NewMediaRepository(db)

	media := &mediadomain.Media{
		UserID:           1,
		Filename:         "test.webp",
		OriginalFilename: "test.jpg",
		MimeType:         mediadomain.MimeTypeWebP,
		FileSize:         1000,
		Width:            100,
		Height:           100,
		AltText:          "Test",
		IsWebP:           true,
		FilePath:         "/uploads/media/test.webp",
		URL:              "http://localhost:8080/uploads/media/test.webp",
		Hash:             "hash-unauth",
	}

	ctx := context.Background()
	_ = repo.Create(ctx, media)

	err := repo.DeleteByOwner(ctx, media.ID, 999)
	assert.Error(t, err, "DeleteByOwner() should fail with wrong userID")
}

func TestMediaRepository_DeleteByID(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	repo := appsqlite.NewMediaRepository(db)

	media := &mediadomain.Media{
		UserID:           1,
		Filename:         "test.webp",
		OriginalFilename: "test.jpg",
		MimeType:         mediadomain.MimeTypeWebP,
		FileSize:         1000,
		Width:            100,
		Height:           100,
		AltText:          "Test",
		IsWebP:           true,
		FilePath:         "/uploads/media/test.webp",
		URL:              "http://localhost:8080/uploads/media/test.webp",
		Hash:             "hash-admin-delete",
	}

	ctx := context.Background()
	_ = repo.Create(ctx, media)

	err := repo.DeleteByID(ctx, media.ID)
	require.NoError(t, err, "DeleteByID() failed")

	_, err = repo.FindByID(ctx, media.ID)
	assert.ErrorIs(t, err, mediadomain.ErrMediaNotFound, "DeleteByID() media still exists")
}

func TestMediaRepository_DeleteByID_NotFound(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	repo := appsqlite.NewMediaRepository(db)

	ctx := context.Background()
	err := repo.DeleteByID(ctx, 999)
	assert.ErrorIs(t, err, mediadomain.ErrMediaNotFound, "DeleteByID() expected ErrMediaNotFound")
}

func TestMediaRepository_Create_DuplicateHash(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	repo := appsqlite.NewMediaRepository(db)

	media1 := &mediadomain.Media{
		UserID:           1,
		Filename:         "test1.webp",
		OriginalFilename: "test1.jpg",
		MimeType:         mediadomain.MimeTypeWebP,
		FileSize:         1000,
		Width:            100,
		Height:           100,
		AltText:          "Test 1",
		IsWebP:           true,
		FilePath:         "/uploads/media/test1.webp",
		URL:              "http://localhost:8080/uploads/media/test1.webp",
		Hash:             "duplicate-hash",
	}

	media2 := &mediadomain.Media{
		UserID:           1,
		Filename:         "test2.webp",
		OriginalFilename: "test2.jpg",
		MimeType:         mediadomain.MimeTypeWebP,
		FileSize:         1000,
		Width:            100,
		Height:           100,
		AltText:          "Test 2",
		IsWebP:           true,
		FilePath:         "/uploads/media/test2.webp",
		URL:              "http://localhost:8080/uploads/media/test2.webp",
		Hash:             "duplicate-hash",
	}

	ctx := context.Background()
	_ = repo.Create(ctx, media1)

	err := repo.Create(ctx, media2)
	assert.ErrorIs(t, err, mediadomain.ErrDuplicateMedia, "Create() expected ErrDuplicateMedia")
}

func createMediaForSearch(t *testing.T, db *sql.DB, userID int, filename, hash string) *mediadomain.Media {
	t.Helper()

	repo := appsqlite.NewMediaRepository(db)
	m := &mediadomain.Media{
		UserID:           userID,
		Filename:         hash + ".webp",
		OriginalFilename: filename,
		MimeType:         mediadomain.MimeTypeWebP,
		FileSize:         1000,
		Width:            100,
		Height:           100,
		AltText:          "Test",
		IsWebP:           true,
		FilePath:         "/uploads/media/" + hash + ".webp",
		URL:              "http://localhost:8080/uploads/media/" + hash + ".webp",
		Hash:             hash,
	}

	ctx := context.Background()
	_ = repo.Create(ctx, m)
	return m
}

func TestMediaRepository_FindAllByFilename(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	createMediaForSearch(t, db, 1, "sunset_beach.jpg", "hash-search-1")
	createMediaForSearch(t, db, 1, "mountain_view.jpg", "hash-search-2")
	createMediaForSearch(t, db, 1, "sunset_city.jpg", "hash-search-3")

	repo := appsqlite.NewMediaRepository(db)
	ctx := context.Background()

	found, err := repo.FindAllByFilename(ctx, "sunset", 100, 0)
	require.NoError(t, err)
	assert.Len(t, found, 2, "FindAllByFilename() expected 2 results matching 'sunset'")
}

func TestMediaRepository_FindAllByFilename_CaseInsensitive(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	createMediaForSearch(t, db, 1, "Sunset_Beach.jpg", "hash-ci-1")

	repo := appsqlite.NewMediaRepository(db)
	ctx := context.Background()

	found, err := repo.FindAllByFilename(ctx, "SUNSET", 100, 0)
	require.NoError(t, err)
	assert.Len(t, found, 1, "FindAllByFilename() should be case-insensitive")
}

func TestMediaRepository_FindAllByFilename_NoMatch(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	createMediaForSearch(t, db, 1, "sunset.jpg", "hash-nm-1")

	repo := appsqlite.NewMediaRepository(db)
	ctx := context.Background()

	found, err := repo.FindAllByFilename(ctx, "mountain", 100, 0)
	require.NoError(t, err)
	assert.Len(t, found, 0, "FindAllByFilename() expected 0 results for non-matching query")
}

func TestMediaRepository_FindAllByFilename_AcrossUsers(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	_, err := db.Exec(`INSERT INTO users (username, password_hash, role, name) VALUES ('user2', 'hash2', 'user', 'User Two')`)
	require.NoError(t, err)

	createMediaForSearch(t, db, 1, "sunset.jpg", "hash-us-1")
	createMediaForSearch(t, db, 2, "sunset_other.jpg", "hash-us-2")

	repo := appsqlite.NewMediaRepository(db)
	ctx := context.Background()

	found, err := repo.FindAllByFilename(ctx, "sunset", 100, 0)
	require.NoError(t, err)
	assert.Len(t, found, 2, "FindAllByFilename() should return results across all users")
}

func TestMediaRepository_FindAllByDateRange(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	createMediaForSearch(t, db, 1, "old_photo.jpg", "hash-dr-old")

	_, err := db.Exec(`UPDATE media_files SET created_at = datetime('now', '-30 days') WHERE hash = 'hash-dr-old'`)
	require.NoError(t, err)

	createMediaForSearch(t, db, 1, "recent_photo.jpg", "hash-dr-new")

	repo := appsqlite.NewMediaRepository(db)
	ctx := context.Background()

	since := time.Now().AddDate(0, 0, -7)
	found, err := repo.FindAllByDateRange(ctx, since, 100, 0)
	require.NoError(t, err)
	assert.Len(t, found, 1, "FindAllByDateRange() expected 1 recent result")
	assert.Equal(t, "recent_photo.jpg", found[0].OriginalFilename)
}

func TestMediaRepository_FindAllByDateRange_NoResults(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	createMediaForSearch(t, db, 1, "old_photo.jpg", "hash-dr-nr")

	_, err := db.Exec(`UPDATE media_files SET created_at = datetime('now', '-60 days') WHERE hash = 'hash-dr-nr'`)
	require.NoError(t, err)

	repo := appsqlite.NewMediaRepository(db)
	ctx := context.Background()

	since := time.Now().AddDate(0, 0, -7)
	found, err := repo.FindAllByDateRange(ctx, since, 100, 0)
	require.NoError(t, err)
	assert.Len(t, found, 0, "FindAllByDateRange() expected 0 results for old media")
}

func TestMediaRepository_FindAllByFilenameAndDateRange(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	createMediaForSearch(t, db, 1, "sunset_old.jpg", "hash-comb-1")

	_, err := db.Exec(`UPDATE media_files SET created_at = datetime('now', '-30 days') WHERE hash = 'hash-comb-1'`)
	require.NoError(t, err)

	createMediaForSearch(t, db, 1, "sunset_recent.jpg", "hash-comb-2")
	createMediaForSearch(t, db, 1, "mountain_recent.jpg", "hash-comb-3")

	repo := appsqlite.NewMediaRepository(db)
	ctx := context.Background()

	since := time.Now().AddDate(0, 0, -7)
	found, err := repo.FindAllByFilenameAndDateRange(ctx, "sunset", since, 100, 0)
	require.NoError(t, err)
	assert.Len(t, found, 1, "FindAllByFilenameAndDateRange() expected 1 result matching both filters")
	assert.Equal(t, "sunset_recent.jpg", found[0].OriginalFilename)
}

func TestMediaRepository_Create_WithVariants(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	repo := appsqlite.NewMediaRepository(db)

	media := &mediadomain.Media{
		UserID:           1,
		Filename:         "test.webp",
		OriginalFilename: "test.jpg",
		MimeType:         mediadomain.MimeTypeWebP,
		FileSize:         1000,
		Width:            100,
		Height:           100,
		AltText:          "Test",
		IsWebP:           true,
		FilePath:         "/uploads/media/test.webp",
		URL:              "http://localhost:8080/uploads/media/test.webp",
		Hash:             "hash-variants",
		Variants: map[string]mediadomain.MediaVariant{
			"_thumb": {
				FilePath: "/uploads/media/test_thumb.webp",
				URL:      "http://localhost:8080/uploads/media/test_thumb.webp",
				Width:    50,
				Height:   50,
			},
		},
	}

	ctx := context.Background()
	err := repo.Create(ctx, media)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, media.ID)
	require.NoError(t, err)
	require.NotNil(t, found.Variants)
	assert.Len(t, found.Variants, 1)
	assert.Equal(t, 50, found.Variants["_thumb"].Width)
	assert.Equal(t, 50, found.Variants["_thumb"].Height)
	assert.Equal(t, "/uploads/media/test_thumb.webp", found.Variants["_thumb"].FilePath)
	assert.Equal(t, "http://localhost:8080/uploads/media/test_thumb.webp", found.Variants["_thumb"].URL)
}

func TestMediaRepository_FindByID_NullVariants_ReturnsEmptyMap(t *testing.T) {
	db := setupMediaTestDB(t)
	defer closeMediaTestDB(db)

	repo := appsqlite.NewMediaRepository(db)

	media := &mediadomain.Media{
		UserID:           1,
		Filename:         "legacy.webp",
		OriginalFilename: "legacy.jpg",
		MimeType:         mediadomain.MimeTypeWebP,
		FileSize:         1000,
		Width:            100,
		Height:           100,
		AltText:          "Legacy",
		IsWebP:           true,
		FilePath:         "/uploads/media/legacy.webp",
		URL:              "http://localhost:8080/uploads/media/legacy.webp",
		Hash:             "hash-legacy-null",
	}

	ctx := context.Background()
	err := repo.Create(ctx, media)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, media.ID)
	require.NoError(t, err)
	require.NotNil(t, found.Variants, "Variants should be non-nil empty map, not nil")
	assert.Empty(t, found.Variants, "Variants should be empty map for NULL column")
}
