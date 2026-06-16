//go:build mysql

package mysql

import (
	"testing"
)

func TestVerificationTokenRepository(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := NewVerificationTokenRepository(rawDB)
	_ = repo
}
