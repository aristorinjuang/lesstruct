package sqlite_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/repository/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentRepository_GetPublishedByPostType_LanguageFallback(t *testing.T) {
	tests := []struct {
		name      string
		languages []string
		wantSlugs []string
	}{
		{
			name:      "prefers primary language and falls back to secondary",
			languages: []string{"en", "id"},
			wantSlugs: []string{"hello-en", "hanya-id"},
		},
		{
			name:      "single language keeps strict behaviour",
			languages: []string{"en"},
			wantSlugs: []string{"hello-en"},
		},
		{
			name:      "empty slice returns every configured row",
			languages: nil,
			wantSlugs: []string{"hello-en", "hello-id", "hanya-id", "bonjour"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContentTestDB(t)
			defer teardownContentTestDB(t, db)

			_, err := db.Exec(`
				INSERT INTO content_items (id, user_id, title, slug, content, tags, status, post_type, language, translation_group_id, created_at) VALUES
				(1, 1, 'Hello EN', 'hello-en', 'Body', '[]', 'published', 'post', 'en', NULL, '2026-03-01 10:00:00'),
				(2, 1, 'Hello ID', 'hello-id', 'Body', '[]', 'published', 'post', 'id', 1, '2026-02-01 10:00:00'),
				(3, 1, 'Hanya ID', 'hanya-id', 'Body', '[]', 'published', 'post', 'id', NULL, '2026-01-15 10:00:00'),
				(4, 1, 'Bonjour', 'bonjour', 'Body', '[]', 'published', 'post', 'fr', NULL, '2026-01-10 10:00:00')
			`)
			require.NoError(t, err)

			repo := sqlite.NewContentRepository(db)
			items, err := repo.GetPublishedByPostType(context.Background(), "post", tt.languages, 0, 0, 50, 0)

			require.NoError(t, err)
			slugCount := len(items)
			gotSlugs := make([]string, 0, slugCount)
			for _, item := range items {
				gotSlugs = append(gotSlugs, item.Slug)
			}
			assert.Equal(t, tt.wantSlugs, gotSlugs)
		})
	}
}

func TestContentRepository_GetPublishedByPostType_LanguageFallbackPagination(t *testing.T) {
	db := setupContentTestDB(t)
	defer teardownContentTestDB(t, db)

	_, err := db.Exec(`
		INSERT INTO content_items (id, user_id, title, slug, content, tags, status, post_type, language, translation_group_id, created_at) VALUES
		(1, 1, 'One EN', 'one-en', 'Body', '[]', 'published', 'post', 'en', NULL, '2026-01-01 10:00:00'),
		(2, 1, 'One ID', 'one-id', 'Body', '[]', 'published', 'post', 'id', 1, '2026-01-01 09:00:00'),
		(3, 1, 'Two EN', 'two-en', 'Body', '[]', 'published', 'post', 'en', NULL, '2026-02-01 10:00:00'),
		(4, 1, 'Two ID', 'two-id', 'Body', '[]', 'published', 'post', 'id', 3, '2026-02-01 09:00:00'),
		(5, 1, 'Three EN', 'three-en', 'Body', '[]', 'published', 'post', 'en', NULL, '2026-03-01 10:00:00')
	`)
	require.NoError(t, err)

	repo := sqlite.NewContentRepository(db)

	t.Run("first page holds one entry per group", func(t *testing.T) {
		items, err := repo.GetPublishedByPostType(context.Background(), "post", []string{"en", "id"}, 0, 0, 2, 0)
		require.NoError(t, err)
		require.Len(t, items, 2)
		assert.Equal(t, "three-en", items[0].Slug)
		assert.Equal(t, "two-en", items[1].Slug)
	})

	t.Run("second page continues after deduplicated groups", func(t *testing.T) {
		items, err := repo.GetPublishedByPostType(context.Background(), "post", []string{"en", "id"}, 0, 0, 2, 2)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "one-en", items[0].Slug)
	})
}

