package sqlite_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/repository/sqlite"
	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupContentTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err, "failed to open test database")

	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			email TEXT,
			name TEXT,
			role TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS content_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			slug TEXT NOT NULL,
			content TEXT,
			tags TEXT,
			status TEXT DEFAULT 'draft',
			post_type TEXT DEFAULT 'post',
			meta_description TEXT,
			og_title TEXT,
			og_description TEXT,
				allow_comments INTEGER DEFAULT 1,
				custom_fields TEXT,
			updated_by INTEGER REFERENCES users(id),
			language TEXT NOT NULL DEFAULT 'en',
			translation_group_id INTEGER DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(slug, language)
		);

		CREATE INDEX IF NOT EXISTS idx_content_items_slug ON content_items(slug);
		CREATE INDEX IF NOT EXISTS idx_content_items_user_id ON content_items(user_id);
		CREATE INDEX IF NOT EXISTS idx_content_items_status ON content_items(status);
		CREATE INDEX IF NOT EXISTS idx_content_items_post_type ON content_items(post_type);
		CREATE INDEX IF NOT EXISTS idx_content_items_language ON content_items(language);
		CREATE INDEX IF NOT EXISTS idx_content_items_translation_group ON content_items(translation_group_id);
	`)
	require.NoError(t, err, "failed to create test tables")

	return db
}

func teardownContentTestDB(t *testing.T, db *sql.DB) {
	assert.NoError(t, db.Close(), "failed to close test database")
}

func TestContentRepository_Create(t *testing.T) {
	tests := []struct {
		name       string
		content    *content.Content
		setupDB    func(*sql.DB) error
		wantErr    error
		validateID bool
	}{
		{
			name: "successful content creation",
			content: &content.Content{
				UserID:  1,
				Title:   "Test Content",
				Slug:    "test-content",
				Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Test content body"}]}]}`,
				Tags:    []string{"test", "example"},
				Status:  content.StatusDraft,
			},
			setupDB:    func(db *sql.DB) error { return nil },
			wantErr:    nil,
			validateID: true,
		},
		{
			name: "content creation with empty tags",
			content: &content.Content{
				UserID:  1,
				Title:   "Test Content 2",
				Slug:    "test-content-2",
				Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Test content body"}]}]}`,
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupDB:    func(db *sql.DB) error { return nil },
			wantErr:    nil,
			validateID: true,
		},
		{
			name: "content creation with published status",
			content: &content.Content{
				UserID:  1,
				Title:   "Published Content",
				Slug:    "published-content",
				Content: "Published content body",
				Tags:    []string{"news"},
				Status:  content.StatusPublished,
			},
			setupDB:    func(db *sql.DB) error { return nil },
			wantErr:    nil,
			validateID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContentTestDB(t)
			defer teardownContentTestDB(t, db)

			require.NoError(t, tt.setupDB(db), "failed to setup db")

			repo := sqlite.NewContentRepository(db)
			err := repo.Create(context.Background(), tt.content)

			if tt.wantErr != nil {
				assert.Error(t, err, "ContentRepository.Create() expected error")
				if err != nil {
					assert.Equal(t, tt.wantErr.Error(), err.Error(), "ContentRepository.Create() error mismatch")
				}
				return
			}

			require.NoError(t, err, "ContentRepository.Create() unexpected error")

			if tt.validateID {
				assert.NotZero(t, tt.content.ID, "ContentRepository.Create() expected ID to be set")
			}

			assert.NotEmpty(t, tt.content.CreatedAt, "ContentRepository.Create() expected CreatedAt to be set")
			assert.NotEmpty(t, tt.content.UpdatedAt, "ContentRepository.Create() expected UpdatedAt to be set")
		})
	}
}

