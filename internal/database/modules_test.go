package database_test

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"

	appdatabase "github.com/aristorinjuang/lesstruct/internal/database"
	"github.com/aristorinjuang/lesstruct/kernel"
	commentmodule "github.com/aristorinjuang/lesstruct/modules/comments"
	mediamodule "github.com/aristorinjuang/lesstruct/modules/media"
	ordersmodule "github.com/aristorinjuang/lesstruct/modules/orders"
	paymentsmodule "github.com/aristorinjuang/lesstruct/modules/payments"
	seomodule "github.com/aristorinjuang/lesstruct/modules/seo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubMigrator struct {
	name string
	fsys fstest.MapFS
}

func (m *stubMigrator) Name() string {
	return m.name
}

func (m *stubMigrator) Version() string {
	return "test"
}

func (m *stubMigrator) Requires() []string {
	return nil
}

func (m *stubMigrator) Optional() []string {
	return nil
}

func (m *stubMigrator) Init(_ kernel.Kernel) error {
	return nil
}

func (m *stubMigrator) Shutdown() error {
	return nil
}

func (m *stubMigrator) Migrations() fs.FS {
	return m.fsys
}

func newStubMigrator(name, sql string) *stubMigrator {
	return &stubMigrator{
		name: name,
		fsys: fstest.MapFS{
			"sqlite/000001_test.up.sql":   {Data: []byte(sql)},
			"sqlite/000001_test.down.sql": {Data: []byte("DROP TABLE IF EXISTS widget;")},
		},
	}
}

func openMemoryDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var name string
	err := db.QueryRowContext(
		context.Background(),
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`,
		table,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	require.NoError(t, err)
	return true
}

func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := db.QueryContext(
		context.Background(),
		`SELECT name FROM pragma_table_info(?)`,
		table,
	)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		if name == column {
			require.NoError(t, rows.Close())
			return true
		}
	}
	require.NoError(t, rows.Err())
	return false
}

func TestMigrateModules(t *testing.T) {
	tests := []struct {
		name       string
		modules    []kernel.Module
		wantTables []string
		wantErr    bool
	}{
		{
			name: "success - migrates into per-module table",
			modules: []kernel.Module{
				newStubMigrator("widget", `CREATE TABLE IF NOT EXISTS widget (id INTEGER PRIMARY KEY);`),
			},
			wantTables: []string{"widget", "schema_migrations_widget"},
			wantErr:    false,
		},
		{
			name:       "success - no migrators is a no-op",
			modules:    nil,
			wantTables: nil,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openMemoryDB(t)
			registry := kernel.NewRegistry()
			for _, module := range tt.modules {
				require.NoError(t, registry.Register(module))
			}

			require.NoError(t, appdatabase.MigrateModules(db, "sqlite", registry))
			for _, table := range tt.wantTables {
				assert.True(t, tableExists(t, db, table), "expected table %q", table)
			}

			// Re-running is idempotent (versions already applied).
			require.NoError(t, appdatabase.MigrateModules(db, "sqlite", registry))
		})
	}
}

func TestMigrateModules_HyphenatedName(t *testing.T) {
	db := openMemoryDB(t)
	registry := kernel.NewRegistry()
	require.NoError(t, registry.Register(
		newStubMigrator("my-module", `CREATE TABLE IF NOT EXISTS gadget (id INTEGER PRIMARY KEY);`),
	))

	require.NoError(t, appdatabase.MigrateModules(db, "sqlite", registry))
	assert.True(t, tableExists(t, db, "gadget"))
	assert.True(t, tableExists(t, db, "schema_migrations_my_module"))
}

func TestMigrateModules_NilRegistry(t *testing.T) {
	assert.NoError(t, appdatabase.MigrateModules(openMemoryDB(t), "sqlite", nil))
}

func TestMigrateModules_UnknownDriver(t *testing.T) {
	registry := kernel.NewRegistry()
	err := appdatabase.MigrateModules(openMemoryDB(t), "cockroach", registry)
	require.Error(t, err)
	assert.ErrorIs(t, err, appdatabase.ErrUnsupportedDriver)
}

func TestMigrateModules_MediaFreshInstall(t *testing.T) {
	core, err := appdatabase.Open("sqlite", ":memory:", 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = core.Close() })
	require.NoError(t, core.RunMigrations("sqlite"))
	assert.False(t, tableExists(t, core.DB(), "media_files"), "core must no longer create media tables")

	registry := kernel.NewRegistry()
	require.NoError(t, registry.Register(mediamodule.New()))
	require.NoError(t, appdatabase.MigrateModules(core.DB(), "sqlite", registry))
	assert.True(t, tableExists(t, core.DB(), "media_files"))
	assert.True(t, tableExists(t, core.DB(), "schema_migrations_media"))

	// Re-running is idempotent.
	require.NoError(t, appdatabase.MigrateModules(core.DB(), "sqlite", registry))
}

func TestMigrateModules_MediaUpgrade(t *testing.T) {
	db := openMemoryDB(t)

	// Simulate a pre-module install: the full media_files table already
	// exists (created by the old core 000001), with no module version table.
	// No core migration ever alters media_files, so this shape is exact.
	_, err := db.Exec(`CREATE TABLE media_files (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    user_id INTEGER NOT NULL,
	    filename TEXT NOT NULL,
	    original_filename TEXT NOT NULL,
	    mime_type TEXT NOT NULL,
	    file_size INTEGER NOT NULL,
	    width INTEGER,
	    height INTEGER,
	    alt_text TEXT,
	    is_webp BOOLEAN DEFAULT TRUE,
	    file_path TEXT NOT NULL,
	    url TEXT NOT NULL,
	    hash TEXT UNIQUE NOT NULL,
	    variants TEXT,
	    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE INDEX idx_media_files_user_id ON media_files(user_id)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE INDEX idx_media_files_hash ON media_files(hash)`)
	require.NoError(t, err)

	registry := kernel.NewRegistry()
	require.NoError(t, registry.Register(mediamodule.New()))
	require.NoError(t, appdatabase.MigrateModules(db, "sqlite", registry))
	assert.True(t, tableExists(t, db, "media_files"))
	assert.True(t, tableExists(t, db, "schema_migrations_media"))
}

func TestMigrateModules_CommentsFreshInstall(t *testing.T) {
	core, err := appdatabase.Open("sqlite", ":memory:", 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = core.Close() })
	require.NoError(t, core.RunMigrations("sqlite"))
	assert.False(t, tableExists(t, core.DB(), "comments"), "core must no longer create comment tables")

	registry := kernel.NewRegistry()
	require.NoError(t, registry.Register(commentmodule.New()))
	require.NoError(t, appdatabase.MigrateModules(core.DB(), "sqlite", registry))
	assert.True(t, tableExists(t, core.DB(), "comments"))
	assert.True(t, tableExists(t, core.DB(), "schema_migrations_comments"))

	// Re-running is idempotent.
	require.NoError(t, appdatabase.MigrateModules(core.DB(), "sqlite", registry))
}

func TestMigrateModules_CommentsUpgrade(t *testing.T) {
	db := openMemoryDB(t)

	// Simulate a pre-module install: the full comments table already
	// exists (created by the old core 000001), with no module version table.
	// No core migration ever alters comments, so this shape is exact.
	_, err := db.Exec(`CREATE TABLE comments (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    content_id INTEGER NOT NULL,
	    user_id INTEGER NOT NULL,
	    comment TEXT NOT NULL,
	    status TEXT NOT NULL DEFAULT 'pending',
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	    FOREIGN KEY (content_id) REFERENCES content_items(id) ON DELETE CASCADE,
	    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE INDEX idx_comments_content_id ON comments(content_id)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE INDEX idx_comments_user_id ON comments(user_id)`)
	require.NoError(t, err)

	registry := kernel.NewRegistry()
	require.NoError(t, registry.Register(commentmodule.New()))
	require.NoError(t, appdatabase.MigrateModules(db, "sqlite", registry))
	assert.True(t, tableExists(t, db, "comments"))
	assert.True(t, tableExists(t, db, "schema_migrations_comments"))
}

func TestMigrateModules_SEOFreshInstall(t *testing.T) {
	core, err := appdatabase.Open("sqlite", ":memory:", 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = core.Close() })
	// Fresh installs apply the gapped core sequence (1,2,3,5) cleanly, then
	// the module migration creates the extracted table.
	require.NoError(t, core.RunMigrations("sqlite"))
	assert.False(t, tableExists(t, core.DB(), "content_aliases"), "core must no longer create alias tables")

	registry := kernel.NewRegistry()
	require.NoError(t, registry.Register(seomodule.New()))
	require.NoError(t, appdatabase.MigrateModules(core.DB(), "sqlite", registry))
	assert.True(t, tableExists(t, core.DB(), "content_aliases"))
	assert.True(t, tableExists(t, core.DB(), "schema_migrations_seo"))

	// Re-running is idempotent.
	require.NoError(t, appdatabase.MigrateModules(core.DB(), "sqlite", registry))
}

func TestMigrateModules_SEOUpgrade(t *testing.T) {
	db := openMemoryDB(t)

	// Simulate a pre-module install: the full content_aliases table already
	// exists (created by the old core 000004), with no module version table.
	// No core migration ever alters content_aliases, so this shape is exact.
	_, err := db.Exec(`CREATE TABLE content_aliases (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    content_id INTEGER NOT NULL,
	    alias TEXT NOT NULL UNIQUE,
	    FOREIGN KEY (content_id) REFERENCES content_items(id) ON DELETE CASCADE
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE INDEX idx_content_aliases_alias ON content_aliases(alias)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE INDEX idx_content_aliases_content_id ON content_aliases(content_id)`)
	require.NoError(t, err)

	registry := kernel.NewRegistry()
	require.NoError(t, registry.Register(seomodule.New()))
	require.NoError(t, appdatabase.MigrateModules(db, "sqlite", registry))
	assert.True(t, tableExists(t, db, "content_aliases"))
	assert.True(t, tableExists(t, db, "schema_migrations_seo"))
}

func TestMigrateModules_OrdersPIIUpgrade(t *testing.T) {
	db := openMemoryDB(t)

	// Simulate a pre-encryption install: the 000001 order tables already
	// exist without the 000002 shipping line or the blind-index column,
	// and with no module version table. The module migrations apply the
	// missing versions on top.
	_, err := db.Exec(`CREATE TABLE orders_order (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    status TEXT NOT NULL DEFAULT 'pending',
	    customer_email TEXT NOT NULL,
	    customer_name TEXT NOT NULL,
	    customer_phone TEXT NOT NULL DEFAULT '',
	    ship_to TEXT NOT NULL DEFAULT '{}',
	    currency TEXT NOT NULL DEFAULT '',
	    subtotal INTEGER NOT NULL DEFAULT 0,
	    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE orders_order_items (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    order_id INTEGER NOT NULL,
	    product_content_id INTEGER NOT NULL,
	    variant_id INTEGER NOT NULL DEFAULT 0,
	    sku TEXT NOT NULL DEFAULT '',
	    title TEXT NOT NULL DEFAULT '',
	    unit_price INTEGER NOT NULL DEFAULT 0,
	    quantity INTEGER NOT NULL DEFAULT 1,
	    FOREIGN KEY (order_id) REFERENCES orders_order(id) ON DELETE CASCADE
	)`)
	require.NoError(t, err)

	registry := kernel.NewRegistry()
	require.NoError(t, registry.Register(ordersmodule.New()))
	require.NoError(t, appdatabase.MigrateModules(db, "sqlite", registry))
	assert.True(t, tableExists(t, db, "orders_order"))
	assert.True(t, tableExists(t, db, "schema_migrations_orders"))
	assert.True(t, columnExists(t, db, "orders_order", "shipping_minor"))
	assert.True(t, columnExists(t, db, "orders_order", "customer_email_idx"))

	// The encrypted write path runs against the migrated schema.
	_, err = db.Exec(`INSERT INTO orders_order
		(status, customer_email, customer_name, customer_phone, ship_to, customer_email_idx, currency, subtotal, shipping_minor)
		VALUES ('pending', 'v1:c2VhbGVk', 'v1:bmFtZQ==', '', '{}', 'idx', 'IDR', 100000, 0)`)
	require.NoError(t, err)

	// Re-running is idempotent.
	require.NoError(t, appdatabase.MigrateModules(db, "sqlite", registry))

	tests := []struct {
		name   string
		driver string
		file   string
	}{
		{
			name:   "success - sqlite migration adds the blind-index column",
			driver: "sqlite",
			file:   "sqlite/000003_orders_pii.up.sql",
		},
		{
			name:   "success - postgresql migration adds the blind-index column",
			driver: "postgresql",
			file:   "postgresql/000003_orders_pii.up.sql",
		},
		{
			name:   "success - mysql migration adds the blind-index column",
			driver: "mysql",
			file:   "mysql/000003_orders_pii.up.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := fs.ReadFile(ordersmodule.New().Migrations(), tt.file)
			require.NoError(t, err, "expected %s migration for %s", tt.file, tt.driver)
			assert.Contains(t, string(payload), "customer_email_idx")
		})
	}
}

func TestMigrateModules_PaymentsPIIUpgrade(t *testing.T) {
	db := openMemoryDB(t)

	// Simulate a pre-encryption install: the 000001 payment table already
	// exists, with no blind-index column and no module version table.
	_, err := db.Exec(`CREATE TABLE payments_payment (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    order_id INTEGER NOT NULL,
	    method TEXT NOT NULL,
	    status TEXT NOT NULL DEFAULT 'pending',
	    reference TEXT NOT NULL DEFAULT '',
	    amount INTEGER NOT NULL DEFAULT 0,
	    verified_by TEXT NOT NULL DEFAULT '',
	    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	registry := kernel.NewRegistry()
	require.NoError(t, registry.Register(paymentsmodule.New()))
	require.NoError(t, appdatabase.MigrateModules(db, "sqlite", registry))
	assert.True(t, tableExists(t, db, "payments_payment"))
	assert.True(t, tableExists(t, db, "schema_migrations_payments"))
	assert.True(t, columnExists(t, db, "payments_payment", "reference_idx"))

	// The encrypted write path runs against the migrated schema.
	_, err = db.Exec(`INSERT INTO payments_payment
		(order_id, method, status, reference, amount, verified_by, reference_idx)
		VALUES (42, 'manual', 'pending', 'v1:c2VhbGVk', 522500, '', 'idx')`)
	require.NoError(t, err)

	// Re-running is idempotent.
	require.NoError(t, appdatabase.MigrateModules(db, "sqlite", registry))

	tests := []struct {
		name   string
		driver string
		file   string
	}{
		{
			name:   "success - sqlite migration adds the blind-index column",
			driver: "sqlite",
			file:   "sqlite/000003_payments_pii.up.sql",
		},
		{
			name:   "success - postgresql migration adds the blind-index column",
			driver: "postgresql",
			file:   "postgresql/000003_payments_pii.up.sql",
		},
		{
			name:   "success - mysql migration adds the blind-index column",
			driver: "mysql",
			file:   "mysql/000003_payments_pii.up.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := fs.ReadFile(paymentsmodule.New().Migrations(), tt.file)
			require.NoError(t, err, "expected %s migration for %s", tt.file, tt.driver)
			assert.Contains(t, string(payload), "reference_idx")
		})
	}
}

// liveTableExists reports whether table exists on a live server database.
// The query is driver-specific: pgx has no ?-placeholder rewriting, and the
// schema is scoped so a shared server cannot leak same-named tables across
// databases. COUNT(*) avoids driver-specific boolean decoding.
func liveTableExists(t *testing.T, db *sql.DB, driver, table string) bool {
	t.Helper()
	var query string
	switch driver {
	case "postgres":
		query = `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1`
	default: // mysql
		query = `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`
	}
	var count int
	require.NoError(t, db.QueryRowContext(
		context.Background(),
		query,
		table,
	).Scan(&count))
	return count > 0
}

func TestMigrateModules_LiveDrivers(t *testing.T) {
	tests := []struct {
		name   string
		driver string
		dsnEnv string
	}{
		{
			name:   "success - postgres versions module independently",
			driver: "postgres",
			dsnEnv: "TEST_POSTGRES_DSN",
		},
		{
			name:   "success - mysql versions module independently",
			driver: "mysql",
			dsnEnv: "TEST_MYSQL_DSN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := os.Getenv(tt.dsnEnv)
			if dsn == "" {
				t.Skipf("%s not set, skipping live migration smoke test", tt.dsnEnv)
			}

			db, err := appdatabase.Open(tt.driver, dsn, 0)
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			require.NoError(t, db.RunMigrations(tt.driver))

			registry := kernel.NewRegistry()
			require.NoError(t, registry.Register(mediamodule.New()))
			require.NoError(t, appdatabase.MigrateModules(db.DB(), tt.driver, registry))
			assert.True(t, liveTableExists(t, db.DB(), tt.driver, "media_files"))
			assert.True(t, liveTableExists(t, db.DB(), tt.driver, "schema_migrations_media"))

			// Re-running is idempotent (versions already applied).
			require.NoError(t, appdatabase.MigrateModules(db.DB(), tt.driver, registry))
		})
	}
}