func TestContentRepository_GetPublishedArchive_LanguageFallback(t *testing.T) {
	db := setupContentTestDB(t)
	defer teardownContentTestDB(t, db)

	_, err := db.Exec(`
		INSERT INTO content_items (id, user_id, title, slug, content, tags, status, post_type, language, translation_group_id, created_at) VALUES
		(1, 1, 'EN Post', 'en-post', 'Body', '[]', 'published', 'post', 'en', NULL, '2026-01-05 10:00:00'),
		(2, 1, 'ID Sibling', 'id-sibling', 'Body', '[]', 'published', 'post', 'id', 1, '2026-02-05 10:00:00'),
		(3, 1, 'ID Only', 'id-only', 'Body', '[]', 'published', 'post', 'id', NULL, '2026-03-05 10:00:00')
	`)
	require.NoError(t, err)

	repo := sqlite.NewContentRepository(db)
	archive, err := repo.GetPublishedArchive(context.Background(), "post", []string{"en", "id"})

	require.NoError(t, err)
	require.Len(t, archive, 2)
	assert.Equal(t, 2026, archive[0].Year)
	assert.Equal(t, 3, archive[0].Month)
	assert.Equal(t, 1, archive[0].Count)
	assert.Equal(t, 2026, archive[1].Year)
	assert.Equal(t, 1, archive[1].Month)
	assert.Equal(t, 1, archive[1].Count)
}

func TestContentRepository_GetPublishedByTag_LanguageFallback(t *testing.T) {
	db := setupContentTestDB(t)
	defer teardownContentTestDB(t, db)

	_, err := db.Exec(`
		INSERT INTO content_items (id, user_id, title, slug, content, tags, status, post_type, language, translation_group_id, created_at) VALUES
		(1, 1, 'Both Tagged EN', 'both-tagged-en', 'Body', '["go"]', 'published', 'post', 'en', NULL, '2026-01-05 10:00:00'),
		(2, 1, 'Both Tagged ID', 'both-tagged-id', 'Body', '["go"]', 'published', 'post', 'id', 1, '2026-01-04 10:00:00'),
		(3, 1, 'Only ID Tagged EN', 'only-id-tagged-en', 'Body', '[]', 'published', 'post', 'en', NULL, '2026-01-03 10:00:00'),
		(4, 1, 'Only ID Tagged ID', 'only-id-tagged-id', 'Body', '["go"]', 'published', 'post', 'id', 3, '2026-01-02 10:00:00')
	`)
	require.NoError(t, err)

	repo := sqlite.NewContentRepository(db)
	items, err := repo.GetPublishedByTag(context.Background(), "go", []string{"en", "id"}, 0, 0, 50, 0)

	require.NoError(t, err)
	require.Len(t, items, 2)
	// The primary-language version wins when it also carries the tag.
	assert.Equal(t, "both-tagged-en", items[0].Slug)
	// A translation that carries the tag is not erased by an untagged sibling.
	assert.Equal(t, "only-id-tagged-id", items[1].Slug)
}

func TestContentRepository_GetPublishedByAuthorUsername_LanguageFallback(t *testing.T) {
	db := setupContentTestDB(t)
	defer teardownContentTestDB(t, db)

	_, err := db.Exec(`INSERT INTO users (id, username, password_hash, role, status) VALUES (1, 'admin', 'hash', 'admin', 'active')`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO content_items (id, user_id, title, slug, content, tags, status, post_type, language, translation_group_id, created_at) VALUES
		(1, 1, 'Profile EN', 'profile-en', 'Body', '[]', 'published', 'post', 'en', NULL, '2026-01-05 10:00:00'),
		(2, 1, 'Profile ID', 'profile-id', 'Body', '[]', 'published', 'post', 'id', 1, '2026-01-04 10:00:00'),
		(3, 1, 'Standalone ID', 'standalone-id', 'Body', '[]', 'published', 'post', 'id', NULL, '2026-01-03 10:00:00')
	`)
	require.NoError(t, err)

	repo := sqlite.NewContentRepository(db)
	items, err := repo.GetPublishedByAuthorUsername(context.Background(), "admin", []string{"en", "id"}, 50, 0)

	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "profile-en", items[0].Slug)
	assert.Equal(t, "standalone-id", items[1].Slug)
}
