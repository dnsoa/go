package sqldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
)

var (
	migrationUpMatcher    = regexp.MustCompile(`^([\w-]+)\.up\.sql$`)
	migrationDownMatcher  = regexp.MustCompile(`^([\w-]+)\.down\.sql$`)
	migrationTableMatcher = regexp.MustCompile(`^[\w.]+$`)
	migrationNameMatcher  = regexp.MustCompile(`^[\w.-]+$`)
)

// MigrationCallback is called before or after each migration inside the same transaction.
type MigrationCallback func(ctx context.Context, tx *sql.Tx, version string) error

// MigrationOption configures a migrator created from DB.
type MigrationOption func(*migrationOptions)

type migrationOptions struct {
	after   MigrationCallback
	before  MigrationCallback
	table   string
	service string
}

// WithMigrationTable sets the migration version table name.
// The table name must match ^[\w.]+$.
func WithMigrationTable(table string) MigrationOption {
	return func(opts *migrationOptions) {
		opts.table = table
	}
}

// WithMigrationService sets the namespace/service name so that multiple
// services sharing the same database keep independent migration histories.
//
// The migrations table stores one row per service (keyed by the service name),
// and each service tracks its own version independently. This avoids conflicts
// when several microservices share a single database.
//
// When unset, the service name defaults to "default", so a single-service
// application still works without any configuration.
//
// The service name must match ^[\w.-]+$.
func WithMigrationService(service string) MigrationOption {
	return func(opts *migrationOptions) {
		opts.service = service
	}
}

// WithMigrationBefore sets a callback run before each migration.
func WithMigrationBefore(before MigrationCallback) MigrationOption {
	return func(opts *migrationOptions) {
		opts.before = before
	}
}

// WithMigrationAfter sets a callback run after each migration.
func WithMigrationAfter(after MigrationCallback) MigrationOption {
	return func(opts *migrationOptions) {
		opts.after = after
	}
}

type Migrator struct {
	after   MigrationCallback
	before  MigrationCallback
	db      *sql.DB
	fs      fs.FS
	table   string
	service string
	flavor  Flavor
}

// NewMigrator creates a migration runner bound to this DB.
func (db *DB) NewMigrator(fsys fs.FS, opts ...MigrationOption) (*Migrator, error) {
	if fsys == nil {
		return nil, fmt.Errorf("migration FS must be set")
	}

	options := migrationOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	if options.table != "" && !migrationTableMatcher.MatchString(options.table) {
		return nil, fmt.Errorf("illegal migration table name %q, must match %s", options.table, migrationTableMatcher.String())
	}

	if options.service != "" && !migrationNameMatcher.MatchString(options.service) {
		return nil, fmt.Errorf("illegal migration service name %q, must match %s", options.service, migrationNameMatcher.String())
	}

	table := options.table
	if table == "" {
		table = "migrations"
	}

	service := options.service
	if service == "" {
		service = defaultMigrationService
	}

	return &Migrator{
		after:   options.after,
		before:  options.before,
		db:      db.DB,
		fs:      fsys,
		table:   table,
		service: service,
		flavor:  db.Flavor,
	}, nil
}

// Up migrates database schema from current version to latest version.
func (m *Migrator) Up(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("error migrating up: %w", err)
		}
	}()

	if err := m.createMigrationsTable(ctx); err != nil {
		return err
	}

	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return err
	}

	names, err := m.getFilenames(migrationUpMatcher)
	if err != nil {
		return err
	}

	for _, name := range names {
		thisVersion := migrationUpMatcher.ReplaceAllString(name, "$1")
		if thisVersion <= currentVersion {
			continue
		}

		if err := m.apply(ctx, name, thisVersion); err != nil {
			return err
		}
	}

	return nil
}

