package database_test

import (
	"database/sql"
	"testing"

	appdatabase "github.com/aristorinjuang/lesstruct/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// closeDB is a helper function that explicitly ignores the error from Close()
// to satisfy errcheck linter for defer statements.
func closeDB(db *appdatabase.Database) {
	_ = db.Close()
}

// closeRows is a helper function that explicitly ignores the error from Close()
// to satisfy errcheck linter for defer statements.
func closeRows(rows *sql.Rows) {
	_ = rows.Close()
}

func setupTestDB(t *testing.T) *appdatabase.Database {
	t.Helper()

	db, err := appdatabase.Open("sqlite", ":memory:", 0)
	require.NoError(t, err, "Failed to open test database")

	require.NoError(t, db.RunMigrations("sqlite"), "Failed to run migrations")

	return db
}

// TestUsersTable_HasStatusColumn verifies the status column exists on users table
func TestUsersTable_HasStatusColumn(t *testing.T) {
	db := setupTestDB(t)
	defer closeDB(db)

	rows, err := db.DB().Query("PRAGMA table_info(users)")
	require.NoError(t, err, "Failed to get table info")
	defer closeRows(rows)

	statusFound := false
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var dfltValue sql.NullString
		var pk int

		err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk)
		require.NoError(t, err, "Failed to scan row")

		if name == "status" {
			statusFound = true
			assert.Equal(t, "TEXT", dataType, "Expected status column to be TEXT")
			assert.Equal(t, 1, notNull, "Expected status column to be NOT NULL")
		}
	}

	assert.True(t, statusFound, "Status column not found in users table")

	var indexCount int
	err = db.DB().QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='index' AND name='idx_users_status'
	`).Scan(&indexCount)
	require.NoError(t, err, "Failed to query index")

	assert.Equal(t, 1, indexCount, "Expected 1 idx_users_status index")
}

// TestVerificationTokensTable verifies the verification_tokens table exists with correct columns and indexes
func TestVerificationTokensTable(t *testing.T) {
	db := setupTestDB(t)
	defer closeDB(db)

	var tableName string
	err := db.DB().QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='verification_tokens'
	`).Scan(&tableName)
	assert.NotEqual(t, sql.ErrNoRows, err, "verification_tokens table not found")
	require.NoError(t, err, "Failed to query table")

	rows, err := db.DB().Query("PRAGMA table_info(verification_tokens)")
	require.NoError(t, err, "Failed to get table info")
	defer closeRows(rows)

	expectedColumns := map[string]string{
		"id":         "INTEGER",
		"user_id":    "INTEGER",
		"token_hash": "TEXT",
		"expires_at": "DATETIME",
		"created_at": "DATETIME",
	}

	foundColumns := make(map[string]string)
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var dfltValue sql.NullString
		var pk int

		err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk)
		require.NoError(t, err, "Failed to scan row")

		foundColumns[name] = dataType
	}

	for col, expectedType := range expectedColumns {
		actualType, found := foundColumns[col]
		assert.True(t, found, "Expected column %s not found", col)
		assert.Equal(t, expectedType, actualType, "Column %s: expected type %s, got %s", col, expectedType, actualType)
	}

	indexes := []string{"idx_verification_tokens_user_id", "idx_verification_tokens_token_hash", "idx_verification_tokens_expires_at"}
	for _, indexName := range indexes {
		var indexCount int
		err = db.DB().QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='index' AND name=?
		`, indexName).Scan(&indexCount)
		require.NoError(t, err, "Failed to query index %s", indexName)

		assert.Equal(t, 1, indexCount, "Expected 1 %s index", indexName)
	}
}

// TestBlockedEmailsTable verifies the blocked_emails table exists with correct columns
func TestBlockedEmailsTable(t *testing.T) {
	db := setupTestDB(t)
	defer closeDB(db)

	var tableName string
	err := db.DB().QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='blocked_emails'
	`).Scan(&tableName)
	assert.NotEqual(t, sql.ErrNoRows, err, "blocked_emails table does not exist")
	require.NoError(t, err, "Failed to query table")

	rows, err := db.DB().Query("PRAGMA table_info(blocked_emails)")
	require.NoError(t, err, "Failed to get table info")
	defer closeRows(rows)

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk)
		require.NoError(t, err, "Failed to scan column info")
		columns[name] = true
	}

	requiredColumns := []string{"id", "email", "created_at", "reason"}
	for _, col := range requiredColumns {
		assert.True(t, columns[col], "Missing required column: %s", col)
	}

	var indexName string
	err = db.DB().QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='index' AND name='idx_blocked_emails_email'
	`).Scan(&indexName)
	assert.NotEqual(t, sql.ErrNoRows, err, "Index idx_blocked_emails_email does not exist")
	require.NoError(t, err, "Failed to query index")
}

// TestBlockedEmailsTableConstraints tests the constraints of blocked_emails table
func TestBlockedEmailsTableConstraints(t *testing.T) {
	db := setupTestDB(t)
	defer closeDB(db)

	_, err := db.DB().Exec("INSERT INTO blocked_emails (email, reason) VALUES (?, ?)", "test@example.com", "spam")
	require.NoError(t, err, "Failed to insert first blocked email")

	_, err = db.DB().Exec("INSERT INTO blocked_emails (email, reason) VALUES (?, ?)", "test@example.com", "spam")
	assert.Error(t, err, "Expected error when inserting duplicate email")

	_, err = db.DB().Exec("INSERT INTO blocked_emails (reason) VALUES (?)", "test")
	assert.Error(t, err, "Expected error when inserting NULL email")

	var createdAt string
	err = db.DB().QueryRow("SELECT created_at FROM blocked_emails WHERE email = ?", "test@example.com").Scan(&createdAt)
	require.NoError(t, err, "Failed to query created_at")
	assert.NotEmpty(t, createdAt, "Expected created_at to have default value")
}

// TestSoftDeletedContentTable verifies the soft_deleted_content table exists with correct columns and indexes
func TestSoftDeletedContentTable(t *testing.T) {
	db := setupTestDB(t)
	defer closeDB(db)

	var tableName string
	err := db.DB().QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='soft_deleted_content'
	`).Scan(&tableName)
	assert.NotEqual(t, sql.ErrNoRows, err, "soft_deleted_content table does not exist")
	require.NoError(t, err, "Failed to query table")

	rows, err := db.DB().Query("PRAGMA table_info(soft_deleted_content)")
	require.NoError(t, err, "Failed to get table info")
	defer closeRows(rows)

	columns := make(map[string]string)
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk)
		require.NoError(t, err, "Failed to scan column info")
		columns[name] = dataType
	}

	requiredColumns := []string{"id", "content_type", "content_id", "user_id", "deleted_at", "deleted_by", "reason", "is_permanent"}
	for _, col := range requiredColumns {
		_, found := columns[col]
		assert.True(t, found, "Missing required column: %s", col)
	}

	indexes := []string{"idx_soft_deleted_content_lookup", "idx_soft_deleted_content_user"}
	for _, indexName := range indexes {
		var indexCount int
		err = db.DB().QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='index' AND name=?
		`, indexName).Scan(&indexCount)
		require.NoError(t, err, "Failed to query index %s", indexName)

		assert.Equal(t, 1, indexCount, "Expected 1 %s index", indexName)
	}
}

// TestUsersTable_StatusValues verifies the status column accepts valid enum values
func TestUsersTable_StatusValues(t *testing.T) {
	db := setupTestDB(t)
	defer closeDB(db)

	validStatuses := []string{"pending", "verified", "suspended", "soft_deleted"}
	for _, status := range validStatuses {
		_, err := db.DB().Exec(`
			INSERT INTO users (username, password_hash, email, role, status)
			VALUES (?, ?, ?, ?, ?)
		`, "testuser_"+status, "hash", "test_"+status+"@example.com", "Author", status)
		assert.NoError(t, err, "Failed to insert user with status %s", status)
	}
}

// TestRunMigrations_Idempotent verifies that calling RunMigrations twice succeeds
func TestRunMigrations_Idempotent(t *testing.T) {
	db, err := appdatabase.Open("sqlite", ":memory:", 0)
	require.NoError(t, err, "Failed to open test database")
	defer closeDB(db)

	require.NoError(t, db.RunMigrations("sqlite"), "First RunMigrations failed")
	require.NoError(t, db.RunMigrations("sqlite"), "Second RunMigrations failed")
}

// TestRunMigrations_ErrorsOnOldSchemaFormat verifies that the old schema_migrations
// table format is detected and reported
func TestRunMigrations_ErrorsOnOldSchemaFormat(t *testing.T) {
	db, err := appdatabase.Open("sqlite", ":memory:", 0)
	require.NoError(t, err, "Failed to open test database")
	defer closeDB(db)

	_, err = db.DB().Exec(`
		CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO schema_migrations (version) VALUES (1);
	`)
	require.NoError(t, err, "Failed to create old schema_migrations")

	err = db.RunMigrations("sqlite")
	assert.Error(t, err, "Expected error for old schema_migrations format")
}
