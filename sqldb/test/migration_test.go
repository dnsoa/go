package test

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
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
