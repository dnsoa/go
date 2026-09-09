package test

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/dnsoa/go/assert"
	"github.com/dnsoa/go/sqldb"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed testdata/svc-a/*.sql
var svcAFS embed.FS

//go:embed testdata/svc-b/*.sql
var svcBFS embed.FS

func subFS(efs embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(efs, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

func newMemoryDB(t *testing.T) *sqldb.DB {
	r := assert.New(t)
	db, err := sqldb.Open("sqlite3", ":memory:")
	r.NoError(err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// tableExists reports whether the given table exists in the sqlite database.
func tableExists(t *testing.T, db *sqldb.DB, table string) bool {
	r := assert.New(t)
	var name string
	err := db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	r.NoError(err)
	return name == table
}

// migrationVersion reads the version for a given service (empty service = the
// legacy single-row layout). Returns empty string when no row exists yet.
func migrationVersion(t *testing.T, db *sqldb.DB, service string) string {
	r := assert.New(t)
	var version string
	var err error
	if service == "" {
		err = db.QueryRow("SELECT version FROM migrations LIMIT 1").Scan(&version)
	} else {
		err = db.QueryRow("SELECT version FROM migrations WHERE service = ?", service).Scan(&version)
	}
	if err == sql.ErrNoRows {
		return ""
	}
	r.NoError(err)
	return version
}

func TestMigration_UpDown(t *testing.T) {
	r := assert.New(t)
	ctx := context.Background()
	db := newMemoryDB(t)

	r.NoError(db.MigrateUp(ctx, subFS(svcAFS, "testdata/svc-a")))
	r.Equal("002_add_email", migrationVersion(t, db, "default"))
	r.True(tableExists(t, db, "users_a"))

	r.NoError(db.MigrateDown(ctx, subFS(svcAFS, "testdata/svc-a")))
	r.Equal("", migrationVersion(t, db, "default"))
	r.False(tableExists(t, db, "users_a"))
}

// TestMigration_MultipleServicesIsolated verifies the core bug fix: two
// services sharing the same database keep independent migration histories and
// do not skip or clobber each other's versions.
func TestMigration_MultipleServicesIsolated(t *testing.T) {
	r := assert.New(t)
	ctx := context.Background()
	db := newMemoryDB(t)

	// Service A migrates to its own latest version.
	r.NoError(db.MigrateUp(ctx, subFS(svcAFS, "testdata/svc-a"), sqldb.WithMigrationService("svc-a")))
	r.Equal("002_add_email", migrationVersion(t, db, "svc-a"))

	// Service B must still be at the initial version: A's progress must not
	// leak into B. (This is exactly the bug: previously B would see A's "002"
	// and skip its own migrations.)
	r.Equal("", migrationVersion(t, db, "svc-b"))

	// Now migrate B independently; it must reach its own 002 without being
	// skipped because A already reached 002.
	r.NoError(db.MigrateUp(ctx, subFS(svcBFS, "testdata/svc-b"), sqldb.WithMigrationService("svc-b")))
	r.Equal("002_add_orders", migrationVersion(t, db, "svc-b"))

	// A's version is untouched by B's migration.
	r.Equal("002_add_email", migrationVersion(t, db, "svc-a"))

	// Both services' tables exist.
	r.True(tableExists(t, db, "users_a"))
	r.True(tableExists(t, db, "users_b"))
	r.True(tableExists(t, db, "orders_b"))

	// The migrations table holds exactly two rows, one per service.
	var count int
	r.NoError(db.QueryRow("SELECT count(*) FROM migrations").Scan(&count))
	r.Equal(2, count)
}

// TestMigration_DefaultService verifies that omitting WithMigrationService
// uses the "default" service namespace and still produces the multi-row table.
func TestMigration_DefaultService(t *testing.T) {
	r := assert.New(t)
	ctx := context.Background()
	db := newMemoryDB(t)

	r.NoError(db.MigrateUp(ctx, subFS(svcAFS, "testdata/svc-a")))
	r.Equal("002_add_email", migrationVersion(t, db, "default"))

	// The table uses the multi-row (service, version) layout.
	rows, err := db.Query("PRAGMA table_info(migrations)")
	r.NoError(err)
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue any
		r.NoError(rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk))
		cols = append(cols, name)
	}
	r.NoError(rows.Err())
	r.DeepEqual([]string{"service", "version"}, cols)

	// Exactly one row, for the default service.
	var count int
	r.NoError(db.QueryRow("SELECT count(*) FROM migrations").Scan(&count))
	r.Equal(1, count)
}

// TestMigration_DefaultServiceIsolatesFromNamedService verifies that the
// default service namespace and an explicitly named service do not interfere
// with each other.
func TestMigration_DefaultServiceIsolatesFromNamedService(t *testing.T) {
	r := assert.New(t)
	ctx := context.Background()
	db := newMemoryDB(t)

	// Default service migrates svc-a files to 002.
	r.NoError(db.MigrateUp(ctx, subFS(svcAFS, "testdata/svc-a")))
	r.Equal("002_add_email", migrationVersion(t, db, "default"))

	// A named service on the same DB starts fresh at the empty version.
	r.Equal("", migrationVersion(t, db, "svc-b"))

	r.NoError(db.MigrateUp(ctx, subFS(svcBFS, "testdata/svc-b"), sqldb.WithMigrationService("svc-b")))
	r.Equal("002_add_orders", migrationVersion(t, db, "svc-b"))
	r.Equal("002_add_email", migrationVersion(t, db, "default"))
}

// TestMigration_LegacyTableUpgrade verifies that a database holding a legacy
// single-column (version) migrations table is upgraded in place: the existing
// row is adopted into the default service and migrations are not replayed.
func TestMigration_LegacyTableUpgrade(t *testing.T) {
	r := assert.New(t)
	ctx := context.Background()
	db := newMemoryDB(t)

	// Simulate a pre-per-service database: already at the latest version.
	_, err := db.Exec("CREATE TABLE migrations (version text not null)")
	r.NoError(err)
	_, err = db.Exec("INSERT INTO migrations (version) VALUES ('002_add_email')")
	r.NoError(err)

	r.NoError(db.MigrateUp(ctx, subFS(svcAFS, "testdata/svc-a")))
	r.Equal("002_add_email", migrationVersion(t, db, "default"))

	// The legacy row was adopted, not duplicated.
	var count int
	r.NoError(db.QueryRow("SELECT count(*) FROM migrations").Scan(&count))
	r.Equal(1, count)
}

// TestMigration_MigrateToWithService verifies targeted migration to a specific
// version works with service namespaces.
func TestMigration_MigrateToWithService(t *testing.T) {
	r := assert.New(t)
	ctx := context.Background()
	db := newMemoryDB(t)

	r.NoError(db.MigrateTo(ctx, subFS(svcAFS, "testdata/svc-a"), "001_init", sqldb.WithMigrationService("svc-a")))
	r.Equal("001_init", migrationVersion(t, db, "svc-a"))
	r.True(tableExists(t, db, "users_a"))

	// Migrate up to 002.
	r.NoError(db.MigrateTo(ctx, subFS(svcAFS, "testdata/svc-a"), "002_add_email", sqldb.WithMigrationService("svc-a")))
	r.Equal("002_add_email", migrationVersion(t, db, "svc-a"))

	// Migrate back down to 001.
	r.NoError(db.MigrateTo(ctx, subFS(svcAFS, "testdata/svc-a"), "001_init", sqldb.WithMigrationService("svc-a")))
	r.Equal("001_init", migrationVersion(t, db, "svc-a"))
}

// TestMigration_RejectIllegalServiceName ensures invalid service names are
// rejected at Migrator construction time.
func TestMigration_RejectIllegalServiceName(t *testing.T) {
	r := assert.New(t)
	db := newMemoryDB(t)

	_, err := db.NewMigrator(subFS(svcAFS, "testdata/svc-a"), sqldb.WithMigrationService("bad service!"))
	r.Error(err)

	// A valid name is accepted.
	_, err = db.NewMigrator(subFS(svcAFS, "testdata/svc-a"), sqldb.WithMigrationService("my.service-1"))
	r.NoError(err)
}

// openMySQL returns a live MySQL handle for the dialect-specific tests below,
// skipping when no server is reachable — the same contract db_test.go's mysql
// subtests use. MYSQL_TEST_DSN overrides the default DSN so CI (or a local
// container with different credentials) can point the suite at its server.
//
// sqldb.Open is lazy — it validates the DSN shape, not the server — so the
// availability probe has to be a real round trip. Otherwise a machine that
// happens to run MySQL on 3306 with different credentials turns the skip into
// an Error 1045 failure.
func openMySQL(t *testing.T) *sqldb.DB {
	t.Helper()
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		dsn = "root:admin@tcp(127.0.0.1:3306)/test"
	}
	db, err := sqldb.Open("mysql", dsn)
	if err != nil {
		t.Skip("MySQL database not available, skipping test")
	}
	if _, err := db.Exec("SELECT 1"); err != nil {
		_ = db.Close()
		t.Skip("MySQL database not available, skipping test")
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestMigration_MySQLUpDown is the Error 1170 regression: MySQL refuses a TEXT
// column in a key specification ("BLOB/TEXT column 'service' used in key
// specification without a key length"), so on a fresh database the very first
// MigrateUp used to fail while creating the bookkeeping table itself. The
// dialect-specific DDL aside, the flow must behave exactly as on SQLite: up
// records the version, down rewinds it to "" and removes the schema, and the
// pair is reversible.
func TestMigration_MySQLUpDown(t *testing.T) {
	r := assert.New(t)
	ctx := context.Background()
	db := openMySQL(t)

	// The MySQL tests share one server (and unlike :memory:, state survives the
	// test), and svc-a's up-migration is a bare CREATE TABLE — so both the
	// bookkeeping table and the schema it tracks have to go, or the next run
	// fails with "users_a already exists" and looks like a migrator bug.
	_, err := db.Exec("DROP TABLE IF EXISTS migrations")
	r.NoError(err)
	_, err = db.Exec("DROP TABLE IF EXISTS users_a")
	r.NoError(err)

	r.NoError(db.MigrateUp(ctx, subFS(svcAFS, "testdata/svc-a")))
	r.Equal("002_add_email", migrationVersion(t, db, "default"))

	r.NoError(db.MigrateDown(ctx, subFS(svcAFS, "testdata/svc-a")))
	r.Equal("", migrationVersion(t, db, "default"))

	r.NoError(db.MigrateUp(ctx, subFS(svcAFS, "testdata/svc-a")))
	r.Equal("002_add_email", migrationVersion(t, db, "default"))
}

// TestMigration_MySQLLegacyTableUpgrade covers the upgrade path on MySQL: a
// legacy single-column table takes an ALTER that adds the service column with a
// constant DEFAULT. MySQL also refuses constant DEFAULTs on TEXT columns
// (Error 1101), so the column is VARCHAR there — and the primary key the
// upgrade then adds lands on an indexable type instead of tripping Error 1170.
func TestMigration_MySQLLegacyTableUpgrade(t *testing.T) {
	r := assert.New(t)
	ctx := context.Background()
	db := openMySQL(t)

	// Same shared-server cleanup as TestMigration_MySQLUpDown: the legacy-row
	// INSERT below would otherwise duplicate on a re-run.
	_, err := db.Exec("DROP TABLE IF EXISTS migrations")
	r.NoError(err)
	_, err = db.Exec("DROP TABLE IF EXISTS users_a")
	r.NoError(err)

	// A pre-per-service database, already at the latest version.
	_, err = db.Exec("CREATE TABLE migrations (version text not null)")
	r.NoError(err)
	_, err = db.Exec("INSERT INTO migrations (version) VALUES ('002_add_email')")
	r.NoError(err)

	r.NoError(db.MigrateUp(ctx, subFS(svcAFS, "testdata/svc-a")))
	r.Equal("002_add_email", migrationVersion(t, db, "default"))

	// The legacy row was adopted, not duplicated.
	var count int
	r.NoError(db.QueryRow("SELECT count(*) FROM migrations").Scan(&count))
	r.Equal(1, count)
}

// openPostgres is openMySQL's twin for the PostgreSQL-specific tests below,
// with the same live-round-trip probe and the same env override so a container
// on a non-default port can be used without editing the file.
func openPostgres(t *testing.T) *sqldb.DB {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		dsn = "user=postgres host=localhost port=5432 password=admin dbname=postgres sslmode=disable"
	}
	db, err := sqldb.Open("pgx", dsn)
	if err != nil {
		t.Skip("PostgreSQL database not available, skipping test")
	}
	if _, err := db.Exec("SELECT 1"); err != nil {
		_ = db.Close()
		t.Skip("PostgreSQL database not available, skipping test")
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// legacyTableWithOwnPrimaryKey is the case the plain ADD PRIMARY KEY cannot
// serve: a pre-per-service migrations table whose single column is itself the
// primary key — `create table migrations (version ... primary key)`, as
// plausible a legacy shape as the bare `(version)` one. Adding a second
// primary key fails, and the upsert that follows needs (service) to be
// unique — so the upgrade falls back to a unique index.
//
// The assertion that matters is the second MigrateUp. Before the fallback
// existed the failed ALTER was ignored, and without any unique key MySQL's
// `on duplicate key update` matched nothing while PostgreSQL's `on conflict
// (service)` refused to execute: one flavor grew a row per Open, the other
// could not open at all.
func legacyTableWithOwnPrimaryKey(t *testing.T, db *sqldb.DB, versionColumn string) {
	r := assert.New(t)
	ctx := context.Background()

	_, err := db.Exec("DROP TABLE IF EXISTS migrations")
	r.NoError(err)
	_, err = db.Exec("DROP TABLE IF EXISTS users_a")
	r.NoError(err)

	_, err = db.Exec("CREATE TABLE migrations (" + versionColumn + ")")
	r.NoError(err)
	_, err = db.Exec("INSERT INTO migrations (version) VALUES ('002_add_email')")
	r.NoError(err)

	r.NoError(db.MigrateUp(ctx, subFS(svcAFS, "testdata/svc-a")))
	r.Equal("002_add_email", migrationVersion(t, db, "default"))

	// Idempotent: the legacy row was adopted rather than duplicated, and a
	// second run inserts nothing.
	r.NoError(db.MigrateUp(ctx, subFS(svcAFS, "testdata/svc-a")))
	var count int
	r.NoError(db.QueryRow("SELECT count(*) FROM migrations").Scan(&count))
	r.Equal(1, count)
}

func TestMigration_MySQLLegacyTableWithOwnPrimaryKey(t *testing.T) {
	legacyTableWithOwnPrimaryKey(t, openMySQL(t), "version varchar(191) not null primary key")
}

func TestMigration_PostgresLegacyTableWithOwnPrimaryKey(t *testing.T) {
	legacyTableWithOwnPrimaryKey(t, openPostgres(t), "version text primary key")
}

// TestMigration_MySQLLegacyTableThatCannotBeKeyed is the other half: a legacy
// table holding more than one row cannot represent a per-service history at
// all. Neither the primary key nor the unique index can be created, and the
// upgrade now says so — where it used to continue and let MySQL append a
// duplicate row on every Open, leaving the recorded version to chance.
func TestMigration_MySQLLegacyTableThatCannotBeKeyed(t *testing.T) {
	r := assert.New(t)
	ctx := context.Background()
	db := openMySQL(t)

	_, err := db.Exec("DROP TABLE IF EXISTS migrations")
	r.NoError(err)
	_, err = db.Exec("DROP TABLE IF EXISTS users_a")
	r.NoError(err)

	_, err = db.Exec("CREATE TABLE migrations (version text not null)")
	r.NoError(err)
	_, err = db.Exec("INSERT INTO migrations (version) VALUES ('001_init'), ('002_add_email')")
	r.NoError(err)

	err = db.MigrateUp(ctx, subFS(svcAFS, "testdata/svc-a"))
	r.Error(err)
	r.True(strings.Contains(err.Error(), "cannot be made unique"))
}
