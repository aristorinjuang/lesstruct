//go:build mysql

package mysql

import (
	"testing"
)

func TestSoftDeleteRepository(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := NewSoftDeleteRepository(rawDB)
	_ = repo
}