func TestContentRepository_GetBySlug(t *testing.T) {
	tests := []struct {
		name       string
		slug       string
		setupDB    func(*sql.DB) error
		wantErr    error
		validateFn func(*testing.T, *content.Content)
	}{
		{
			name: "successful retrieval",
			slug: "test-content",
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status)
					VALUES (1, 'Test Content', 'test-content', 'Test body', '["test"]', 'draft')
				`)
				return err
			},
			wantErr: nil,
			validateFn: func(t *testing.T, content *content.Content) {
				assert.Equal(t, "Test Content", content.Title, "expected title 'Test Content'")
				assert.Equal(t, "test-content", content.Slug, "expected slug 'test-content'")
				assert.Len(t, content.Tags, 1, "expected 1 tag")
				assert.Equal(t, "test", content.Tags[0], "expected tag 'test'")
			},
		},
		{
			name: "content not found",
			slug: "non-existent",
			setupDB: func(db *sql.DB) error {
				return nil
			},
			wantErr:    content.ErrContentNotFound,
			validateFn: nil,
		},
		{
			name: "retrieval with empty tags",
			slug: "no-tags",
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status)
					VALUES (1, 'No Tags', 'no-tags', 'Test body', '[]', 'draft')
				`)
				return err
			},
			wantErr: nil,
			validateFn: func(t *testing.T, content *content.Content) {
				if content.Tags == nil {
					content.Tags = []string{}
				}
				assert.Empty(t, content.Tags, "expected no tags")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContentTestDB(t)
			defer teardownContentTestDB(t, db)

			require.NoError(t, tt.setupDB(db), "failed to setup db")

			repo := sqlite.NewContentRepository(db)
			content, err := repo.GetBySlug(context.Background(), tt.slug, "en")

			if tt.wantErr != nil {
				assert.Error(t, err, "ContentRepository.GetBySlug() expected error")
				assert.ErrorIs(t, err, tt.wantErr, "ContentRepository.GetBySlug() error mismatch")
				return
			}

			require.NoError(t, err, "ContentRepository.GetBySlug() unexpected error")
			require.NotNil(t, content, "ContentRepository.GetBySlug() expected content")

			if tt.validateFn != nil {
				tt.validateFn(t, content)
			}
		})
	}
}

