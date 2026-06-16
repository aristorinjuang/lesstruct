package sqlite_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/repository/sqlite"
	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCustomFieldsTestDB(t *testing.T) *sql.DB {
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

func TestContentRepository_CustomFields_Create(t *testing.T) {
	t.Run("content creation with custom fields", func(t *testing.T) {
		db := setupCustomFieldsTestDB(t)
		defer func() { assert.NoError(t, db.Close()) }()

		repo := sqlite.NewContentRepository(db)
		customFields := map[string]any{
			"price":    "$4.50",
			"servings": float64(2),
			"available": true,
		}
		c := &content.Content{
			UserID:       1,
			Title:        "Chocolate Croissant",
			Slug:         "chocolate-croissant",
			Content:      `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Delicious"}]}]}`,
			Tags:         []string{"pastry"},
			Status:       content.StatusDraft,
			PostType:     "menu-item",
			CustomFields: customFields,
		}

		err := repo.Create(context.Background(), c)
		require.NoError(t, err, "Create with custom fields should not error")

		// Verify stored in DB
		var customFieldsJSON string
		err = db.QueryRow(`SELECT custom_fields FROM content_items WHERE id = ?`, c.ID).Scan(&customFieldsJSON)
		require.NoError(t, err)

		var stored map[string]any
		require.NoError(t, json.Unmarshal([]byte(customFieldsJSON), &stored))
		assert.Equal(t, "$4.50", stored["price"])
		assert.Equal(t, float64(2), stored["servings"])
		assert.Equal(t, true, stored["available"])
	})

	t.Run("content creation without custom fields", func(t *testing.T) {
		db := setupCustomFieldsTestDB(t)
		defer func() { assert.NoError(t, db.Close()) }()

		repo := sqlite.NewContentRepository(db)
		c := &content.Content{
			UserID:   1,
			Title:    "Plain Post",
			Slug:     "plain-post",
			Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`,
			Tags:     []string{},
			Status:   content.StatusDraft,
		}

		err := repo.Create(context.Background(), c)
		require.NoError(t, err, "Create without custom fields should not error")

		var customFieldsJSON sql.NullString
		err = db.QueryRow(`SELECT custom_fields FROM content_items WHERE id = ?`, c.ID).Scan(&customFieldsJSON)
		require.NoError(t, err)
		assert.False(t, customFieldsJSON.Valid, "custom_fields should be NULL")
	})
}

func TestContentRepository_CustomFields_GetBySlug(t *testing.T) {
	t.Run("retrieves content with custom fields", func(t *testing.T) {
		db := setupCustomFieldsTestDB(t)
		defer func() { assert.NoError(t, db.Close()) }()

		_, err := db.Exec(`
			INSERT INTO content_items (user_id, title, slug, content, tags, status, custom_fields)
			VALUES (1, 'Test', 'test-slug', 'Content', '[]', 'draft', '{"price":"$4.50","servings":2}')
		`)
		require.NoError(t, err)

		repo := sqlite.NewContentRepository(db)
		result, err := repo.GetBySlug(context.Background(), "test-slug", "en")
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "$4.50", result.CustomFields["price"])
		assert.Equal(t, float64(2), result.CustomFields["servings"])
	})

	t.Run("retrieves content without custom fields", func(t *testing.T) {
		db := setupCustomFieldsTestDB(t)
		defer func() { assert.NoError(t, db.Close()) }()

		_, err := db.Exec(`
			INSERT INTO content_items (user_id, title, slug, content, tags, status)
			VALUES (1, 'No CF', 'no-cf', 'Content', '[]', 'draft')
		`)
		require.NoError(t, err)

		repo := sqlite.NewContentRepository(db)
		result, err := repo.GetBySlug(context.Background(), "no-cf", "en")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Nil(t, result.CustomFields)
	})
}

func TestContentRepository_CustomFields_GetByID(t *testing.T) {
	t.Run("retrieves content with custom fields by ID", func(t *testing.T) {
		db := setupCustomFieldsTestDB(t)
		defer func() { assert.NoError(t, db.Close()) }()

		_, err := db.Exec(`
			INSERT INTO content_items (user_id, title, slug, content, tags, status, custom_fields)
			VALUES (1, 'Test', 'test-id-cf', 'Content', '[]', 'draft', '{"category":"Pastry","available":true}')
		`)
		require.NoError(t, err)

		repo := sqlite.NewContentRepository(db)
		result, err := repo.GetByID(context.Background(), 1)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "Pastry", result.CustomFields["category"])
		assert.Equal(t, true, result.CustomFields["available"])
	})
}

func TestContentRepository_CustomFields_GetByUser(t *testing.T) {
	t.Run("retrieves content with custom fields by user", func(t *testing.T) {
		db := setupCustomFieldsTestDB(t)
		defer func() { assert.NoError(t, db.Close()) }()

		_, err := db.Exec(`
			INSERT INTO content_items (user_id, title, slug, content, tags, status, custom_fields)
			VALUES (1, 'Item 1', 'item-1', 'Content', '[]', 'draft', '{"price":"$5.00"}')
		`)
		require.NoError(t, err)

		repo := sqlite.NewContentRepository(db)
		results, err := repo.GetByUser(context.Background(), 1, 10, 0)
		require.NoError(t, err)
		require.Len(t, results, 1)

		assert.Equal(t, "$5.00", results[0].CustomFields["price"])
	})
}

func TestContentRepository_CustomFields_Update(t *testing.T) {
	t.Run("update with custom fields", func(t *testing.T) {
		db := setupCustomFieldsTestDB(t)
		defer func() { assert.NoError(t, db.Close()) }()

		_, err := db.Exec(`
			INSERT INTO content_items (user_id, title, slug, content, tags, status)
			VALUES (1, 'Title', 'test-update-cf', 'Content', '[]', 'draft')
		`)
		require.NoError(t, err)

		repo := sqlite.NewContentRepository(db)
		existing, err := repo.GetByID(context.Background(), 1)
		require.NoError(t, err)

		existing.Title = "Updated"
		existing.Content = "Updated content"
		existing.CustomFields = map[string]any{
			"color": "red",
			"count": float64(5),
		}
		err = repo.Update(context.Background(), existing)
		require.NoError(t, err)

		// Verify updated
		result, err := repo.GetByID(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, "red", result.CustomFields["color"])
		assert.Equal(t, float64(5), result.CustomFields["count"])
	})

	t.Run("update clearing custom fields", func(t *testing.T) {
		db := setupCustomFieldsTestDB(t)
		defer func() { assert.NoError(t, db.Close()) }()

		_, err := db.Exec(`
			INSERT INTO content_items (user_id, title, slug, content, tags, status, custom_fields)
			VALUES (1, 'Title', 'test-clear-cf', 'Content', '[]', 'draft', '{"price":"$4.50"}')
		`)
		require.NoError(t, err)

		repo := sqlite.NewContentRepository(db)
		existing, err := repo.GetByID(context.Background(), 1)
		require.NoError(t, err)
		require.NotNil(t, existing.CustomFields)

		existing.Title = "Cleared"
		existing.Content = "Cleared content"
		existing.CustomFields = nil
		err = repo.Update(context.Background(), existing)
		require.NoError(t, err)

		result, err := repo.GetByID(context.Background(), 1)
		require.NoError(t, err)
		assert.Nil(t, result.CustomFields)
	})
}

func TestContentRepository_CustomFields_PublishedQueries(t *testing.T) {
	setupPublished := func(t *testing.T, db *sql.DB) {
		_, err := db.Exec(`
			INSERT INTO content_items (user_id, title, slug, content, tags, status, custom_fields)
			VALUES (1, 'Published', 'published-cf', 'Content', '[]', 'published', '{"price":"$3.00"}')
		`)
		require.NoError(t, err)
	}

	t.Run("GetPublished includes custom fields", func(t *testing.T) {
		db := setupCustomFieldsTestDB(t)
		defer func() { assert.NoError(t, db.Close()) }()
		setupPublished(t, db)

		repo := sqlite.NewContentRepository(db)
		results, err := repo.GetPublished(context.Background(), 10, 0)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "$3.00", results[0].CustomFields["price"])
	})

	t.Run("GetPublishedBySlug includes custom fields", func(t *testing.T) {
		db := setupCustomFieldsTestDB(t)
		defer func() { assert.NoError(t, db.Close()) }()
		setupPublished(t, db)

		repo := sqlite.NewContentRepository(db)
		result, err := repo.GetPublishedBySlug(context.Background(), "published-cf", "en")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "$3.00", result.CustomFields["price"])
	})
}
