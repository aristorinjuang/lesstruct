//go:build mysql

package mysql

import (
	"testing"
)

func TestCommentRepository(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := NewCommentRepository(rawDB)
	_ = repo // use in actual tests
}