func TestContentRepository_GetByUser(t *testing.T) {
	tests := []struct {
		name       string
		userID     int
		limit      int
		offset     int
		setupDB    func(*sql.DB) error
		wantErr    error
		validateFn func(*testing.T, []*content.Content)
	}{
		{
			name:   "successful retrieval with results",
			userID: 1,
			limit:  10,
			offset: 0,
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status) VALUES
					(1, 'Content 1', 'content-1', 'Body 1', '["tag1"]', 'draft'),
					(1, 'Content 2', 'content-2', 'Body 2', '["tag2"]', 'draft'),
					(2, 'Content 3', 'content-3', 'Body 3', '["tag3"]', 'draft')
				`)
				return err
			},
			wantErr: nil,
			validateFn: func(t *testing.T, contents []*content.Content) {
				assert.Len(t, contents, 2, "expected 2 contents")
			},
		},
		{
			name:   "retrieval with no results",
			userID: 999,
			limit:  10,
			offset: 0,
			setupDB: func(db *sql.DB) error {
				return nil
			},
			wantErr: nil,
			validateFn: func(t *testing.T, contents []*content.Content) {
				assert.Empty(t, contents, "expected 0 contents")
			},
		},
		{
			name:   "retrieval with limit",
			userID: 1,
			limit:  1,
			offset: 0,
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status) VALUES
					(1, 'Content 1', 'content-1', 'Body 1', '["tag1"]', 'draft'),
					(1, 'Content 2', 'content-2', 'Body 2', '["tag2"]', 'draft')
				`)
				return err
			},
			wantErr: nil,
			validateFn: func(t *testing.T, contents []*content.Content) {
				assert.Len(t, contents, 1, "expected 1 content")
			},
		},
		{
			name:   "retrieval with offset",
			userID: 1,
			limit:  10,
			offset: 1,
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status) VALUES
					(1, 'Content 1', 'content-1', 'Body 1', '["tag1"]', 'draft'),
					(1, 'Content 2', 'content-2', 'Body 2', '["tag2"]', 'draft')
				`)
				return err
			},
			wantErr: nil,
			validateFn: func(t *testing.T, contents []*content.Content) {
				assert.Len(t, contents, 1, "expected 1 content")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContentTestDB(t)
			defer teardownContentTestDB(t, db)

			require.NoError(t, tt.setupDB(db), "failed to setup db")

			repo := sqlite.NewContentRepository(db)
			contents, err := repo.GetByUser(context.Background(), tt.userID, tt.limit, tt.offset)

			if tt.wantErr != nil {
				assert.Error(t, err, "ContentRepository.GetByUser() expected error")
				if err != nil {
					assert.Equal(t, tt.wantErr.Error(), err.Error(), "ContentRepository.GetByUser() error mismatch")
				}
				return
			}

			require.NoError(t, err, "ContentRepository.GetByUser() unexpected error")

			if tt.validateFn != nil {
				tt.validateFn(t, contents)
			}
		})
	}
}

func TestContentRepository_ListByCursor(t *testing.T) {
	tests := []struct {
		name       string
		userID     int
		limit      int
		beforeID   int
		filters    content.ContentFilters
		setupDB    func(*sql.DB) error
		wantIDs    []int
		wantLen    int
	}{
		{
			name:     "first page returns newest-first scoped to caller",
			userID:   1,
			limit:    10,
			beforeID: 0,
			filters:  content.ContentFilters{},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (id, user_id, title, slug, content, tags, status) VALUES
					(1, 1, 'Oldest', 'oldest', 'Body 1', '[]', 'draft'),
					(2, 1, 'Middle', 'middle', 'Body 2', '[]', 'draft'),
					(3, 1, 'Newest', 'newest', 'Body 3', '[]', 'draft'),
					(4, 2, 'Other User', 'other-user', 'Body 4', '[]', 'draft')
				`)
				return err
			},
			wantIDs: []int{3, 2, 1},
			wantLen: 3,
		},
		{
			name:     "beforeID filters to older rows",
			userID:   1,
			limit:    10,
			beforeID: 2,
			filters:  content.ContentFilters{},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (id, user_id, title, slug, content, tags, status) VALUES
					(1, 1, 'Oldest', 'oldest', 'Body 1', '[]', 'draft'),
					(2, 1, 'Middle', 'middle', 'Body 2', '[]', 'draft'),
					(3, 1, 'Newest', 'newest', 'Body 3', '[]', 'draft')
				`)
				return err
			},
			wantIDs: []int{1},
			wantLen: 1,
		},
		{
			name:     "limit is honored",
			userID:   1,
			limit:    2,
			beforeID: 0,
			filters:  content.ContentFilters{},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (id, user_id, title, slug, content, tags, status) VALUES
					(1, 1, 'Oldest', 'oldest', 'Body 1', '[]', 'draft'),
					(2, 1, 'Middle', 'middle', 'Body 2', '[]', 'draft'),
					(3, 1, 'Newest', 'newest', 'Body 3', '[]', 'draft')
				`)
				return err
			},
			wantIDs: []int{3, 2},
			wantLen: 2,
		},
		{
			name:     "empty set when beforeID below minimum id",
			userID:   1,
			limit:    10,
			beforeID: 1,
			filters:  content.ContentFilters{},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (id, user_id, title, slug, content, tags, status) VALUES
					(1, 1, 'Oldest', 'oldest', 'Body 1', '[]', 'draft'),
					(2, 1, 'Newest', 'newest', 'Body 2', '[]', 'draft')
				`)
				return err
			},
			wantIDs: []int{},
			wantLen: 0,
		},
		{
			name:     "filter status=draft drops published",
			userID:   1,
			limit:    10,
			beforeID: 0,
			filters:  content.ContentFilters{Status: "draft"},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (id, user_id, title, slug, content, tags, status) VALUES
					(1, 1, 'Draft One', 'd1', 'b', '[]', 'draft'),
					(2, 1, 'Pub One', 'p1', 'b', '[]', 'published'),
					(3, 1, 'Draft Two', 'd2', 'b', '[]', 'draft')
				`)
				return err
			},
			wantIDs: []int{3, 1},
			wantLen: 2,
		},
		{
			name:     "filter post_type=page drops posts",
			userID:   1,
			limit:    10,
			beforeID: 0,
			filters:  content.ContentFilters{PostType: "page"},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (id, user_id, title, slug, content, tags, status, post_type) VALUES
					(1, 1, 'A Post', 'a', 'b', '[]', 'draft', 'post'),
					(2, 1, 'A Page', 'p', 'b', '[]', 'draft', 'page')
				`)
				return err
			},
			wantIDs: []int{2},
			wantLen: 1,
		},
		{
			name:     "filter language=en drops others",
			userID:   1,
			limit:    10,
			beforeID: 0,
			filters:  content.ContentFilters{Language: "en"},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (id, user_id, title, slug, content, tags, status, language) VALUES
					(1, 1, 'EN One', 'en1', 'b', '[]', 'draft', 'en'),
					(2, 1, 'FR One', 'fr1', 'b', '[]', 'draft', 'fr')
				`)
				return err
			},
			wantIDs: []int{1},
			wantLen: 1,
		},
		{
			name:     "filter single tag matches only items containing it",
			userID:   1,
			limit:    10,
			beforeID: 0,
			filters:  content.ContentFilters{Tags: []string{"go"}},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (id, user_id, title, slug, content, tags, status) VALUES
					(1, 1, 'Go Post', 'g', 'b', '["go","tutorial"]', 'draft'),
					(2, 1, 'Python Post', 'p', 'b', '["python"]', 'draft'),
					(3, 1, 'Tagged', 't', 'b', '[]', 'draft')
				`)
				return err
			},
			wantIDs: []int{1},
			wantLen: 1,
		},
		{
			name:     "filter multiple tags AND-of-tags requires every tag to be present",
			userID:   1,
			limit:    10,
			beforeID: 0,
			filters:  content.ContentFilters{Tags: []string{"go", "tutorial"}},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (id, user_id, title, slug, content, tags, status) VALUES
					(1, 1, 'Both', 'b', 'b', '["go","tutorial"]', 'draft'),
					(2, 1, 'Just Go', 'g', 'b', '["go"]', 'draft'),
					(3, 1, 'Just Tutorial', 't', 'b', '["tutorial"]', 'draft')
				`)
				return err
			},
			wantIDs: []int{1},
			wantLen: 1,
		},
		{
			name:     "filter author matches joined users.name case-insensitively",
			userID:   1,
			limit:    10,
			beforeID: 0,
			filters:  content.ContentFilters{Author: "ALICE"},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO users (id, username, password_hash, role, status, name) VALUES
					(1, 'alice-user', 'h', 'admin', 'active', 'Alice')
				`)
				if err != nil {
					return err
				}
				_, err = db.Exec(`
					INSERT INTO content_items (id, user_id, title, slug, content, tags, status) VALUES
					(1, 1, 'Alice Post', 'a', 'b', '[]', 'draft')
				`)
				return err
			},
			wantIDs: []int{1},
			wantLen: 1,
		},
		{
			name:     "filter search matches title substring case-insensitively",
			userID:   1,
			limit:    10,
			beforeID: 0,
			filters:  content.ContentFilters{Search: "GOLANG"},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (id, user_id, title, slug, content, tags, status) VALUES
					(1, 1, 'Golang Rocks', 'g', 'b', '[]', 'draft'),
					(2, 1, 'Python Is Fine', 'p', 'b', '[]', 'draft')
				`)
				return err
			},
			wantIDs: []int{1},
			wantLen: 1,
		},
		{
			name:     "filter status + post_type combined",
			userID:   1,
			limit:    10,
			beforeID: 0,
			filters:  content.ContentFilters{Status: "draft", PostType: "post"},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (id, user_id, title, slug, content, tags, status, post_type) VALUES
					(1, 1, 'Draft Post', 'dp', 'b', '[]', 'draft', 'post'),
					(2, 1, 'Draft Page', 'dpg', 'b', '[]', 'draft', 'page'),
					(3, 1, 'Pub Post', 'pp', 'b', '[]', 'published', 'post')
				`)
				return err
			},
			wantIDs: []int{1},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContentTestDB(t)
			defer teardownContentTestDB(t, db)

			require.NoError(t, tt.setupDB(db), "failed to setup db")

			repo := sqlite.NewContentRepository(db)
			contents, err := repo.ListByCursor(context.Background(), tt.userID, tt.limit, tt.beforeID, tt.filters)

			require.NoError(t, err, "ContentRepository.ListByCursor() unexpected error")
			assert.Len(t, contents, tt.wantLen, "ContentRepository.ListByCursor() result length")

			gotIDs := make([]int, 0, len(contents))
			for _, c := range contents {
				gotIDs = append(gotIDs, c.ID)
			}
			assert.Equal(t, tt.wantIDs, gotIDs, "ContentRepository.ListByCursor() result order/IDs")
		})
	}
}

func TestContentRepository_CheckSlugUnique(t *testing.T) {
	tests := []struct {
		name       string
		slug       string
		setupDB    func(*sql.DB) error
		wantUnique bool
		wantErr    bool
	}{
		{
			name: "slug is unique",
			slug: "unique-slug",
			setupDB: func(db *sql.DB) error {
				return nil
			},
			wantUnique: true,
			wantErr:    false,
		},
		{
			name: "slug already exists",
			slug: "existing-slug",
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status)
					VALUES (1, 'Existing', 'existing-slug', 'Body', '[]', 'draft')
				`)
				return err
			},
			wantUnique: false,
			wantErr:    false,
		},
		{
			name: "slug with leading/trailing whitespace",
			slug: "  trimmed-slug  ",
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status)
					VALUES (1, 'Trimmed', 'trimmed-slug', 'Body', '[]', 'draft')
				`)
				return err
			},
			wantUnique: false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContentTestDB(t)
			defer teardownContentTestDB(t, db)

			require.NoError(t, tt.setupDB(db), "failed to setup db")

			repo := sqlite.NewContentRepository(db)
			unique, err := repo.CheckSlugUnique(context.Background(), tt.slug, "en")

			if tt.wantErr {
				assert.Error(t, err, "ContentRepository.CheckSlugUnique() expected error")
				return
			}

			require.NoError(t, err, "ContentRepository.CheckSlugUnique() unexpected error")
			assert.Equal(t, tt.wantUnique, unique, "ContentRepository.CheckSlugUnique() result mismatch")
		})
	}
}

func TestContentRepository_GetByID(t *testing.T) {
	tests := []struct {
		name       string
		id         int
		setupDB    func(*sql.DB) error
		wantErr    error
		validateFn func(*testing.T, *content.Content)
	}{
		{
			name: "successful retrieval",
			id:   1,
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status)
					VALUES (1, 'Test Content', 'test-content', 'Test body', '["test"]', 'draft')
				`)
				return err
			},
			wantErr: nil,
			validateFn: func(t *testing.T, content *content.Content) {
				assert.Equal(t, 1, content.ID, "expected ID 1")
				assert.Equal(t, "Test Content", content.Title, "expected title 'Test Content'")
			},
		},
		{
			name: "content not found",
			id:   999,
			setupDB: func(db *sql.DB) error {
				return nil
			},
			wantErr:    content.ErrContentNotFound,
			validateFn: nil,
		},
		{
			name: "retrieval with published status",
			id:   1,
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status)
					VALUES (1, 'Published Content', 'published-content', 'Test body', '["news"]', 'published')
				`)
				return err
			},
			wantErr: nil,
			validateFn: func(t *testing.T, c *content.Content) {
				assert.Equal(t, content.StatusPublished, c.Status, "expected status 'published'")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContentTestDB(t)
			defer teardownContentTestDB(t, db)

			require.NoError(t, tt.setupDB(db), "failed to setup db")

			repo := sqlite.NewContentRepository(db)
			content, err := repo.GetByID(context.Background(), tt.id)

			if tt.wantErr != nil {
				assert.Error(t, err, "ContentRepository.GetByID() expected error")
				assert.ErrorIs(t, err, tt.wantErr, "ContentRepository.GetByID() error mismatch")
				return
			}

			require.NoError(t, err, "ContentRepository.GetByID() unexpected error")
			require.NotNil(t, content, "ContentRepository.GetByID() expected content")

			if tt.validateFn != nil {
				tt.validateFn(t, content)
			}
		})
	}
}

func TestContentRepository_Update(t *testing.T) {
	tests := []struct {
		name       string
		content    *content.Content
		setupDB    func(*sql.DB) error
		wantErr    error
		validateFn func(*testing.T, *sql.DB, *content.Content)
	}{
		{
			name: "successful update of draft content",
			content: &content.Content{
				ID:      1,
				UserID:  1,
				Title:   "Updated Title",
				Slug:    "test-content",
				Content: "Updated content",
				Tags:    []string{"updated"},
				Status:  content.StatusDraft,
			},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status)
					VALUES (1, 'Original Title', 'test-content', 'Original content', '["original"]', 'draft')
				`)
				return err
			},
			wantErr: nil,
			validateFn: func(t *testing.T, db *sql.DB, content *content.Content) {
				var title, contentText, status string
				var tagsJSON string
				err := db.QueryRow(`
					SELECT title, content, tags, status FROM content_items WHERE id = ?
				`, content.ID).Scan(&title, &contentText, &tagsJSON, &status)
				require.NoError(t, err, "failed to query updated content")
				assert.Equal(t, "Updated Title", title, "expected title 'Updated Title'")
				assert.Equal(t, "Updated content", contentText, "expected content 'Updated content'")
				assert.Equal(t, "draft", status, "expected status 'draft'")
			},
		},
		{
			name: "successful publish draft to published",
			content: &content.Content{
				ID:      1,
				UserID:  1,
				Title:   "Title",
				Slug:    "test-content",
				Content: "Content",
				Tags:    []string{},
				Status:  content.StatusPublished,
			},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status)
					VALUES (1, 'Title', 'test-content', 'Content', '[]', 'draft')
				`)
				return err
			},
			wantErr: nil,
			validateFn: func(t *testing.T, db *sql.DB, content *content.Content) {
				var status string
				err := db.QueryRow(`
					SELECT status FROM content_items WHERE id = ?
				`, content.ID).Scan(&status)
				require.NoError(t, err, "failed to query updated content")
				assert.Equal(t, "published", status, "expected status 'published'")
			},
		},
		{
			name: "successful unpublish published to draft",
			content: &content.Content{
				ID:      1,
				UserID:  1,
				Title:   "Title",
				Slug:    "test-content",
				Content: "Content",
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status)
					VALUES (1, 'Title', 'test-content', 'Content', '[]', 'published')
				`)
				return err
			},
			wantErr: nil,
			validateFn: func(t *testing.T, db *sql.DB, content *content.Content) {
				var status string
				err := db.QueryRow(`
					SELECT status FROM content_items WHERE id = ?
				`, content.ID).Scan(&status)
				require.NoError(t, err, "failed to query updated content")
				assert.Equal(t, "draft", status, "expected status 'draft'")
			},
		},
		{
			name: "successful update of published content",
			content: &content.Content{
				ID:      1,
				UserID:  1,
				Title:   "Updated Title",
				Slug:    "test-content",
				Content: "Updated content",
				Tags:    []string{},
				Status:  content.StatusPublished,
			},
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status)
					VALUES (1, 'Title', 'test-content', 'Content', '[]', 'published')
				`)
				return err
			},
			wantErr: nil,
			validateFn: func(t *testing.T, db *sql.DB, content *content.Content) {
				var title, status string
				err := db.QueryRow(`
					SELECT title, status FROM content_items WHERE id = ?
				`, content.ID).Scan(&title, &status)
				require.NoError(t, err, "failed to query updated content")
				assert.Equal(t, "Updated Title", title, "expected title 'Updated Title'")
				assert.Equal(t, "published", status, "expected status 'published'")
			},
		},
		{
			name: "content not found",
			content: &content.Content{
				ID:      999,
				UserID:  1,
				Title:   "Title",
				Slug:    "test-content",
				Content: "Content",
				Tags:    []string{},
				Status:  content.StatusDraft,
			},
			setupDB:    func(db *sql.DB) error { return nil },
			wantErr:    content.ErrContentNotFound,
			validateFn: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContentTestDB(t)
			defer teardownContentTestDB(t, db)

			require.NoError(t, tt.setupDB(db), "failed to setup db")

			repo := sqlite.NewContentRepository(db)
			err := repo.Update(context.Background(), tt.content)

			if tt.wantErr != nil {
				assert.Error(t, err, "ContentRepository.Update() expected error")
				assert.ErrorIs(t, err, tt.wantErr, "ContentRepository.Update() error mismatch")
				return
			}

			require.NoError(t, err, "ContentRepository.Update() unexpected error")
			assert.NotEmpty(t, tt.content.UpdatedAt, "ContentRepository.Update() expected UpdatedAt to be set")

			if tt.validateFn != nil {
				tt.validateFn(t, db, tt.content)
			}
		})
	}
}

func TestContentRepository_GetPublishedByTag(t *testing.T) {
	tests := []struct {
		name       string
		tag        string
		setupDB    func(*sql.DB) error
		wantLen    int
		wantTitle  string
	}{
		{
			name: "returns content matching tag",
			tag:  "golang",
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				if err != nil {
					return err
				}
				_, err = db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status) VALUES
					(1, 'Go Basics', 'go-basics', 'body', '["golang","tutorial"]', 'published'),
					(1, 'Python Basics', 'python-basics', 'body', '["python"]', 'published'),
					(1, 'Go Advanced', 'go-advanced', 'body', '["golang"]', 'draft')
				`)
				return err
			},
			wantLen:   1,
			wantTitle: "Go Basics",
		},
		{
			name: "returns empty for non-existent tag",
			tag:  "nonexistent",
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				if err != nil {
					return err
				}
				_, err = db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status) VALUES
					(1, 'Go Basics', 'go-basics', 'body', '["golang"]', 'published')
				`)
				return err
			},
			wantLen: 0,
		},
		{
			name: "case-insensitive tag match",
			tag:  "Golang",
			setupDB: func(db *sql.DB) error {
				_, err := db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				if err != nil {
					return err
				}
				_, err = db.Exec(`
					INSERT INTO content_items (user_id, title, slug, content, tags, status) VALUES
					(1, 'Go Basics', 'go-basics', 'body', '["golang"]', 'published')
				`)
				return err
			},
			wantLen:   1,
			wantTitle: "Go Basics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContentTestDB(t)
			defer teardownContentTestDB(t, db)

			require.NoError(t, tt.setupDB(db))

			repo := sqlite.NewContentRepository(db)
			contents, err := repo.GetPublishedByTag(context.Background(), tt.tag, 10, 0)

			require.NoError(t, err)
			assert.Len(t, contents, tt.wantLen)

			if tt.wantTitle != "" && len(contents) > 0 {
				assert.Equal(t, tt.wantTitle, contents[0].Title)
			}
		})
	}
}

func TestContentRepository_GetPublishedByTag_NullTags(t *testing.T) {
	db := setupContentTestDB(t)
	defer teardownContentTestDB(t, db)

	_, err := db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO content_items (user_id, title, slug, content, tags, status) VALUES
		(1, 'No Tags Post', 'no-tags', 'body', NULL, 'published'),
		(1, 'Empty Tags Post', 'empty-tags', 'body', '', 'published'),
		(1, 'Tagged Post', 'tagged', 'body', '["golang"]', 'published')
	`)
	require.NoError(t, err)

	repo := sqlite.NewContentRepository(db)
	contents, err := repo.GetPublishedByTag(context.Background(), "golang", 10, 0)

	require.NoError(t, err)
	assert.Len(t, contents, 1)
	assert.Equal(t, "Tagged Post", contents[0].Title)
}

