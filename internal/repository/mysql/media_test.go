//go:build mysql

package mysql

import (
	"testing"
)

func TestMediaRepository(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := NewMediaRepository(rawDB)
	_ = repo // use in actual tests
}