// Down migrates database schema from current version down to zero.
func (m *Migrator) Down(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("error migrating down: %w", err)
		}
	}()

	if err := m.createMigrationsTable(ctx); err != nil {
		return err
	}

	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return err
	}

	names, err := m.getFilenames(migrationDownMatcher)
	if err != nil {
		return err
	}

	for i := len(names) - 1; i >= 0; i-- {
		thisVersion := migrationDownMatcher.ReplaceAllString(names[i], "$1")
		if thisVersion > currentVersion {
			continue
		}

		nextVersion := ""
		if i > 0 {
			nextVersion = migrationDownMatcher.ReplaceAllString(names[i-1], "$1")
		}

		if err := m.apply(ctx, names[i], nextVersion); err != nil {
			return err
		}
	}

	return nil
}

// To migrates database schema to a specific version.
// Empty version behaves like Down.
func (m *Migrator) To(ctx context.Context, version string) error {
	return m.MigrateTo(ctx, version)
}

// MigrateTo migrates database schema to a specific version.
// Empty version behaves like Down.
func (m *Migrator) MigrateTo(ctx context.Context, version string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("error migrating to: %w", err)
		}
	}()

	if version == "" {
		return m.Down(ctx)
	}

	if err := m.createMigrationsTable(ctx); err != nil {
		return err
	}

	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return err
	}

	if currentVersion == version {
		return nil
	}

	var matcher *regexp.Regexp
	if version > currentVersion {
		matcher = migrationUpMatcher
	} else {
		matcher = migrationDownMatcher
	}
	names, err := m.getFilenames(matcher)
	if err != nil {
		return err
	}

	foundVersion := false
	for _, name := range names {
		thisVersion := matcher.ReplaceAllString(name, "$1")
		if thisVersion == version {
			foundVersion = true
		}
	}
	if !foundVersion {
		return errors.New("error finding version " + version)
	}

	switch {
	case version > currentVersion:
		for _, name := range names {
			thisVersion := matcher.ReplaceAllString(name, "$1")
			if thisVersion <= currentVersion {
				continue
			}
			if thisVersion > version {
				break
			}

			if err := m.apply(ctx, name, thisVersion); err != nil {
				return err
			}
		}
	case version < currentVersion:
		for i := len(names) - 1; i >= 0; i-- {
			thisVersion := matcher.ReplaceAllString(names[i], "$1")
			if thisVersion > currentVersion {
				continue
			}

			if thisVersion <= version {
				break
			}

			nextVersion := ""
			if i > 0 {
				nextVersion = matcher.ReplaceAllString(names[i-1], "$1")
			}

			if err := m.apply(ctx, names[i], nextVersion); err != nil {
				return err
			}
		}
	}

	return nil
}

// apply a file identified by name and update to version.
func (m *Migrator) apply(ctx context.Context, name, version string) error {
	content, err := fs.ReadFile(m.fs, name)
	if err != nil {
		return fmt.Errorf("error reading migration file %v: %w", name, err)
	}

	return m.inTransaction(ctx, func(tx *sql.Tx) error {
		if m.before != nil {
			if err := m.before(ctx, tx, version); err != nil {
				return fmt.Errorf("error in 'before' callback when applying version %v from %v: %w", version, name, err)
			}
		}

		if err := m.updateVersion(ctx, tx, version); err != nil {
			return fmt.Errorf("error updating version to %v: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("error running migration %v from %v: %w", version, name, err)
		}

		if m.after != nil {
			if err := m.after(ctx, tx, version); err != nil {
				return fmt.Errorf("error in 'after' callback when applying version %v from %v: %w", version, name, err)
			}
		}
		return nil
	})
}

// getFilenames alphabetically where the name matches the given matcher.
func (m *Migrator) getFilenames(matcher *regexp.Regexp) ([]string, error) {
	var names []string
	entries, err := fs.ReadDir(m.fs, ".")
	if err != nil {
		return names, err
	}

	for _, entry := range entries {
		if !matcher.MatchString(entry.Name()) {
			continue
		}
		names = append(names, entry.Name())
	}
	return names, nil
}

// defaultMigrationService is used when no service namespace is configured via
// WithMigrationService. It keeps a single shared history in the multi-row
// migrations table, equivalent to the old global-version behavior.
const defaultMigrationService = "default"

// createMigrationsTable creates the migrations table if it does not exist yet
// and ensures a row exists for the configured service.
//
// The table always uses the multi-row (service, version) layout so that
// multiple services sharing the same database keep independent histories.
func (m *Migrator) createMigrationsTable(ctx context.Context) error {
	return m.inTransaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `create table if not exists `+m.table+` (service text not null, version text not null, primary key (service))`); err != nil {
			return fmt.Errorf("error creating migrations table %v: %w", m.table, err)
		}

		// Ensure a row exists for this service. Use a flavor-specific upsert so
		// the initial empty version is inserted exactly once.
		if _, err := tx.ExecContext(ctx, fixQuery(m.flavor, m.upsertServiceSQL()), m.service, ""); err != nil {
			return fmt.Errorf("error initializing migration row for service %q: %w", m.service, err)
		}
		return nil
	})
}

