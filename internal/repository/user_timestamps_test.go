package repository_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestUserRepository_GetAdminUserTimestamps(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	before := time.Now().Add(-time.Second)
	_, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, status)
		VALUES (?, ?, ?, ?, ?)
	`, "admin", "$2a$12$testHash", "admin@example.com", "Admin", "verified")
	require.NoError(t, err, "Failed to create admin user")
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

func TestUserRepository_GetUserByEmailTimestamps(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	before := time.Now().Add(-time.Second)
	user := &repository.User{
		Username:     "timestampuser",
		PasswordHash: "$2a$12$testHash",
		Email:        "timestamp@example.com",
		Role:         "Contributor",
		Status:       "verified",
	}
	require.NoError(t, repo.CreateUser(context.Background(), user))
	after := time.Now().Add(time.Second)

	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{
			name:    "success - get by email returns populated timestamps",
			email:   "TIMESTAMP@example.com",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetUserByEmail(context.Background(), tt.email)
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

func TestUserRepository_GetUserByUsernameTimestamps(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	before := time.Now().Add(-time.Second)
	user := &repository.User{
		Username:     "timestampuser2",
		PasswordHash: "$2a$12$testHash",
		Email:        "timestamp2@example.com",
		Role:         "Contributor",
		Status:       "verified",
	}
	require.NoError(t, repo.CreateUser(context.Background(), user))
	after := time.Now().Add(time.Second)

	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{
			name:     "success - get by username returns populated timestamps",
			username: "TIMESTAMPUSER2",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetUserByUsername(context.Background(), tt.username)
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

func TestUserRepository_GetUserByIDNullCreatedAt(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	result, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NULL, NULL)
	`, "nulltimeuser", "$2a$12$testHash", "nulltime@example.com", "Contributor", "verified")
	require.NoError(t, err)

	id, err := result.LastInsertId()
	require.NoError(t, err)

	tests := []struct {
		name    string
		userID  string
		wantErr bool
	}{
		{
			name:    "success - NULL timestamps stay zero",
			userID:  strconv.FormatInt(id, 10),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedID, err := strconv.Atoi(tt.userID)
			require.NoError(t, err)
			user, err := repo.GetUserByID(context.Background(), parsedID)
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

func TestUserRepository_GetAllUsersNullCreatedAt(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	_, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NULL, NULL)
	`, "nulllistuser", "$2a$12$testHash", "nulllist@example.com", "Contributor", "pending")
	require.NoError(t, err)

	tests := []struct {
		name    string
		get     func(ctx context.Context, r *repository.UserRepository) ([]*repository.User, error)
		wantErr bool
	}{
		{
			name: "success - GetAllUsers tolerates NULL created_at",
			get: func(ctx context.Context, r *repository.UserRepository) ([]*repository.User, error) {
				return r.GetAllUsers(ctx, "", 10, 0)
			},
			wantErr: false,
		},
		{
			name: "success - GetPendingUsers tolerates NULL created_at",
			get: func(ctx context.Context, r *repository.UserRepository) ([]*repository.User, error) {
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
