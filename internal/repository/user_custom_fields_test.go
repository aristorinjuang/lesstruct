package repository_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/database"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCustomFieldsTestDB(t *testing.T) *database.Database {
	t.Helper()
	db, err := database.Open("sqlite", ":memory:", 0)
	require.NoError(t, err, "Failed to open test database")
	require.NoError(t, db.RunMigrations("sqlite"), "Failed to run migrations")
	return db
}

func seedUser(t *testing.T, db *database.Database, username, role, status string) int {
	t.Helper()
	passwordHash := "$2a$12$testHash"
	result, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, status)
		VALUES (?, ?, ?, ?, ?)
	`, username, passwordHash, username+"@example.com", role, status)
	require.NoError(t, err, "Failed to seed user")
	id, err := result.LastInsertId()
	require.NoError(t, err, "Failed to get last insert ID")
	return int(id)
}

func TestUserCustomFields_UpdateProfileWithCustomFields(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer closeUserTestDB(db)
	repo := repository.NewUserRepository(db.DB())

	userID := seedUser(t, db, "testuser", "Author", "verified")

	customFields := map[string]any{
		"job_title": "Software Engineer",
		"company":   "Acme Corp",
	}

	err := repo.UpdateProfile(context.Background(), userID, "testuser", "testuser@example.com", "Author", customFields)
	require.NoError(t, err)

	user, err := repo.GetUserByID(context.Background(), userID)
	require.NoError(t, err)
	assert.Equal(t, "Software Engineer", user.CustomFields["job_title"])
	assert.Equal(t, "Acme Corp", user.CustomFields["company"])
}

func TestUserCustomFields_UpdateProfileNilCustomFields(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer closeUserTestDB(db)
	repo := repository.NewUserRepository(db.DB())

	userID := seedUser(t, db, "testuser2", "Author", "verified")

	err := repo.UpdateProfile(context.Background(), userID, "testuser2", "testuser2@example.com", "Author", nil)
	require.NoError(t, err)

	user, err := repo.GetUserByID(context.Background(), userID)
	require.NoError(t, err)
	assert.Nil(t, user.CustomFields)
}

func TestUserCustomFields_UpdateProfileOverwriteCustomFields(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer closeUserTestDB(db)
	repo := repository.NewUserRepository(db.DB())

	userID := seedUser(t, db, "testuser3", "Author", "verified")

	initialFields := map[string]any{"role": "gold"}
	err := repo.UpdateProfile(context.Background(), userID, "testuser3", "testuser3@example.com", "Author", initialFields)
	require.NoError(t, err)

	updatedFields := map[string]any{"job_title": "CTO", "location": "NYC"}
	err = repo.UpdateProfile(context.Background(), userID, "testuser3", "testuser3@example.com", "Author", updatedFields)
	require.NoError(t, err)

	user, err := repo.GetUserByID(context.Background(), userID)
	require.NoError(t, err)
	assert.Nil(t, user.CustomFields["role"])
	assert.Equal(t, "CTO", user.CustomFields["job_title"])
	assert.Equal(t, "NYC", user.CustomFields["location"])
}

func TestUserCustomFields_NilCustomFieldsPreservesExisting(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer closeUserTestDB(db)
	repo := repository.NewUserRepository(db.DB())

	userID := seedUser(t, db, "testuser_preserve", "Author", "verified")

	fields := map[string]any{"job_title": "Engineer", "company": "Acme"}
	err := repo.UpdateProfile(context.Background(), userID, "testuser_preserve", "testuser_preserve@example.com", "Author", fields)
	require.NoError(t, err)

	err = repo.UpdateProfile(context.Background(), userID, "New Name", "testuser_preserve@example.com", "Author", nil)
	require.NoError(t, err)

	user, err := repo.GetUserByID(context.Background(), userID)
	require.NoError(t, err)
	require.NotNil(t, user.CustomFields, "custom fields should be preserved when nil is passed")
	assert.Equal(t, "Engineer", user.CustomFields["job_title"])
	assert.Equal(t, "Acme", user.CustomFields["company"])
	assert.Equal(t, "New Name", user.Name)
}

func TestUserCustomFields_GetUserByIDWithCustomFields(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer closeUserTestDB(db)
	repo := repository.NewUserRepository(db.DB())

	userID := seedUser(t, db, "testuser4", "Author", "verified")

	customFields := map[string]any{"internal_rating": "gold"}
	err := repo.UpdateProfile(context.Background(), userID, "testuser4", "testuser4@example.com", "Author", customFields)
	require.NoError(t, err)

	user, err := repo.GetUserByID(context.Background(), userID)
	require.NoError(t, err)
	require.NotNil(t, user.CustomFields)
	assert.Equal(t, "gold", user.CustomFields["internal_rating"])
}

func TestUserCustomFields_GetUserByIDWithoutCustomFields(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer closeUserTestDB(db)
	repo := repository.NewUserRepository(db.DB())

	userID := seedUser(t, db, "testuser5", "Author", "verified")

	user, err := repo.GetUserByID(context.Background(), userID)
	require.NoError(t, err)
	assert.Nil(t, user.CustomFields)
}

func TestUserCustomFields_GetUserByEmailWithCustomFields(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer closeUserTestDB(db)
	repo := repository.NewUserRepository(db.DB())

	userID := seedUser(t, db, "testuser6", "Author", "verified")

	fields := map[string]any{"website": "https://example.com"}
	_ = repo.UpdateProfile(context.Background(), userID, "testuser6", "testuser6@example.com", "Author", fields)

	user, err := repo.GetUserByEmail(context.Background(), "testuser6@example.com")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "https://example.com", user.CustomFields["website"])
}

func TestUserCustomFields_GetUserByUsernameWithCustomFields(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer closeUserTestDB(db)
	repo := repository.NewUserRepository(db.DB())

	userID := seedUser(t, db, "testuser7", "Author", "verified")

	fields := map[string]any{"bio": "Hello world"}
	_ = repo.UpdateProfile(context.Background(), userID, "testuser7", "testuser7@example.com", "Author", fields)

	user, err := repo.GetUserByUsername(context.Background(), "testuser7")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "Hello world", user.CustomFields["bio"])
}

func TestUserCustomFields_GetAllUsersWithCustomFields(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer closeUserTestDB(db)
	repo := repository.NewUserRepository(db.DB())

	userID := seedUser(t, db, "testuser8", "Author", "verified")

	fields := map[string]any{"department": "Engineering"}
	_ = repo.UpdateProfile(context.Background(), userID, "testuser8", "testuser8@example.com", "Author", fields)

	users, err := repo.GetAllUsers(context.Background(), "", 10, 0)
	require.NoError(t, err)
	require.NotEmpty(t, users)

	var found bool
	for _, u := range users {
		if u.ID == userID {
			found = true
			assert.Equal(t, "Engineering", u.CustomFields["department"])
			break
		}
	}
	assert.True(t, found, "User with custom fields not found in GetAllUsers")
}

func TestUserCustomFields_GetPendingUsersNoCustomFields(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer closeUserTestDB(db)
	repo := repository.NewUserRepository(db.DB())

	passwordHash := "$2a$12$testHash"
	_, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, status)
		VALUES (?, ?, ?, ?, 'pending')
	`, "pendinguser", passwordHash, "pending@example.com", "Author")
	require.NoError(t, err)

	users, err := repo.GetPendingUsers(context.Background(), 10, 0)
	require.NoError(t, err)
	require.NotEmpty(t, users)
	assert.Nil(t, users[0].CustomFields)
}