// upsertServiceSQL returns a flavor-specific statement that inserts a
// (service, version) row unless the service already exists.
func (m *Migrator) upsertServiceSQL() string {
	switch m.flavor {
	case MySQL:
		return `insert into ` + m.table + ` (service, version) values (?, ?) on duplicate key update service = service`
	default: // PostgreSQL and SQLite both support ON CONFLICT.
		return `insert into ` + m.table + ` (service, version) values (?, ?) on conflict (service) do nothing`
	}
}

// updateVersion sets the current version for the configured service. Uses
// parameter binding to avoid SQL injection.
func (m *Migrator) updateVersion(ctx context.Context, tx *sql.Tx, version string) error {
	_, err := tx.ExecContext(ctx, fixQuery(m.flavor, `update `+m.table+` set version = ? where service = ?`), version, m.service)
	return err
}

// getCurrentVersion from the migrations table.
//
// When no row exists yet (e.g. a service that has never migrated), it returns
// the empty string so callers treat it as the initial version.
func (m *Migrator) getCurrentVersion(ctx context.Context) (string, error) {
	var version string
	err := m.db.QueryRowContext(ctx, fixQuery(m.flavor, `select version from `+m.table+` where service = ?`), m.service).Scan(&version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("error getting current migration version: %w", err)
	}
	return version, nil
}

func (m *Migrator) inTransaction(ctx context.Context, callback func(tx *sql.Tx) error) (err error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error beginning transaction: %w", err)
	}
	defer func() {
		if rec := recover(); rec != nil {
			err = migrationRollback(tx, fmt.Errorf("panic: %v", rec))
		}
	}()
	if err := callback(tx); err != nil {
		return migrationRollback(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

func migrationRollback(tx *sql.Tx, err error) error {
	if txErr := tx.Rollback(); txErr != nil {
		return fmt.Errorf("error rolling back transaction after error (transaction error: %v), original error: %w", txErr, err)
	}
	return err
}

// MigrateUp is a convenience one-liner to migrate to latest version.
func (db *DB) MigrateUp(ctx context.Context, fsys fs.FS, opts ...MigrationOption) error {
	migrator, err := db.NewMigrator(fsys, opts...)
	if err != nil {
		return err
	}
	return migrator.Up(ctx)
}

// MigrateDown is a convenience one-liner to migrate down to zero.
func (db *DB) MigrateDown(ctx context.Context, fsys fs.FS, opts ...MigrationOption) error {
	migrator, err := db.NewMigrator(fsys, opts...)
	if err != nil {
		return err
	}
	return migrator.Down(ctx)
}

// MigrateTo is a convenience one-liner to migrate to a target version.
func (db *DB) MigrateTo(ctx context.Context, fsys fs.FS, version string, opts ...MigrationOption) error {
	migrator, err := db.NewMigrator(fsys, opts...)
	if err != nil {
		return err
	}
	return migrator.To(ctx, version)
}
