package modules_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/database"
	"github.com/aristorinjuang/lesstruct/internal/modules"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupProfilePictureAdapterTestDB(t *testing.T) *database.Database {
	t.Helper()
	db, err := database.Open("sqlite", ":memory:", 0)
	require.NoError(t, err, "Failed to open test database")
	require.NoError(t, db.RunMigrations("sqlite"), "Failed to run migrations")
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestProfilePictureUserRepo(t *testing.T) {
	tests := []struct {
		name    string
		run     func(t *testing.T, adapter *modules.ProfilePictureUserRepo, userID int)
		wantErr bool
	}{
		{
			name: "success - round trip picture filename",
			run: func(t *testing.T, adapter *modules.ProfilePictureUserRepo, userID int) {
				require.NoError(t, adapter.UpdateProfilePicture(context.Background(), userID, "pic_20260101.webp"))
				filename, err := adapter.GetUserByID(context.Background(), userID)
				require.NoError(t, err)
				assert.Equal(t, "pic_20260101.webp", filename)
				require.NoError(t, adapter.DeleteProfilePicture(context.Background(), userID))
				cleared, err := adapter.GetUserByID(context.Background(), userID)
				require.NoError(t, err)
				assert.Empty(t, cleared)
			},
			wantErr: false,
		},
		{
			name: "error - unknown user id",
			run: func(t *testing.T, adapter *modules.ProfilePictureUserRepo, _ int) {
				_, err := adapter.GetUserByID(context.Background(), 9999)
				require.Error(t, err)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupProfilePictureAdapterTestDB(t)
			repo := repository.NewUserRepository(db.DB())
			adapter := modules.NewProfilePictureUserRepo(repo)

			result, err := db.DB().Exec(
				`INSERT INTO users (username, password_hash, email, role, status) VALUES (?, ?, ?, ?, ?)`,
				"picuser", "$2a$12$testHash", "picuser@example.com", "viewer", "verified",
			)
			require.NoError(t, err)
			id, err := result.LastInsertId()
			require.NoError(t, err)

			tt.run(t, adapter, int(id))
		})
	}
}