func TestUserCustomFields_JSONRoundtrip(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer closeUserTestDB(db)
	repo := repository.NewUserRepository(db.DB())

	userID := seedUser(t, db, "testuser9", "Author", "verified")

	complexFields := map[string]any{
		"job_title":  "Engineer",
		"experience": 5,
		"remote":     true,
		"skills":     []any{"Go", "Vue"},
		"address": map[string]any{
			"city":    "San Francisco",
			"country": "US",
		},
	}
	err := repo.UpdateProfile(context.Background(), userID, "testuser9", "testuser9@example.com", "Author", complexFields)
	require.NoError(t, err)

	user, err := repo.GetUserByID(context.Background(), userID)
	require.NoError(t, err)
	require.NotNil(t, user.CustomFields)
	assert.Equal(t, "Engineer", user.CustomFields["job_title"])
	assert.Equal(t, 5.0, user.CustomFields["experience"])
	assert.Equal(t, true, user.CustomFields["remote"])
	assert.Equal(t, "San Francisco", user.CustomFields["address"].(map[string]any)["city"])
}

func TestUserCustomFields_OmitemptyInJSON(t *testing.T) {
	userWithout := repository.User{ID: 1, Username: "test", Role: "Author"}
	data, err := json.Marshal(userWithout)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "customFields")

	userWith := repository.User{ID: 1, Username: "test", Role: "Author", CustomFields: map[string]any{"key": "val"}}
	data, err = json.Marshal(userWith)
	require.NoError(t, err)
	assert.Contains(t, string(data), "customFields")
	assert.Contains(t, string(data), "key")
}

func TestUserCustomFields_MigrationApplied(t *testing.T) {
	db := setupCustomFieldsTestDB(t)
	defer closeUserTestDB(db)

	var columnName string
	err := db.DB().QueryRow(`
		SELECT name FROM pragma_table_info('users') WHERE name = 'custom_fields'
	`).Scan(&columnName)
	require.NoError(t, err, "custom_fields column should exist after migration")
	assert.Equal(t, "custom_fields", columnName)
}