func TestContentRepository_SearchPublished(t *testing.T) {
	tests := []struct {
		name       string
		setupDB    func(*sql.DB)
		query      string
		limit      int
		wantCount  int
		wantErr    bool
	}{
		{
			name:  "search by title",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Golang Tutorial', 'golang-tut', 'body', '[]', 'published', 'post')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Python Guide', 'python-guide', 'body', '[]', 'published', 'post')`)
			},
			query:     "golang",
			limit:     10,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:  "search by meta_description",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, meta_description, status, post_type) VALUES (1, 'Post One', 'post-one', 'body', '[]', 'Learn golang basics', 'published', 'post')`)
			},
			query:     "golang",
			limit:     10,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:  "search case insensitive",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'GOLANG Rocks', 'golang-rocks', 'body', '[]', 'published', 'post')`)
			},
			query:     "golang",
			limit:     10,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:  "no matching results",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Golang Tutorial', 'golang-tut', 'body', '[]', 'published', 'post')`)
			},
			query:     "ruby",
			limit:     10,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:  "excludes draft posts",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Golang Draft', 'golang-draft', 'body', '[]', 'draft', 'post')`)
			},
			query:     "golang",
			limit:     10,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:  "excludes pages",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Golang Page', 'golang-page', 'body', '[]', 'published', 'page')`)
			},
			query:     "golang",
			limit:     10,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:  "respects limit",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Golang 1', 'golang-1', 'body', '[]', 'published', 'post')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Golang 2', 'golang-2', 'body', '[]', 'published', 'post')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Golang 3', 'golang-3', 'body', '[]', 'published', 'post')`)
			},
			query:     "golang",
			limit:     2,
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:  "default limit when zero",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Golang Post', 'golang-post', 'body', '[]', 'published', 'post')`)
			},
			query:     "golang",
			limit:     0,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:  "nil context uses default timeout",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Golang Nil', 'golang-nil', 'body', '[]', 'published', 'post')`)
			},
			query:     "golang",
			limit:     10,
			wantCount: 1,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContentTestDB(t)
			defer teardownContentTestDB(t, db)

			if tt.setupDB != nil {
				tt.setupDB(db)
			}

			repo := sqlite.NewContentRepository(db)

			var ctx context.Context
			if tt.name == "nil context uses default timeout" {
				ctx = nil
			} else {
				ctx = context.Background()
			}

			results, err := repo.SearchPublished(ctx, tt.query, tt.limit)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, results, tt.wantCount)
		})
	}
}

