package sqldb

import (
	"context"
	"database/sql"
	"fmt"
)

type Tx struct {
	*sql.Tx
	Flavor Flavor
	Option option
}

// Exec runs a statement in the transaction using context.Background.
func (tx *Tx) Exec(query string, args ...any) (sql.Result, error) {
	return tx.ExecContext(context.Background(), query, args...)
}

// ExecContext runs a statement in the transaction with context.
func (tx *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	opt := tx.Option
	if opt.TraceSQL {
		fmt.Println("TraceSQL:Exec ->", FormatSQL(query, args))
		fmt.Println()
	}
	query = fixQuery(tx.Flavor, query)
	if opt.Debug {
		start := nanotime()
		defer opt.Log("query: %s, args: %v, time: %v\n", query, args, timeSince(start))
	}
	return tx.Tx.ExecContext(ctx, query, args...)
}

// Query runs a query in the transaction using context.Background.
func (tx *Tx) Query(query string, args ...any) (*sql.Rows, error) {
	return tx.QueryContext(context.Background(), query, args...)
}

// QueryContext runs a query in the transaction with context.
func (tx *Tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	opt := tx.Option
	if opt.TraceSQL {
		fmt.Println("TraceSQL:Query ->", FormatSQL(query, args))
		fmt.Println()
	}
	query = fixQuery(tx.Flavor, query)
	if opt.Debug {
		start := nanotime()
		defer opt.Log("query: %s, args: %v, time: %v\n", query, args, timeSince(start))
	}
	return tx.Tx.QueryContext(ctx, query, args...)
}

// QueryRow runs a query expected to return at most one row,
// using context.Background.
func (tx *Tx) QueryRow(query string, args ...any) *sql.Row {
	return tx.QueryRowContext(context.Background(), query, args...)
}

// QueryRowContext runs a query expected to return at most one row.
func (tx *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	opt := tx.Option
	if opt.TraceSQL {
		fmt.Println("TraceSQL:QueryRow ->", FormatSQL(query, args))
		fmt.Println()
	}
	query = fixQuery(tx.Flavor, query)
	if opt.Debug {
		start := nanotime()
		defer opt.Log("query: %s, args: %v, time: %v\n", query, args, timeSince(start))
	}
	return tx.Tx.QueryRowContext(ctx, query, args...)
}

// Prepare creates a prepared statement in the transaction using context.Background.
func (tx *Tx) Prepare(query string) (*sql.Stmt, error) {
	return tx.PrepareContext(context.Background(), query)
}

// PrepareContext creates a prepared statement in the transaction with context.
func (tx *Tx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	query = fixQuery(tx.Flavor, query)
	return tx.Tx.PrepareContext(ctx, query)
}

// QueryScan executes query and scans rows into dest in the transaction.
func (tx *Tx) QueryScan(dest any, query string, args ...any) error {
	return Scan(tx, dest, query, args...)
}
