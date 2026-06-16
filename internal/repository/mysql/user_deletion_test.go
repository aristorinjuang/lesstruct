//go:build mysql

package mysql

import (
	"testing"
)

func TestUserDeletionRepository(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := NewUserDeletionRepository(rawDB)
	_ = repo
}
