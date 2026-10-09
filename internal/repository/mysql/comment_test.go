//go:build mysql

package mysql

import (
	"testing"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
	commentstore "github.com/aristorinjuang/lesstruct/modules/comments/store"
)

func TestCommentRepository(t *testing.T) {
	dsn := mysqlDSN(t)
	db, rawDB := setupMySQLTestDB(t, dsn)
	defer db.Close()

	repo := commentstore.New(rawDB, kerneldatabase.MySQLDialect())
	_ = repo // use in actual tests
}
