package repository_test

import (
	"context"
	"testing"

	appdatabase "github.com/aristorinjuang/lesstruct/internal/database"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/kernel"
	commentmodule "github.com/aristorinjuang/lesstruct/modules/comments"
	mediamodule "github.com/aristorinjuang/lesstruct/modules/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserDataExportRepository_ModuleSchemas proves the GDPR export reads the
// real module-owned comments and media schemas. Stale column names here made the
// endpoint answer 500 on every call while the privacy policy promised data
// portability.
func TestUserDataExportRepository_ModuleSchemas(t *testing.T) {
	core, err := appdatabase.Open("sqlite", ":memory:", 0)
	require.NoError(t, err, "failed to open test database")
	t.Cleanup(func() { _ = core.Close() })
	require.NoError(t, core.RunMigrations("sqlite"), "failed to run core migrations")

	registry := kernel.NewRegistry()
	require.NoError(t, registry.Register(mediamodule.New()))
	require.NoError(t, registry.Register(commentmodule.New()))
	require.NoError(t, appdatabase.MigrateModules(core.DB(), "sqlite", registry), "failed to migrate modules")

	db := core.DB()
	_, err = db.Exec(`
		INSERT INTO users (id, username, password_hash, email, role, status)
		VALUES (1, 'alice', 'hash', 'alice@example.com', 'Commentator', 'verified');
		INSERT INTO content_items (id, user_id, title, slug, content, status)
		VALUES (10, 1, 'Post', 'post', 'body', 'published');
		INSERT INTO comments (id, content_id, user_id, comment)
		VALUES (20, 10, 1, 'Nice post');
		INSERT INTO media_files (id, user_id, filename, original_filename, mime_type, file_size, file_path, url, hash)
		VALUES (30, 1, 'x.webp', 'photo.jpg', 'image/webp', 1234, 'data/uploads/media/x.webp', '/uploads/media/x.webp', 'abc');
	`)
	require.NoError(t, err, "failed to seed export fixtures")

	repo := repository.NewUserDataExportRepository(db)
	export, err := repo.GetUserDataForExport(context.Background(), 1)

	require.NoError(t, err, "export must succeed against the real module schemas")
	require.NotNil(t, export)
	assert.Equal(t, "alice", export.User.Username)
	require.Len(t, export.Content, 1)
	require.Len(t, export.Comments, 1)
	assert.Equal(t, "Nice post", export.Comments[0].Content)
	require.Len(t, export.Media, 1)
	assert.Equal(t, "photo.jpg", export.Media[0].OriginalFilename)
}
