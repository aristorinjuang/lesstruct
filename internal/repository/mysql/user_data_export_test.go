//go:build mysql

package mysql

import (
	"testing"
)

func TestUserDataExportRepository(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := NewUserDataExportRepository(rawDB)
	_ = repo
}
