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

type DatabaseProvider interface {
	Execer
	Queryer
}

type ExecerAndQueryer interface {
	Execer
	Queryer
}

type Execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type Queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func Scan(queryer Queryer, dest any, query string, args ...any) error {
	return ScanContext(context.Background(), queryer, dest, query, args...)
}

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
			scanArgs := make([]any, 1)
			for rows.Next() {
				scanArgs[0] = vp.Interface()
				err = rows.Scan(scanArgs...)
				if err != nil {
					return err
				}
				direct.Set(reflect.Append(direct, reflect.Indirect(vp)))
			}
			return rows.Err()
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
		fieldMap := make(map[int][]int)

		// 预先计算字段索引
		for _, field := range fields(base) {
			if columnIndex := slices.Index(columns, field.name); columnIndex >= 0 {
				fieldMap[columnIndex] = field.field.Index
			}
		}
		for rows.Next() {
			vp = reflect.New(base)
			for i := range columns {
				scanArgs[i] = &discard[i]
			}
			for columnIndex, fieldIndex := range fieldMap {
				scanArgs[columnIndex] = vp.Elem().FieldByIndex(fieldIndex).Addr().Interface()
			}
			err = rows.Scan(scanArgs...)
			if err != nil {
				return err
			}
			// append
			if isPtr {
				direct.Set(reflect.Append(direct, vp))
			} else {
				direct.Set(reflect.Append(direct, reflect.Indirect(vp)))
			}
		}
		return rows.Err()
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
	switch flavor {
	case MySQL, SQLite:
		return query
	}
	builder := acquireStringBuilder()
	defer releaseStringBuilder(builder)
	var i, j int
	for i = strings.IndexRune(query, '?'); i != -1; i = strings.IndexRune(query, '?') {
		j++
		builder.WriteString(query[:i])
		switch flavor {
		case PostgreSQL:
			builder.WriteString("$" + strconv.Itoa(j))
		}
		query = query[i+1:]
	}
	builder.WriteString(query)
	return builder.String()
}

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
