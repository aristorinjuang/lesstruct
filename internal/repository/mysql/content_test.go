//go:build mysql

package mysql

import (
	"context"
	"testing"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentRepository(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := NewContentRepository(rawDB)
	_ = repo // use in actual tests
}

func TestContentRepository_RapidFireUpdate_NoOp(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := NewContentRepository(rawDB)
	ctx := context.Background()

	_, err := rawDB.Exec(`
		INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type, language)
		VALUES (1, 'Rapid Test', 'rapid-test', 'body', '[]', 'draft', 'post', 'en')
	`)
	require.NoError(t, err, "failed to insert test content")

	// First update: change the title
	first := &contentdomain.Content{
		ID:       1,
		UserID:   1,
		Title:    "Updated Title",
		Slug:     "rapid-test",
		Content:  "body",
		Tags:     []string{},
		Status:   contentdomain.StatusDraft,
		PostType: "post",
		Language: "en",
	}
	err = repo.Update(ctx, first)
	require.NoError(t, err, "first update should succeed")

	// Second update: identical data (no-op). With clientFoundRows=true this should
	// still succeed; without it, RowsAffected() returns 0 and we get
	// ErrContentNotFound.
	second := &contentdomain.Content{
		ID:       1,
		UserID:   1,
		Title:    "Updated Title",
		Slug:     "rapid-test",
		Content:  "body",
		Tags:     []string{},
		Status:   contentdomain.StatusDraft,
		PostType: "post",
		Language: "en",
	}
	err = repo.Update(ctx, second)
	assert.NoError(t, err, "no-op update should succeed with clientFoundRows=true")
}
