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

func insertCustomFieldTestData(t *testing.T, db *sql.DB) {
	_, err := db.Exec(`
		INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type, custom_fields)
		VALUES
			(1, 'Croissant', 'croissant', 'Content', '[]', 'draft', 'menu-item', '{"category":"Pastry","price":4.5}'),
			(1, 'Baguette', 'baguette', 'Content', '[]', 'draft', 'menu-item', '{"category":"Bread","price":3.0}'),
			(1, 'Eclair', 'eclair', 'Content', '[]', 'draft', 'menu-item', '{"category":"Pastry","price":5.0}'),
			(1, 'No Fields Post', 'no-fields', 'Content', '[]', 'draft', 'post', NULL),
			(1, 'Null CF', 'null-cf', 'Content', '[]', 'draft', 'post', '{}'),
			(2, 'Other User', 'other-user-item', 'Content', '[]', 'draft', 'menu-item', '{"category":"Pastry","price":6.0}')
	`)
	require.NoError(t, err, "failed to insert test data")
}

func TestContentRepository_ListByFilters_ExactMatch(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer func() { assert.NoError(t, db.Close()) }()
	insertCustomFieldTestData(t, db)

	repo := sqlite.NewContentRepository(db)
	filters := content.ContentFilters{
		Limit:  100,
		Offset: 0,
		CustomFieldFilters: []content.CustomFieldFilter{
			{Field: "category", Operator: content.FilterOpEqual, Value: "Pastry"},
		},
	}

	results, err := repo.ListByFilters(context.Background(), 1, filters)
	require.NoError(t, err)
	require.Len(t, results, 2)

	titles := map[string]bool{}
	for _, c := range results {
		titles[c.Title] = true
	}
	assert.True(t, titles["Croissant"])
	assert.True(t, titles["Eclair"])
}

func TestContentRepository_ListByFilters_NumberRange(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer func() { assert.NoError(t, db.Close()) }()
	insertCustomFieldTestData(t, db)

	repo := sqlite.NewContentRepository(db)

	t.Run("min and max", func(t *testing.T) {
		filters := content.ContentFilters{
			Limit:  100,
			Offset: 0,
			CustomFieldFilters: []content.CustomFieldFilter{
				{Field: "price", Operator: content.FilterOpMin, Value: "4.0"},
				{Field: "price", Operator: content.FilterOpMax, Value: "5.0"},
			},
		}

		results, err := repo.ListByFilters(context.Background(), 1, filters)
		require.NoError(t, err)
		require.Len(t, results, 2)

		titles := map[string]bool{}
		for _, c := range results {
			titles[c.Title] = true
		}
		assert.True(t, titles["Croissant"])
		assert.True(t, titles["Eclair"])
	})

	t.Run("min only", func(t *testing.T) {
		filters := content.ContentFilters{
			Limit:  100,
			Offset: 0,
			CustomFieldFilters: []content.CustomFieldFilter{
				{Field: "price", Operator: content.FilterOpMin, Value: "4.5"},
			},
		}

		results, err := repo.ListByFilters(context.Background(), 1, filters)
		require.NoError(t, err)
		require.Len(t, results, 2)

		titles := map[string]bool{}
		for _, c := range results {
			titles[c.Title] = true
		}
		assert.True(t, titles["Croissant"])
		assert.True(t, titles["Eclair"])
	})

	t.Run("max only", func(t *testing.T) {
		filters := content.ContentFilters{
			Limit:  100,
			Offset: 0,
			CustomFieldFilters: []content.CustomFieldFilter{
				{Field: "price", Operator: content.FilterOpMax, Value: "3.5"},
			},
		}

		results, err := repo.ListByFilters(context.Background(), 1, filters)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "Baguette", results[0].Title)
	})
}

func TestContentRepository_ListByFilters_PostTypeAndCustomField(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer func() { assert.NoError(t, db.Close()) }()
	insertCustomFieldTestData(t, db)

	repo := sqlite.NewContentRepository(db)
	filters := content.ContentFilters{
		Limit:    100,
		Offset:   0,
		PostType: "menu-item",
		CustomFieldFilters: []content.CustomFieldFilter{
			{Field: "category", Operator: content.FilterOpEqual, Value: "Pastry"},
		},
	}

	results, err := repo.ListByFilters(context.Background(), 1, filters)
	require.NoError(t, err)
	require.Len(t, results, 2)
}

func TestContentRepository_ListByFilters_NoFilters(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer func() { assert.NoError(t, db.Close()) }()
	insertCustomFieldTestData(t, db)

	repo := sqlite.NewContentRepository(db)
	filters := content.ContentFilters{
		Limit:  100,
		Offset: 0,
	}

	results, err := repo.ListByFilters(context.Background(), 1, filters)
	require.NoError(t, err)
	assert.Len(t, results, 5, "should return all content for user 1")
}

func TestContentRepository_ListByFilters_ExcludesNullCustomFields(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer func() { assert.NoError(t, db.Close()) }()
	insertCustomFieldTestData(t, db)

	repo := sqlite.NewContentRepository(db)
	filters := content.ContentFilters{
		Limit:  100,
		Offset: 0,
		CustomFieldFilters: []content.CustomFieldFilter{
			{Field: "category", Operator: content.FilterOpEqual, Value: "Pastry"},
		},
	}

	results, err := repo.ListByFilters(context.Background(), 1, filters)
	require.NoError(t, err)
	for _, c := range results {
		assert.NotNil(t, c.CustomFields, "content with null custom fields should be excluded")
		assert.Equal(t, "Pastry", c.CustomFields["category"])
	}
}

func TestContentRepository_ListByFilters_MultipleFilters(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer func() { assert.NoError(t, db.Close()) }()
	insertCustomFieldTestData(t, db)

	repo := sqlite.NewContentRepository(db)
	filters := content.ContentFilters{
		Limit:  100,
		Offset: 0,
		CustomFieldFilters: []content.CustomFieldFilter{
			{Field: "category", Operator: content.FilterOpEqual, Value: "Pastry"},
			{Field: "price", Operator: content.FilterOpMin, Value: "4.6"},
		},
	}

	results, err := repo.ListByFilters(context.Background(), 1, filters)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Eclair", results[0].Title)
}

func TestContentRepository_ListByFilters_ScopedToUser(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer func() { assert.NoError(t, db.Close()) }()
	insertCustomFieldTestData(t, db)

	repo := sqlite.NewContentRepository(db)
	filters := content.ContentFilters{
		Limit:  100,
		Offset: 0,
		CustomFieldFilters: []content.CustomFieldFilter{
			{Field: "category", Operator: content.FilterOpEqual, Value: "Pastry"},
		},
	}

	results, err := repo.ListByFilters(context.Background(), 2, filters)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Other User", results[0].Title)
}

func TestContentRepository_ListByFilters_LimitOffset(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer func() { assert.NoError(t, db.Close()) }()
	insertCustomFieldTestData(t, db)

	repo := sqlite.NewContentRepository(db)
	filters := content.ContentFilters{
		Limit:  2,
		Offset: 0,
	}

	results, err := repo.ListByFilters(context.Background(), 1, filters)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}
