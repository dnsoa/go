package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

type Option func(opt *option)

func WithDebug(debug bool) Option {
	return func(opt *option) {
		opt.Debug = debug
	}
}

func WithLog(log func(string, ...any)) Option {
	return func(opt *option) {
		opt.Log = log
	}
}

func WithTraceSQL(traceSQL bool) Option {
	return func(opt *option) {
		opt.TraceSQL = traceSQL
	}
}

type option struct {
	Debug    bool
	TraceSQL bool
	Log      func(string, ...any)
}

type DB struct {
	*sql.DB
	*builder
	Flavor Flavor
	Option option
}

// Open is the same as sql.Open, but returns an *sqlx.DB instead.
func Open(driverName, dataSourceName string, opts ...Option) (*DB, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	flavor := invalidFlavor
	switch driverName {
	case "mysql", "nrmysql":
		flavor = MySQL
	case "postgres", "pgx", "pq-timeouts", "cloudsqlpostgres", "ql", "nrpostgres", "cockroach":
		flavor = PostgreSQL
	case "sqlite3", "sqlite", "nrsqlite3":
		flavor = SQLite
	default:
		err = fmt.Errorf("unsupported driver: %s", driverName)
	}
	sqlDB := &DB{
		DB:     db,
		Flavor: flavor,
		Option: option{
			Debug: false,
			Log:   log.Printf,
		},
	}
	for _, opt := range opts {
		opt(&sqlDB.Option)
	}
	sqlDB.builder = newBuilder(flavor, sqlDB)
	return sqlDB, err
}

// Connect to a database and verify with a ping.
func Connect(driverName, dataSourceName string) (*DB, error) {
	db, err := Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func NewSqlDB(db *sql.DB, flavor Flavor, opts ...Option) *DB {
	sqlDB := &DB{
		DB:     db,
		Flavor: flavor,
		Option: option{
			Debug: false,
			Log:   log.Printf,
		},
	}
	for _, opt := range opts {
		opt(&sqlDB.Option)
	}
	sqlDB.builder = newBuilder(flavor, sqlDB)
	return sqlDB
}

// NewSQLDB is an alias of NewSqlDB.
func NewSQLDB(db *sql.DB, flavor Flavor, opts ...Option) *DB {
	return NewSqlDB(db, flavor, opts...)
}

func (db *DB) Begin() (*Tx, error) {
	return db.BeginTx(context.Background(), nil)
}

func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := db.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{
		Tx:     tx,
		Flavor: db.Flavor,
		Option: db.Option,
	}, nil
}

// func (db *DB) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
// 	start := Now()
// 	stmt, err := db.DB.Prepare(query)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to prepare query: %s with error: %w", query, err)
// 	}
// 	defer stmt.Close()
// 	rows, err := stmt.QueryContext(ctx, args...)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to execute query: %s with error: %w", query, err)
// 	}
// 	spentTime := Since(start)
// 	if db.Debug {
// 		db.Log("query: %s, args: %v, time: %v\n", query, args, spentTime)
// 	}
// 	return rows, nil
// }

func (db *DB) Transaction(txFunc func(*Tx) error) (err error) {
	tx, err := db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	err = txFunc(tx)
	if err != nil {
		return
	}
	err = tx.Commit()
	return err
}

// func (db *DB) Insert(ctx context.Context, table string, data map[string]any) (sql.Result, error) {
// 	return Insert(ctx, db.Flavor, db.Option.Prefix, db, table, data)
// }

// func (db *DB) Update(ctx context.Context, table string, data map[string]any, where Conditions) (sql.Result, error) {
// 	return Update(ctx, db.Flavor, db.Option.Prefix, db, table, data, where)
// }

func (db *DB) QueryScan(dest any, query string, args ...any) error {
	return db.QueryScanContext(context.Background(), dest, query, args...)
}

func (db *DB) QueryScanContext(ctx context.Context, dest any, query string, args ...any) error {
	return ScanContext(ctx, db, dest, query, args...)
}

// func (db *DB) Count(ctx context.Context, table string, where string, args ...any) (int, error) {
// 	return Count(ctx, db, table, where, args...)
// }

func (db *DB) Exec(query string, args ...any) (sql.Result, error) {
	return db.ExecContext(context.Background(), query, args...)
}

func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	opt := db.Option
	if opt.TraceSQL {
		fmt.Println("TraceSQL:Exec ->", FormatSQL(query, args))
		fmt.Println()
	}
	query = fixQuery(db.Flavor, query)
	if opt.Debug {
		start := nanotime()
		defer opt.Log("query: %s, args: %v, time: %v\n", query, args, timeSince(start))
	}
	return db.DB.ExecContext(ctx, query, args...)
}

func (db *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return db.QueryContext(context.Background(), query, args...)
}

func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	opt := db.Option
	if opt.TraceSQL {
		fmt.Println("TraceSQL:Query ->", FormatSQL(query, args))
		fmt.Println()
	}
	query = fixQuery(db.Flavor, query)
	if opt.Debug {
		start := nanotime()
		defer opt.Log("query: %s, args: %v, time: %v\n", query, args, timeSince(start))
	}
	return db.DB.QueryContext(ctx, query, args...)
}

func (db *DB) QueryRow(query string, args ...any) *sql.Row {
	return db.QueryRowContext(context.Background(), query, args...)
}

func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	opt := db.Option
	if opt.TraceSQL {
		fmt.Println("TraceSQL:QueryRow ->", FormatSQL(query, args))
		fmt.Println()
	}
	query = fixQuery(db.Flavor, query)
	if opt.Debug {
		start := nanotime()
		defer opt.Log("query: %s, args: %v, time: %v\n", query, args, timeSince(start))
	}
	return db.DB.QueryRowContext(ctx, query, args...)
}

func (db *DB) Prepare(query string) (*sql.Stmt, error) {
	return db.PrepareContext(context.Background(), query)
}

func (db *DB) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	query = fixQuery(db.Flavor, query)
	return db.DB.PrepareContext(ctx, query)
}

// Table starts a new query builder for the given table.
//
// NOTE: This intentionally returns a fresh builder to avoid shared mutable state
// on DB when chaining builder methods.
func (db *DB) Table(table string) *builder {
	b := newBuilder(db.Flavor, db)
	b.table = table
	return b
}
