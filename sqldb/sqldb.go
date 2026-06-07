package sqldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

var stringBuilderPool = sync.Pool{
	New: func() any {
		return new(strings.Builder)
	}}

func acquireStringBuilder() *strings.Builder {
	return stringBuilderPool.Get().(*strings.Builder)
}

func releaseStringBuilder(b *strings.Builder) {
	b.Reset()
	stringBuilderPool.Put(b)
}

// DatabaseProvider is a legacy alias-like interface that exposes both
// execution and query operations.
type DatabaseProvider interface {
	Execer
	Queryer
}

// ExecerAndQueryer combines SQL execution and query capabilities.
type ExecerAndQueryer interface {
	Execer
	Queryer
}

// Execer represents types that can execute SQL statements.
type Execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Queryer represents types that can perform SQL queries.
type Queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Scan executes a query and scans into dest using context.Background.
//
// See ScanContext for supported destination types.
func Scan(queryer Queryer, dest any, query string, args ...any) error {
	return ScanContext(context.Background(), queryer, dest, query, args...)
}

// ScanContext executes a query and scans rows into dest.
//
// Supported destination types:
//   - *struct: first row is scanned, sql.ErrNoRows if empty.
//   - *[]struct / *[]*struct: all rows are scanned.
//   - *[]scalar: single-column result scanned into scalar slice.
//   - *scalar: first row first column scanned via row.Scan.
//
// Field mapping uses struct tags in this priority: `sql`, then `db`, then
// lower-cased field name.
func ScanContext(ctx context.Context, queryer Queryer, dest any, query string, args ...any) error {

	var vp reflect.Value
	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Ptr {
		return errors.New("must pass a pointer, not a value, to StructScan destination")
	}
	if value.IsNil() {
		return errors.New("nil pointer passed to StructScan destination")
	}

	//case struct or *struct
	base := deref(value.Type())
	switch base.Kind() {
	case reflect.Struct:
		rows, err := queryer.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return sql.ErrNoRows
		}
		columns, err := rows.Columns()
		if err != nil {
			return err
		}
		destElem := value.Elem()
		scanArgs := make([]any, len(columns))
		discard := make([]any, len(columns))
		for i := range columns {
			scanArgs[i] = &discard[i]
		}
		for _, field := range fields(destElem.Type()) {
			if columnIndex := slices.Index(columns, field.name); columnIndex >= 0 {
				scanArgs[columnIndex] = destElem.FieldByIndex(field.field.Index).Addr().Interface()
			}
		}
		// 扫描结果到结构体字段
		err = rows.Scan(scanArgs...)
		if err != nil {
			return err
		}
		return rows.Err()
	case reflect.Slice:
		rows, err := queryer.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		direct := reflect.Indirect(value)
		direct.SetLen(0)
		slice := deref(value.Type())
		base := deref(slice.Elem())
		if base.Kind() == reflect.String || (base.Kind() > reflect.Invalid && base.Kind() < reflect.Array) {
			columns, err := rows.Columns()
			if err != nil {
				return err
			}
			if len(columns) != 1 {
				return fmt.Errorf("can only scan single column into basic type slice, got %d columns", len(columns))
			}

			vp = reflect.New(base)
			scanArgs := []any{vp.Interface()}
			result := direct
			for rows.Next() {
				// vp is reused across rows: reflect.Append copies the scalar
				// value into the slice's backing storage each iteration.
				err = rows.Scan(scanArgs...)
				if err != nil {
					return err
				}
				result = reflect.Append(result, vp.Elem())
			}
			if err := rows.Err(); err != nil {
				return err
			}
			direct.Set(result)
			return nil
		}
		if base.Kind() != reflect.Struct {
			return fmt.Errorf("must pass a pointer to a slice of structs, not %s", base.Kind())
		}
		isPtr := slice.Elem().Kind() == reflect.Pointer
		columns, err := rows.Columns()
		if err != nil {
			return err
		}
		scanArgs := make([]any, len(columns))
		discard := make([]any, len(columns))
		// Unmapped columns always scan into discard; set once up front so the
		// per-row loop only has to (re)point the mapped columns.
		for i := range columns {
			scanArgs[i] = &discard[i]
		}

		// Precompute column -> struct field index mappings once. A slice keeps
		// iteration cheap and avoids per-row map traversal.
		type colMapping struct {
			column int
			index  []int
		}
		var mappings []colMapping
		for _, field := range fields(base) {
			if columnIndex := slices.Index(columns, field.name); columnIndex >= 0 {
				mappings = append(mappings, colMapping{columnIndex, field.field.Index})
			}
		}

		result := direct
		for rows.Next() {
			vp = reflect.New(base)
			elem := vp.Elem()
			for _, m := range mappings {
				scanArgs[m.column] = elem.FieldByIndex(m.index).Addr().Interface()
			}
			err = rows.Scan(scanArgs...)
			if err != nil {
				return err
			}
			if isPtr {
				result = reflect.Append(result, vp)
			} else {
				result = reflect.Append(result, elem)
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		direct.Set(result)
		return nil
	default:
		row := queryer.QueryRowContext(ctx, query, args...)
		return row.Scan(dest)
	}
}

func tagLookup(tag reflect.StructTag) string {
	if s, ok := tag.Lookup("sql"); ok {
		return s
	}
	if s, ok := tag.Lookup("db"); ok {
		return s
	}
	return ""
}

func fixQuery(flavor Flavor, query string) string {
	// Only PostgreSQL uses positional $n placeholders; other flavors keep '?'.
	if flavor != PostgreSQL {
		return query
	}
	// Fast path: nothing to rewrite.
	if !strings.ContainsRune(query, '?') {
		return query
	}
	builder := acquireStringBuilder()
	defer releaseStringBuilder(builder)
	builder.Grow(len(query) + 8)
	argNum := 0
	for i := 0; i < len(query); i++ {
		c := query[i]
		switch c {
		case '\'', '"':
			// Copy a quoted span (string literal or quoted identifier) verbatim
			// so a '?' inside it is never treated as a placeholder. Doubled
			// quotes ('' or "") are in-span escapes, not terminators.
			quote := c
			builder.WriteByte(c)
			i++
			for i < len(query) {
				builder.WriteByte(query[i])
				if query[i] == quote {
					if i+1 < len(query) && query[i+1] == quote {
						i++
						builder.WriteByte(query[i])
						i++
						continue
					}
					break
				}
				i++
			}
		case '?':
			// A doubled '??' is an escaped literal '?' (e.g. PostgreSQL jsonb
			// operators ?, ?| and ?&), emitted as a single '?'.
			if i+1 < len(query) && query[i+1] == '?' {
				builder.WriteByte('?')
				i++
				continue
			}
			argNum++
			builder.WriteByte('$')
			builder.WriteString(strconv.Itoa(argNum))
		default:
			builder.WriteByte(c)
		}
	}
	return builder.String()
}

// FormatSQL formats query with args expanded for debug output.
//
// It is intended for logging/tracing only and must not be used to execute SQL.
func FormatSQL(query string, args []any) string {
	builder := acquireStringBuilder()
	defer releaseStringBuilder(builder)
	if len(args) == 0 {
		return query
	}
	argIndex := 0
	for {
		pos := strings.IndexByte(query, '?')
		if pos < 0 {
			builder.WriteString(query)
			break
		}
		builder.WriteString(query[:pos])
		if argIndex < len(args) {
			builder.WriteString(formatSQLArg(args[argIndex]))
		} else {
			builder.WriteByte('?')
		}
		argIndex++
		query = query[pos+1:]
	}
	return builder.String()
}

func formatSQLArg(arg any) string {
	if arg == nil {
		return "NULL"
	}
	rv := reflect.ValueOf(arg)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return "NULL"
		}
		rv = rv.Elem()
		arg = rv.Interface()
	}

	switch v := arg.(type) {
	case time.Time:
		return "'" + v.Format("2006-01-02 15:04:05") + "'"
	case sql.NullBool:
		if !v.Valid {
			return "NULL"
		}
		if v.Bool {
			return "TRUE"
		}
		return "FALSE"
	case sql.NullInt64:
		if !v.Valid {
			return "NULL"
		}
		return strconv.FormatInt(v.Int64, 10)
	case sql.NullFloat64:
		if !v.Valid {
			return "NULL"
		}
		return strconv.FormatFloat(v.Float64, 'f', -1, 64)
	case sql.NullString:
		if !v.Valid {
			return "NULL"
		}
		return quoteSQLString(v.String)
	case string:
		return quoteSQLString(v)
	case []byte:
		return quoteSQLString(string(v))
	case bool:
		if v {
			return "TRUE"
		}
		return "FALSE"
	case int:
		return strconv.FormatInt(int64(v), 10)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return quoteSQLString(fmt.Sprint(arg))
	}
}

func quoteSQLString(s string) string {
	// Debug formatting only; escapes single quotes to keep output readable.
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// Deref is Indirect for reflect.Types
func deref(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}
