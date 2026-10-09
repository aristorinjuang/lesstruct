package postgresql_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/auth"
	contentdomain "github.com/aristorinjuang/lesstruct/kernel/content"
	postgresqlrepo "github.com/aristorinjuang/lesstruct/internal/repository/postgresql"
	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
	commentstore "github.com/aristorinjuang/lesstruct/modules/comments/store"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	mediastore "github.com/aristorinjuang/lesstruct/modules/media/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentRepo_PostgreSQL_CreateAndGet(t *testing.T) {
	dsn := postgresDSN(t)
	db, rawDB := setupPostgresTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	userRepo := postgresqlrepo.NewUserRepository(rawDB)
	userID := seedPostgresUser(t, rawDB, userRepo, "contentauthor", "Author")

	repo := postgresqlrepo.NewContentRepository(rawDB)
	content := &contentdomain.Content{
		UserID:        userID,
		Title:         "Test Post",
		Slug:          "test-post",
		Content:       "<p>Hello World</p>",
		Status:        contentdomain.StatusDraft,
		PostType:      "post",
		AllowComments: true,
		Language:      "en",
		Tags:          []string{"test"},
	}

	err := repo.Create(context.Background(), content)
	require.NoError(t, err)
	assert.NotZero(t, content.ID)

	found, err := repo.GetBySlug(context.Background(), "test-post", "en")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "Test Post", found.Title)
}

func TestDashboardRepo_PostgreSQL(t *testing.T) {
	dsn := postgresDSN(t)
	db, rawDB := setupPostgresTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	userRepo := postgresqlrepo.NewUserRepository(rawDB)
	userID := seedPostgresUser(t, rawDB, userRepo, "dashboarduser", "Author")

	repo := postgresqlrepo.NewDashboardRepository(rawDB)
	stats, err := repo.GetStats(context.Background(), userID)
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.NotNil(t, stats.RecentContent)
}

func TestCommentRepo_PostgreSQL(t *testing.T) {
	dsn := postgresDSN(t)
	db, rawDB := setupPostgresTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	userRepo := postgresqlrepo.NewUserRepository(rawDB)
	userID := seedPostgresUser(t, rawDB, userRepo, "commentauthor", "Author")

	repo := commentstore.New(rawDB, kerneldatabase.PostgresDialect())
	comment := &commentdomain.Comment{
		ContentID: 1,
		UserID:    userID,
		Comment:   "Test comment",
		Status:    commentdomain.CommentStatusPending,
	}

	err := repo.Create(context.Background(), comment)
	require.NoError(t, err)
	assert.NotZero(t, comment.ID)
}

func TestMediaRepo_PostgreSQL(t *testing.T) {
	dsn := postgresDSN(t)
	db, rawDB := setupPostgresTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	userRepo := postgresqlrepo.NewUserRepository(rawDB)
	userID := seedPostgresUser(t, rawDB, userRepo, "mediaauthor", "Author")

	repo := mediastore.New(rawDB, kerneldatabase.PostgresDialect())
	media := &mediadomain.Media{
		UserID:           userID,
		Filename:         "test.webp",
		OriginalFilename: "test.jpg",
		MimeType:         "image/webp",
		FileSize:         1024,
		Width:            100,
		Height:           100,
		AltText:          "Test image",
		IsWebP:           true,
		FilePath:         "data/uploads/media/test.webp",
		URL:              "/media/test.webp",
		Hash:             "abc123test",
	}

	err := repo.Create(context.Background(), media)
	require.NoError(t, err)
	assert.NotZero(t, media.ID)

	found, err := repo.FindByHash(context.Background(), "abc123test")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "test.webp", found.Filename)
}

func seedPostgresUser(t *testing.T, rawDB *sql.DB, userRepo *postgresqlrepo.UserRepository, username, role string) int {
	t.Helper()
	passwordHash, err := auth.HashPassword("testpassword123")
	require.NoError(t, err, "Failed to hash password")

	user := &postgresqlrepo.User{
		Username:     username,
		PasswordHash: passwordHash,
		Email:        username + "@example.com",
		Role:         role,
		Status:       "verified",
	}
	err = userRepo.CreateUser(context.Background(), user)
	require.NoError(t, err, "Failed to seed user")
	return user.ID
}
