//go:build mysql

package mysql

import (
	"testing"
)

func TestFailedLoginAttemptRepository(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := NewFailedLoginAttemptRepository(rawDB)
	_ = repo
}
