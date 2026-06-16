//go:build mysql

package mysql

import (
	"testing"
)

func TestBlockedEmailRepository(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := NewBlockedEmailRepository(rawDB)
	_ = repo
}
