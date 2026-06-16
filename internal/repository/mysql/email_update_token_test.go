//go:build mysql

package mysql

import (
	"testing"
)

func TestEmailUpdateTokenRepository(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := NewEmailUpdateTokenRepository(rawDB)
	_ = repo
}
