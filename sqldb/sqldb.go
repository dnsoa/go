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
			return sql.ErrNoRows
		}
		columns, err := rows.Columns()
		if err != nil {
			return err
		}
		destElem := value.Elem()
		scanArgs := make([]any, len(columns))
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
			return nil
		}
		if base.Kind() != reflect.Struct {
			return fmt.Errorf("must pass a pointer to a slice of structs, not %s", base.Kind())
		}
		isPtr := slice.Elem().Kind() == reflect.Ptr
		columns, err := rows.Columns()
		if err != nil {
			return err
		}
		scanArgs := make([]any, len(columns))
		fieldMap := make(map[int][]int)

		// 预先计算字段索引
		for _, field := range fields(base) {
			if columnIndex := slices.Index(columns, field.name); columnIndex >= 0 {
				fieldMap[columnIndex] = field.field.Index
			}
		}
		for rows.Next() {
			vp = reflect.New(base)
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
	default:
		row := queryer.QueryRowContext(ctx, query, args...)
		return row.Scan(dest)
	}

	return nil
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
	nArgs := len(args)
	if nArgs == 0 {
		return query
	}
	var i, j int
	for i = strings.IndexRune(query, '?'); i != -1; i = strings.IndexRune(query, '?') {
		builder.WriteString(query[:i])
		switch a := args[j].(type) {
		// case *int64:
		// 	val := args[i]
		// 	if val.(*int64) != nil {
		// 		builder.WriteString(fmt.Sprintf("%d", *val.(*int64)))
		// 	} else {
		// 		builder.WriteString("NULL")
		// 	}
		// case *int:
		// 	val := args[i]
		// 	if val.(*int) != nil {
		// 		builder.WriteString(fmt.Sprintf("%d", *val.(*int)))
		// 	} else {
		// 		builder.WriteString("NULL")
		// 	}
		case *float64, *float32:
			val := args[i]
			if val.(*float64) != nil {
				fmt.Fprintf(builder, "%f", *val.(*float64))
			} else {
				builder.WriteString("NULL")
			}
		case *bool:
			val := args[i]
			if val.(*bool) != nil {
				fmt.Fprintf(builder, "%t", *val.(*bool))
			} else {
				builder.WriteString("NULL")
			}
		case *string:
			val := args[i]
			if val.(*string) != nil {
				fmt.Fprintf(builder, "'%q'", *val.(*string))
			} else {
				builder.WriteString("NULL")
			}
		case *time.Time:
			val := args[i]
			if val.(*time.Time) != nil {
				time := *val.(*time.Time)
				fmt.Fprintf(builder, "'%v'", time.Format("2006-01-02 15:04:05"))
			} else {
				builder.WriteString("NULL")
			}
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64:
			fmt.Fprintf(builder, "%d", a)
		case float64:
			fmt.Fprintf(builder, "%f", a)
		case bool:
			fmt.Fprintf(builder, "%t", a)
		case time.Time:
			fmt.Fprintf(builder, "'%v'", a.Format("2006-01-02 15:04:05"))
		case sql.NullBool:
			if a.Valid {
				fmt.Fprintf(builder, "%t", a.Bool)
			} else {
				builder.WriteString("NULL")
			}
		case sql.NullInt64:
			if a.Valid {
				fmt.Fprintf(builder, "%d", a.Int64)
			} else {
				builder.WriteString("NULL")
			}
		case sql.NullString:
			if a.Valid {
				fmt.Fprintf(builder, "%q", a.String)
			} else {
				builder.WriteString("NULL")
			}
		case sql.NullFloat64:
			if a.Valid {
				fmt.Fprintf(builder, "%f", a.Float64)
			} else {
				builder.WriteString("NULL")
			}
		case *int, *int8, *int16, *int32, *int64,
			*uint, *uint8, *uint16, *uint32, *uint64:
			val := args[i]
			if val.(*int) != nil {
				builder.WriteString(fmt.Sprintf("%d", *val.(*int)))
			} else {
				builder.WriteString("NULL")
			}
		case string:
			fmt.Fprintf(builder, "'%q'", a)
		case nil:
			builder.WriteString("NULL")
		default:
			fmt.Fprintf(builder, "'%v'", a)
		}
		query = query[i+1:]
		j++
	}
	builder.WriteString(query)
	return builder.String()
}

// Deref is Indirect for reflect.Types
func deref(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}
