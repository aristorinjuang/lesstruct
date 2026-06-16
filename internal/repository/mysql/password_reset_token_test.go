//go:build mysql

package mysql

import (
	"testing"
)

func TestPasswordResetTokenRepository(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := NewPasswordResetTokenRepository(rawDB)
	_ = repo
}
