//go:build mysql

package mysql

import (
	"testing"
)

func TestContentRepository(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := NewContentRepository(rawDB)
	_ = repo // use in actual tests
}
