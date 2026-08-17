package mysql_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/database"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	mysqlrepo "github.com/aristorinjuang/lesstruct/internal/repository/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// closeUserGetTestDB explicitly ignores the error from Close() to satisfy the
// errcheck linter for defer statements.
func closeUserGetTestDB(db *database.Database) {
	_ = db.Close()
}

func setupUserGetTestDB(t *testing.T) *database.Database {
	t.Helper()

	db, err := database.Open("sqlite", ":memory:", 0)
	require.NoError(t, err, "Failed to open test database")

	require.NoError(t, db.RunMigrations("sqlite"), "Failed to run migrations")

	return db
}

func createTimestampTestUser(t *testing.T, repo *mysqlrepo.UserRepository, username string) *repository.User {
	t.Helper()

	user := &repository.User{
		Username:     username,
		PasswordHash: "$2a$12$testHash",
		Email:        username + "@example.com",
		Role:         "Contributor",
		Status:       "verified",
	}
	require.NoError(t, repo.CreateUser(context.Background(), user))
	return user
}

func TestUserRepository_GetUserTimestamps(t *testing.T) {
	db := setupUserGetTestDB(t)
	defer closeUserGetTestDB(db)

	repo := mysqlrepo.NewUserRepository(db.DB())

	before := time.Now().Add(-time.Second)
	user := createTimestampTestUser(t, repo, "timestampuser")
	after := time.Now().Add(time.Second)

	tests := []struct {
		name    string
		input   string
		get     func(ctx context.Context, repo *mysqlrepo.UserRepository, input string) (*repository.User, error)
		wantErr bool
	}{
		{
			name:  "success - get by id returns populated timestamps",
			input: strconv.Itoa(user.ID),
			get: func(ctx context.Context, r *mysqlrepo.UserRepository, input string) (*repository.User, error) {
				id, err := strconv.Atoi(input)
				if err != nil {
					return nil, err
				}
				return r.GetUserByID(ctx, id)
			},
			wantErr: false,
		},
		{
			name:  "success - get by email returns populated timestamps",
			input: "TIMESTAMPUSER@example.com",
			get: func(ctx context.Context, r *mysqlrepo.UserRepository, input string) (*repository.User, error) {
				return r.GetUserByEmail(ctx, input)
			},
			wantErr: false,
		},
		{
			name:  "success - get by username returns populated timestamps",
			input: "timestampuser",
			get: func(ctx context.Context, r *mysqlrepo.UserRepository, input string) (*repository.User, error) {
				return r.GetUserByUsername(ctx, input)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.get(context.Background(), repo, tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, user.ID, result.ID)
			assert.False(t, result.CreatedAt.IsZero(), "CreatedAt should be populated")
			assert.False(t, result.UpdatedAt.IsZero(), "UpdatedAt should be populated")
			assert.False(t, result.CreatedAt.Before(before), "CreatedAt should not predate the insert")
			assert.False(t, result.CreatedAt.After(after), "CreatedAt should not postdate the insert")
		})
	}
}

func TestUserRepository_GetUserByID_NullTimestamps(t *testing.T) {
	db := setupUserGetTestDB(t)
	defer closeUserGetTestDB(db)

	repo := mysqlrepo.NewUserRepository(db.DB())

	result, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NULL, NULL)
	`, "nulltimeuser", "$2a$12$testHash", "nulltime@example.com", "Contributor", "verified")
	require.NoError(t, err)

	id, err := result.LastInsertId()
	require.NoError(t, err)

	tests := []struct {
		name    string
		userID  int
		wantErr bool
	}{
		{
			name:    "success - NULL timestamps stay zero",
			userID:  int(id),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.GetUserByID(context.Background(), tt.userID)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, user)
			assert.True(t, user.CreatedAt.IsZero(), "CreatedAt should stay zero when the column is NULL")
			assert.True(t, user.UpdatedAt.IsZero(), "UpdatedAt should stay zero when the column is NULL")
		})
	}
}

func TestUserRepository_GetUserByUsername_NotFound(t *testing.T) {
	db := setupUserGetTestDB(t)
	defer closeUserGetTestDB(db)

	repo := mysqlrepo.NewUserRepository(db.DB())

	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{
			name:     "success - unknown username returns nil user",
			username: "nouser",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.GetUserByUsername(context.Background(), tt.username)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Nil(t, user)
		})
	}
}

func TestUserRepository_GetAdminUserTimestamps(t *testing.T) {
	db := setupUserGetTestDB(t)
	defer closeUserGetTestDB(db)

	repo := mysqlrepo.NewUserRepository(db.DB())

	before := time.Now().Add(-time.Second)
	admin := &repository.User{
		Username:     "admin",
		PasswordHash: "$2a$12$testHash",
		Email:        "admin@example.com",
		Role:         "Admin",
		Status:       "verified",
	}
	require.NoError(t, repo.CreateUser(context.Background(), admin))
	after := time.Now().Add(time.Second)

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - admin user returns populated timestamps",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.GetAdminUser(context.Background())
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, user)
			assert.Equal(t, "admin", user.Username)
			assert.False(t, user.CreatedAt.IsZero(), "CreatedAt should be populated")
			assert.False(t, user.UpdatedAt.IsZero(), "UpdatedAt should be populated")
			assert.False(t, user.CreatedAt.Before(before), "CreatedAt should not predate the insert")
			assert.False(t, user.CreatedAt.After(after), "CreatedAt should not postdate the insert")
		})
	}
}

func TestUserRepository_GetAllUsers_NullCreatedAt(t *testing.T) {
	db := setupUserGetTestDB(t)
	defer closeUserGetTestDB(db)

	repo := mysqlrepo.NewUserRepository(db.DB())

	result, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NULL, NULL)
	`, "nulllistuser", "$2a$12$testHash", "nulllist@example.com", "Contributor", "pending")
	require.NoError(t, err)

	_, err = result.LastInsertId()
	require.NoError(t, err)

	tests := []struct {
		name    string
		get     func(ctx context.Context, repo *mysqlrepo.UserRepository) ([]*repository.User, error)
		wantErr bool
	}{
		{
			name: "success - GetAllUsers tolerates NULL created_at",
			get: func(ctx context.Context, r *mysqlrepo.UserRepository) ([]*repository.User, error) {
				return r.GetAllUsers(ctx, "", 10, 0)
			},
			wantErr: false,
		},
		{
			name: "success - GetPendingUsers tolerates NULL created_at",
			get: func(ctx context.Context, r *mysqlrepo.UserRepository) ([]*repository.User, error) {
				return r.GetPendingUsers(ctx, 10, 0)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users, err := tt.get(context.Background(), repo)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotEmpty(t, users)

			var found *repository.User
			for _, u := range users {
				if u.Username == "nulllistuser" {
					found = u
					break
				}
			}
			require.NotNil(t, found, "expected the NULL-created_at user to be returned")
			assert.True(t, found.CreatedAt.IsZero(), "CreatedAt should stay zero when the column is NULL")
		})
	}
}
