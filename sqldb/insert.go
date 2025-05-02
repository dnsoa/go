package sqldb

//fork https://github.com/zachvictor/sqlinsert
import (
	"fmt"
	"reflect"
)

// TokenType represents a type of token in a SQL INSERT statement, whether column or value expression.
type TokenType int

const (

	/* COLUMN TokenType */

	// ColumnNameTokenType uses the column name from the struct tag specified by UseStructTag.
	// INSERT INTO tbl (foo, bar, ... baz)
	ColumnNameTokenType TokenType = 0

	/* VALUE TokenType */

	// QuestionMarkTokenType uses question marks as value-tokens.
	// VALUES (?, ?, ... ?) -- MySQL, SingleStore
	QuestionMarkTokenType TokenType = 1

	// AtColumnNameTokenType uses @ followed by the column name from the struct tag specified by UseStructTag.
	// VALUES (@foo, @bar, ... @baz) -- MySQL, SingleStore
	AtColumnNameTokenType TokenType = 2

	// OrdinalNumberTokenType uses % plus the value of an ordered sequence of integers starting at 1.
	// %1, %2, ... %n -- Postgres
	OrdinalNumberTokenType TokenType = 3

	// ColonTokenType uses : followed by the column name from the struct tag specified by UseStructTag.
	// :foo, :bar, ... :baz -- Oracle
	ColonTokenType TokenType = 4
)

func tokenize(recordType reflect.Type, tokenType TokenType) string {
	b := acquireStringBuilder()
	defer releaseStringBuilder(b)
	b.WriteString(`(`)
	for i := 0; i < recordType.NumField(); i++ {
		switch tokenType {
		case ColumnNameTokenType:
			b.WriteString(tagLookup(recordType.Field(i).Tag))
		case QuestionMarkTokenType:
			_, _ = fmt.Fprint(b, `?`)
		case AtColumnNameTokenType:
			_, _ = fmt.Fprintf(b, `@%s`, tagLookup(recordType.Field(i).Tag))
		case OrdinalNumberTokenType:
			_, _ = fmt.Fprintf(b, `$%d`, i+1)
		case ColonTokenType:
			_, _ = fmt.Fprintf(b, `:%s`, tagLookup(recordType.Field(i).Tag))
		}
		if i < recordType.NumField()-1 {
			b.WriteString(`,`)
		}
	}
	b.WriteString(`)`)
	return b.String()
}

// inserter models data used to produce a valid SQL INSERT statement with bind args.
// Table is the table name. Data is either a struct with column-name tagged fields and the data to be inserted or
// a slice struct (struct ptr works too).
type inserter struct {
	Table string
	Data  any
}

// Columns returns the comma-separated list of column names-as-tokens for the SQL INSERT statement.
// Multi Row inserter: inserter.Data is a slice; first item in slice is
func (ins *inserter) Columns() string {
	v := reflect.ValueOf(ins.Data)
	if v.Kind() == reflect.Slice {
		if v.Index(0).Kind() == reflect.Pointer {
			return tokenize(v.Index(0).Elem().Type(), ColumnNameTokenType)
		} else {
			return tokenize(v.Index(0).Type(), ColumnNameTokenType)
		}
	} else if v.Kind() == reflect.Pointer {
		return tokenize(v.Elem().Type(), ColumnNameTokenType)
	} else {
		return tokenize(v.Type(), ColumnNameTokenType)
	}
}

// Params returns the comma-separated list of bind param tokens for the SQL INSERT statement.
func (ins *inserter) Params() string {
	v := reflect.ValueOf(ins.Data)
	if v.Kind() == reflect.Slice {
		var (
			b        = acquireStringBuilder()
			paramRow string
		)
		defer releaseStringBuilder(b)
		if v.Index(0).Kind() == reflect.Pointer {
			paramRow = tokenize(v.Index(0).Elem().Type(), QuestionMarkTokenType)
		} else {
			paramRow = tokenize(v.Index(0).Type(), QuestionMarkTokenType)
		}
		b.WriteString(paramRow)
		for i := 1; i < v.Len(); i++ {
			b.WriteString(`,`)
			b.WriteString(paramRow)
		}
		return b.String()
	} else if v.Kind() == reflect.Pointer {
		return tokenize(v.Elem().Type(), QuestionMarkTokenType)
	} else {
		return tokenize(v.Type(), QuestionMarkTokenType)
	}
}

// SQL returns the full parameterized SQL INSERT statement.
func (ins *inserter) SQL() string {
	b := acquireStringBuilder()
	defer releaseStringBuilder(b)
	_, _ = fmt.Fprintf(b, `INSERT INTO %s %s VALUES %s`,
		ins.Table, ins.Columns(), ins.Params())
	return b.String()
}

// Args returns the arguments to be bound in inserter() or the variadic Exec/ExecContext functions in database/sql.
func (ins *inserter) Args() []any {
	var (
		data    reflect.Value
		rec     reflect.Value
		recType reflect.Type
		args    []any
	)
	data = reflect.ValueOf(ins.Data)
	if data.Kind() == reflect.Slice { // Multi row INSERT: inserter.Data is a slice-of-struct-pointer or slice-of-struct
		argIndex := -1
		if data.Index(0).Kind() == reflect.Pointer { // First slice element is struct pointers
			recType = data.Index(0).Elem().Type()
		} else { // First slice element is struct
			recType = data.Index(0).Type()
		}
		numRecs := data.Len()
		numFieldsPerRec := recType.NumField()
		numBindArgs := numRecs * numFieldsPerRec
		args = make([]any, numBindArgs)
		for rowIndex := range data.Len() {
			if data.Index(0).Kind() == reflect.Pointer {
				rec = data.Index(rowIndex).Elem() // Cur slice elem is struct pointer, get arg val from ref-element
			} else {
				rec = data.Index(rowIndex) // Cur slice elem is struct, can get arg val directly
			}
			for fieldIndex := 0; fieldIndex < numFieldsPerRec; fieldIndex++ {
				argIndex += 1
				args[argIndex] = rec.Field(fieldIndex).Interface()
			}
		}
		return args
	} else { // Single-row INSERT: inserter.Data must be a struct pointer or struct (otherwise reflect will panic)
		if data.Kind() == reflect.Pointer { // Row information via struct pointer
			recType = data.Elem().Type()
			rec = data.Elem()
		} else { // Row information via struct
			recType = data.Type()
			rec = data
		}
		args = make([]any, recType.NumField())
		for i := 0; i < recType.NumField(); i++ {
			args[i] = rec.Field(i).Interface()
		}
		return args
	}
}