func TestContentRepository_ListByFilters_WithSearch(t *testing.T) {
	tests := []struct {
		name      string
		setupDB   func(*sql.DB)
		userID    int
		filters   content.ContentFilters
		wantCount int
		wantErr   bool
	}{
		{
			name: "search by title matches",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Golang Tutorial', 'golang-tut', 'body', '[]', 'draft', 'post')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Python Guide', 'python-guide', 'body', '[]', 'draft', 'post')`)
			},
			userID:    1,
			filters:   content.ContentFilters{Limit: 100, Offset: 0, Search: "golang"},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "search by meta_description matches",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, meta_description, status, post_type) VALUES (1, 'Post One', 'post-one', 'body', '[]', 'Learn golang basics', 'draft', 'post')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, meta_description, status, post_type) VALUES (1, 'Post Two', 'post-two', 'body', '[]', 'Python advanced', 'draft', 'post')`)
			},
			userID:    1,
			filters:   content.ContentFilters{Limit: 100, Offset: 0, Search: "golang"},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "search with no results",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Golang Tutorial', 'golang-tut', 'body', '[]', 'draft', 'post')`)
			},
			userID:    1,
			filters:   content.ContentFilters{Limit: 100, Offset: 0, Search: "ruby"},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "search case insensitive",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'GOLANG Rocks', 'golang-rocks', 'body', '[]', 'draft', 'post')`)
			},
			userID:    1,
			filters:   content.ContentFilters{Limit: 100, Offset: 0, Search: "golang"},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "search with percent special char",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, '50% Discount', 'fifty-discount', 'body', '[]', 'draft', 'post')`)
			},
			userID:    1,
			filters:   content.ContentFilters{Limit: 100, Offset: 0, Search: "50%"},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "search combined with post_type filter",
			setupDB: func(db *sql.DB) {
				_, _ = db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'author', 'hash', 'admin', 'active')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Golang Tutorial', 'golang-tut', 'body', '[]', 'draft', 'post')`)
				_, _ = db.Exec(`INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES (1, 'Golang Page', 'golang-page', 'body', '[]', 'draft', 'page')`)
			},
			userID:    0,
			filters:   content.ContentFilters{Limit: 100, Offset: 0, Search: "golang", PostType: "post"},
			wantCount: 1,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContentTestDB(t)
			defer teardownContentTestDB(t, db)

			if tt.setupDB != nil {
				tt.setupDB(db)
			}

			repo := sqlite.NewContentRepository(db)
			results, err := repo.ListByFilters(context.Background(), tt.userID, tt.filters)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, results, tt.wantCount)
		})
	}
}
