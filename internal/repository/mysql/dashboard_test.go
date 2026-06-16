//go:build mysql

package mysql

import (
	"testing"
)

func TestDashboardRepository(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := NewDashboardRepository(rawDB)
	_ = repo // use in actual tests
}
