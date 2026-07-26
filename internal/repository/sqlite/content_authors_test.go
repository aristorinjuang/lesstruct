package sqlite_test

import (
	"context"
	"database/sql"
	"testing"

	content "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/repository/sqlite"
	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAuthorsTestDB seeds users (with profile pictures and custom_fields) and
// published/draft content across distinct post types so the aggregation can be
// exercised. Each user carries a numeric "points" custom field used by the
// sort-by-cf test cases.
func setupAuthorsTestDB(t *testing.T) *sql.DB {
	t.Helper()
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
			profile_picture TEXT,
			custom_fields TEXT,
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
			format TEXT DEFAULT 'tiptap',
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
	`)
	require.NoError(t, err, "failed to create test tables")

	// Jane (2 published, points=42, tier=gold), Bob (NULL name → falls back to
	// username, points=87, tier=silver), Alice (1 published, 1 draft — draft
	// must NOT count, points=5, tier=bronze), and Dave (drafts only — must NOT
	// appear, points=1000, tier=platinum).
	_, err = db.Exec(`
		INSERT INTO users (id, username, password_hash, email, name, role, status, profile_picture, custom_fields) VALUES
		(1, 'janedoe',  'h', 'jane@example.com', 'Jane Doe',      'Author', 'verified', 'jane.webp', '{"points":42,"tier":"gold"}'),
		(2, 'bobsmith', 'h', 'bob@example.com',  NULL,            'Author', 'verified', NULL,        '{"points":87,"tier":"silver"}'),
		(3, 'alice',    'h', 'alice@example.com','Alice Johnson', 'Author', 'verified', 'alice.webp','{"points":5,"tier":"bronze"}'),
		(4, 'dave',     'h', 'dave@example.com', 'Dave Drafts',   'Author', 'verified', NULL,        '{"points":1000,"tier":"platinum"}');

		INSERT INTO content_items (user_id, title, slug, status, post_type) VALUES
		(1, 'Jane Article', 'jane-article', 'published', 'article'),
		(1, 'Jane Event',   'jane-event',   'published', 'event'),
		(2, 'Bob Post',     'bob-post',     'published', 'post'),
		(3, 'Alice Post',   'alice-post',   'published', 'post'),
		(3, 'Alice Draft',  'alice-draft',  'draft',     'post'),
		(4, 'Dave Draft 1', 'dave-draft-1', 'draft',     'post'),
		(4, 'Dave Draft 2', 'dave-draft-2', 'draft',     'post');
	`)
	require.NoError(t, err, "failed to seed test data")

	return db
}

func TestContentRepository_GetPublishedAuthors(t *testing.T) {
	t.Run("aggregates, orders by count desc, excludes draft-only users", func(t *testing.T) {
		db := setupAuthorsTestDB(t)
		defer func() { _ = db.Close() }()

		repo := sqlite.NewContentRepository(db)
		authors, err := repo.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{Limit: 100, Offset: 0})

		require.NoError(t, err)
		require.Len(t, authors, 3, "draft-only Dave must be excluded")

		// Ordered by content_count DESC then username ASC: Jane(2), Alice(1), Bob(1)
		assert.Equal(t, "janedoe", authors[0].Username)
		assert.Equal(t, "Jane Doe", authors[0].DisplayName)
		assert.Equal(t, 2, authors[0].ContentCount)
		assert.Equal(t, "jane.webp", authors[0].ProfilePicture)
		assert.ElementsMatch(t, []string{"article", "event"}, authors[0].PostTypes)

		// Alice and Bob both have 1; username asc → alice before bobsmith
		assert.Equal(t, "alice", authors[1].Username)
		assert.Equal(t, 1, authors[1].ContentCount)

		assert.Equal(t, "bobsmith", authors[2].Username)
		assert.Equal(t, "bobsmith", authors[2].DisplayName, "NULL name falls back to username")
		assert.Empty(t, authors[2].ProfilePicture)
		assert.Equal(t, []string{"post"}, authors[2].PostTypes)
	})

	t.Run("respects limit and offset", func(t *testing.T) {
		db := setupAuthorsTestDB(t)
		defer func() { _ = db.Close() }()

		repo := sqlite.NewContentRepository(db)
		page1, err := repo.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{Limit: 2, Offset: 0})
		require.NoError(t, err)
		require.Len(t, page1, 2)
		assert.Equal(t, "janedoe", page1[0].Username)

		page2, err := repo.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{Limit: 2, Offset: 2})
		require.NoError(t, err)
		require.Len(t, page2, 1, "only one author remains after offset 2")
		assert.Equal(t, "bobsmith", page2[0].Username)
	})

	t.Run("returns empty slice when no published content", func(t *testing.T) {
		db, err := sql.Open("sqlite", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()
		_, err = db.Exec(`
			CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT, password_hash TEXT, name TEXT, role TEXT, status TEXT, profile_picture TEXT, custom_fields TEXT);
			CREATE TABLE content_items (id INTEGER PRIMARY KEY, user_id INTEGER, title TEXT, slug TEXT, status TEXT, format TEXT DEFAULT 'tiptap', post_type TEXT);
		`)
		require.NoError(t, err)

		repo := sqlite.NewContentRepository(db)
		authors, err := repo.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{Limit: 100, Offset: 0})

		require.NoError(t, err)
		assert.Empty(t, authors)
	})

	t.Run("post_types is never nil so JSON renders []", func(t *testing.T) {
		db := setupAuthorsTestDB(t)
		defer func() { _ = db.Close() }()

		repo := sqlite.NewContentRepository(db)
		authors, err := repo.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{Limit: 100, Offset: 0})
		require.NoError(t, err)
		for _, a := range authors {
			assert.NotNil(t, a.PostTypes)
		}
	})
}

func TestContentRepository_GetPublishedAuthors_SortByCustomField(t *testing.T) {
	t.Run("sorts by numeric custom field desc when SortBy is set", func(t *testing.T) {
		db := setupAuthorsTestDB(t)
		defer func() { _ = db.Close() }()

		repo := sqlite.NewContentRepository(db)
		authors, err := repo.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{
			Limit:     100,
			Offset:    0,
			SortBy:    "points",
			SortOrder: string(content.SortOrderDesc),
		})
		require.NoError(t, err)
		require.Len(t, authors, 3, "draft-only Dave must still be excluded even when sorting")

		// Points: Bob=87, Jane=42, Alice=5. Descending order.
		assert.Equal(t, "bobsmith", authors[0].Username)
		assert.Equal(t, "janedoe", authors[1].Username)
		assert.Equal(t, "alice", authors[2].Username)
	})

	t.Run("sorts ascending when SortOrder is asc", func(t *testing.T) {
		db := setupAuthorsTestDB(t)
		defer func() { _ = db.Close() }()

		repo := sqlite.NewContentRepository(db)
		authors, err := repo.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{
			Limit:     100,
			Offset:    0,
			SortBy:    "points",
			SortOrder: string(content.SortOrderAsc),
		})
		require.NoError(t, err)
		require.Len(t, authors, 3)

		// Ascending: Alice=5, Jane=42, Bob=87.
		assert.Equal(t, "alice", authors[0].Username)
		assert.Equal(t, "janedoe", authors[1].Username)
		assert.Equal(t, "bobsmith", authors[2].Username)
	})

	t.Run("falls back to desc when SortOrder is empty", func(t *testing.T) {
		db := setupAuthorsTestDB(t)
		defer func() { _ = db.Close() }()

		repo := sqlite.NewContentRepository(db)
		authors, err := repo.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{
			Limit:  100,
			Offset: 0,
			SortBy: "points",
		})
		require.NoError(t, err)
		require.Len(t, authors, 3)
		assert.Equal(t, "bobsmith", authors[0].Username, "empty SortOrder defaults to desc")
	})
}

func TestContentRepository_GetPublishedAuthors_FilterByCustomField(t *testing.T) {
	t.Run("filter by minimum points excludes low-ranking authors", func(t *testing.T) {
		db := setupAuthorsTestDB(t)
		defer func() { _ = db.Close() }()

		repo := sqlite.NewContentRepository(db)
		authors, err := repo.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{
			Limit: 100,
			Offset: 0,
			CustomFieldFilters: []content.CustomFieldFilter{
				{Field: "points", Operator: content.FilterOpMin, Value: "40"},
			},
		})
		require.NoError(t, err)
		require.Len(t, authors, 2, "only authors with points >= 40 (Jane=42, Bob=87) qualify")

		usernames := []string{authors[0].Username, authors[1].Username}
		assert.ElementsMatch(t, []string{"janedoe", "bobsmith"}, usernames)
	})

	t.Run("filter by exact string value matches authors with that tier", func(t *testing.T) {
		db := setupAuthorsTestDB(t)
		defer func() { _ = db.Close() }()

		repo := sqlite.NewContentRepository(db)
		authors, err := repo.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{
			Limit: 100,
			Offset: 0,
			CustomFieldFilters: []content.CustomFieldFilter{
				{Field: "tier", Operator: content.FilterOpEqual, Value: "silver"},
			},
		})
		require.NoError(t, err)
		require.Len(t, authors, 1)
		assert.Equal(t, "bobsmith", authors[0].Username)
	})

	t.Run("combines filter and sort on the same field", func(t *testing.T) {
		db := setupAuthorsTestDB(t)
		defer func() { _ = db.Close() }()

		repo := sqlite.NewContentRepository(db)
		authors, err := repo.GetPublishedAuthors(context.Background(), content.PublishedAuthorFilters{
			Limit:     100,
			Offset:    0,
			SortBy:    "points",
			SortOrder: string(content.SortOrderDesc),
			CustomFieldFilters: []content.CustomFieldFilter{
				{Field: "points", Operator: content.FilterOpMin, Value: "10"},
			},
		})
		require.NoError(t, err)
		require.Len(t, authors, 2, "Jane=42 and Bob=87 qualify; Alice=5 does not")

		// Descending by points: Bob first, then Jane.
		assert.Equal(t, "bobsmith", authors[0].Username)
		assert.Equal(t, "janedoe", authors[1].Username)
	})
}
