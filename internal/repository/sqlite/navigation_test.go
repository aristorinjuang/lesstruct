package sqlite_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/repository/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentRepository_GetPublishedPages(t *testing.T) {
	t.Run("returns published pages ordered by title", func(t *testing.T) {
		db := setupContentTestDB(t)
		defer teardownContentTestDB(t, db)

		_, err := db.Exec(`
			INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES
			(1, 'Zebra Page', 'zebra-page', 'Body', '[]', 'published', 'page'),
			(1, 'About', 'about', 'Body', '[]', 'published', 'page'),
			(1, 'Draft Page', 'draft-page', 'Body', '[]', 'draft', 'page'),
			(1, 'Blog Post', 'blog-post', 'Body', '[]', 'published', 'post')
		`)
		require.NoError(t, err)

		repo := sqlite.NewContentRepository(db)
		pages, err := repo.GetPublishedPages(context.Background())

		require.NoError(t, err)
		require.Len(t, pages, 2)
		assert.Equal(t, "About", pages[0].Title)
		assert.Equal(t, "Zebra Page", pages[1].Title)
	})

	t.Run("returns empty when no published pages", func(t *testing.T) {
		db := setupContentTestDB(t)
		defer teardownContentTestDB(t, db)

		repo := sqlite.NewContentRepository(db)
		pages, err := repo.GetPublishedPages(context.Background())

		require.NoError(t, err)
		assert.Empty(t, pages)
	})
}

func TestContentRepository_GetPublishedCustomPostTypes(t *testing.T) {
	t.Run("returns distinct custom post types with published content", func(t *testing.T) {
		db := setupContentTestDB(t)
		defer teardownContentTestDB(t, db)

		_, err := db.Exec(`
			INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES
			(1, 'Item 1', 'item-1', 'Body', '[]', 'published', 'menu-item'),
			(1, 'Item 2', 'item-2', 'Body', '[]', 'published', 'menu-item'),
			(1, 'Member 1', 'member-1', 'Body', '[]', 'published', 'team-member'),
			(1, 'Draft', 'draft-item', 'Body', '[]', 'draft', 'menu-item'),
			(1, 'Blog Post', 'blog-post', 'Body', '[]', 'published', 'post'),
			(1, 'About', 'about', 'Body', '[]', 'published', 'page')
		`)
		require.NoError(t, err)

		repo := sqlite.NewContentRepository(db)
		postTypes, err := repo.GetPublishedCustomPostTypes(context.Background())

		require.NoError(t, err)
		assert.Len(t, postTypes, 2)
		assert.Contains(t, postTypes, "menu-item")
		assert.Contains(t, postTypes, "team-member")
	})

	t.Run("returns empty when no custom post types have published content", func(t *testing.T) {
		db := setupContentTestDB(t)
		defer teardownContentTestDB(t, db)

		_, err := db.Exec(`
			INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES
			(1, 'Blog Post', 'blog-post', 'Body', '[]', 'published', 'post'),
			(1, 'About', 'about', 'Body', '[]', 'published', 'page')
		`)
		require.NoError(t, err)

		repo := sqlite.NewContentRepository(db)
		postTypes, err := repo.GetPublishedCustomPostTypes(context.Background())

		require.NoError(t, err)
		assert.Empty(t, postTypes)
	})
}

func TestContentRepository_GetPublishedByPostType(t *testing.T) {
	t.Run("returns published content filtered by post type", func(t *testing.T) {
		db := setupContentTestDB(t)
		defer teardownContentTestDB(t, db)

		_, err := db.Exec(`
			INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES
			(1, 'Croissant', 'croissant', 'Body', '[]', 'published', 'menu-item'),
			(1, 'Eclair', 'eclair', 'Body', '[]', 'published', 'menu-item'),
			(1, 'Draft Item', 'draft-item', 'Body', '[]', 'draft', 'menu-item'),
			(1, 'Blog Post', 'blog-post', 'Body', '[]', 'published', 'post')
		`)
		require.NoError(t, err)

		repo := sqlite.NewContentRepository(db)
		items, err := repo.GetPublishedByPostType(context.Background(), "menu-item", 50, 0)

		require.NoError(t, err)
		require.Len(t, items, 2)
		assert.Equal(t, "menu-item", items[0].PostType)
		assert.Equal(t, content.StatusPublished, items[0].Status)
	})

	t.Run("returns empty when no published content for post type", func(t *testing.T) {
		db := setupContentTestDB(t)
		defer teardownContentTestDB(t, db)

		repo := sqlite.NewContentRepository(db)
		items, err := repo.GetPublishedByPostType(context.Background(), "nonexistent", 50, 0)

		require.NoError(t, err)
		assert.Empty(t, items)
	})
}
