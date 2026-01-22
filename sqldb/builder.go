package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

//https://github.com/arthurkushman/buildsqlx/blob/master/builder.go#L37

type builder struct {
	flavor          Flavor
	db              ExecerAndQueryer
	table           string
	columns         []string
	whereBindings   []map[string]any
	orderBy         []map[string]string
	groupBy         string
	startBindingsAt int
	offset          int64
	limit           int64
}

func newBuilder(flavor Flavor, db ExecerAndQueryer) *builder {
	return &builder{
		flavor:  flavor,
		db:      db,
		columns: []string{"*"},
	}
}

func (b *builder) Table(table string) *builder {
	b.table = table
	return b
}

func (b *builder) Select(columns ...string) *builder {
	b.columns = columns
	return b
}

func (b *builder) Where(column, operator string, value any) *builder {
	prefix := ""
	if len(b.whereBindings) > 0 {
		prefix = "AND"
	}
	return b.buildWhere(prefix, column, operator, value)
}

func (b *builder) Count() (int, error) {
	b1 := b.Clone()
	defer b1.Reset()
	b1.columns = []string{"COUNT(*)"}
	query, args := b1.buildSelect(), prepareValues(b1.whereBindings)
	var count int
	row := b1.db.QueryRow(query, args...)
	err := row.Scan(&count)
	return count, err
}

func (b *builder) buildWhere(prefix, operand, operator string, val any) *builder {
	if prefix != "" {
		prefix = " " + prefix + " "
	}
	operand = b.flavor.columnQuote(operand)
	b.whereBindings = append(b.whereBindings, map[string]any{prefix + operand + " " + operator: val})
	return b
}

func (b *builder) OrderBy(column, direction string) *builder {
	b.orderBy = append(b.orderBy, map[string]string{column: direction})
	return b
}

func (b *builder) GroupBy(expr string) *builder {
	b.groupBy = expr
	return b
}

func (b *builder) Offset(offset int64) *builder {
	b.offset = offset
	return b
}

func (b *builder) Limit(limit int64) *builder {
	b.limit = limit
	return b
}

func (b *builder) buildSelect() string {
	cols := make([]string, 0, len(b.columns))
	for _, c := range b.columns {
		if c == "*" {
			cols = append(cols, "*")
			continue
		}
		cols = append(cols, b.flavor.columnQuote(c))
	}
	query := `SELECT ` + strings.Join(cols, `, `) + ` FROM ` + b.flavor.tableQuote("", b.table) + ``

	return query + b.buildClauses()
}

// builds query string clauses
func (b *builder) buildClauses() string {
	clauses := ""
	// for _, j := range b.join {
	// 	clauses += j
	// }

	// build where clause
	if len(b.whereBindings) > 0 {
		clauses += composeWhere(b.whereBindings, b.startBindingsAt)
	}

	if b.groupBy != "" {
		clauses += " GROUP BY " + b.groupBy
	}

	// if r.having != "" {
	// 	clauses += " HAVING " + r.having
	// }

	clauses += composeOrderBy(b.orderBy)

	if b.limit > 0 {
		clauses += " LIMIT " + strconv.FormatInt(b.limit, 10)
	}

	if b.offset > 0 {
		clauses += " OFFSET " + strconv.FormatInt(b.offset, 10)
	}

	return clauses
}

// composes WHERE clause string for particular query stmt
func composeWhere(whereBindings []map[string]any, startedAt int) string {
	where := " WHERE "
	for _, m := range whereBindings {
		for k, v := range m {
			// operand >= $i
			switch vi := v.(type) {
			case []any:
				dataLen := len(vi)
				where += k + " (" + strings.Repeat("?,", dataLen)[:dataLen*2-1] + ")"
			default:
				// if strings.Contains(k, sqlOperatorIs) || strings.Contains(k, sqlOperatorBetween) {
				// 	where += k + " " + vi.(string)
				// 	break
				// }

				where += k + " ?"
			}
		}
	}
	return where
}

// composers ORDER BY clause string for particular query stmt
func composeOrderBy(orderBy []map[string]string) string {
	if len(orderBy) > 0 {
		orderStr := ""
		for _, m := range orderBy {
			for field, direct := range m {
				if orderStr == "" {
					orderStr = " ORDER BY " + field + " " + direct
				} else {
					orderStr += ", " + field + " " + direct
				}
			}
		}
		return orderStr
	}
	return ""
}
func prepareValues(values []map[string]any) []any {
	var vls []any
	for _, v := range values {
		_, vals, _ := prepareBindings(v)
		vls = append(vls, vals...)
	}
	return vls
}
func prepareValue(value any) []any {
	var values []any
	switch v := value.(type) {
	case string:
		values = append(values, v)
	case int:
		values = append(values, v)
	case float64:
		values = append(values, v)
	case int64:
		values = append(values, v)
	case uint64:
		values = append(values, v)
	case []any:
		for _, vi := range v {
			values = append(values, prepareValue(vi)...)
		}
	case nil:
		values = append(values, nil)
	}

	return values
}

// prepareBindings prepares slices to split in favor of INSERT sql statement
func prepareBindings(data map[string]any) (columns []string, values []any, bindings []string) {
	i := 1
	for column, value := range data {
		// if strings.Contains(column, sqlOperatorIs) || strings.Contains(column, sqlOperatorBetween) {
		// 	continue
		// }

		columns = append(columns, column)
		pValues := prepareValue(value)
		if len(pValues) > 0 {
			values = append(values, pValues...)

			for range pValues {
				bindings = append(bindings, "?")
				i++
			}
		}
	}

	return
}

func (b *builder) Insert(data any) (sql.Result, error) {
	defer b.Reset()
	switch v := data.(type) {
	case map[string]any:
		return b.insertMap(v)
	default:
		return b.insertAny(data)
	}
}

func (b *builder) insertAny(data any) (sql.Result, error) {
	rv := reflect.ValueOf(data)
	if rv.Kind() == reflect.Slice && rv.Len() == 0 {
		return nil, fmt.Errorf("empty slice")
	}
	ins := &inserter{
		Table: b.table,
		Data:  data,
	}
	return b.db.Exec(ins.SQL(), ins.Args()...)
}

func (b *builder) insertMap(data map[string]any) (sql.Result, error) {
	columns, values, bindings := prepareBindings(data)
	for i := range columns {
		columns[i] = b.flavor.columnQuote(columns[i])
	}
	query := `INSERT INTO ` + b.flavor.tableQuote("", b.table) + ` (` + strings.Join(columns, ", ") + `) VALUES (` + strings.Join(bindings, ", ") + `)`
	return b.db.Exec(query, values...)
}

func (b *builder) Update(data any) (sql.Result, error) {
	defer b.Reset()
	switch v := data.(type) {
	case map[string]any:
		return b.updateMap(v)
	default:
		return nil, fmt.Errorf("unsupported type %T", v)
	}
}

func (b *builder) updateMap(data map[string]any) (sql.Result, error) {
	dataLen := len(data)
	if dataLen == 0 {
		return nil, fmt.Errorf("no data to update")
	}
	if len(b.whereBindings) == 0 {
		return nil, fmt.Errorf("missing WHERE clause")
	}
	fields := make([]string, 0, dataLen)
	values := make([]any, 0, dataLen)
	for k, v := range data {
		fields = append(fields, fmt.Sprintf("%s=?", b.flavor.columnQuote(k)))
		values = append(values, v)
	}
	whereClause, whereArgs := composeWhere(b.whereBindings, 1), prepareValues(b.whereBindings)

	query := "UPDATE " + b.flavor.tableQuote("", b.table) + " SET " + strings.Join(fields, ", ") + whereClause
	values = append(values, whereArgs...)

	return b.db.Exec(query, values...)
}

func (b *builder) Scan(dest any) error {
	defer b.Reset()
	query, args := b.buildSelect(), prepareValues(b.whereBindings)
	return ScanContext(context.Background(), b.db, dest, query, args...)
}

func (b *builder) Reset() {
	b.table = ""
	b.columns = []string{"*"}
	b.whereBindings = make([]map[string]any, 0)
	b.orderBy = make([]map[string]string, 0)
	b.groupBy = ""
	b.offset = 0
	b.limit = 0
}

func (b *builder) Clone() *builder {
	return &builder{
		flavor:        b.flavor,
		db:            b.db,
		table:         b.table,
		columns:       b.columns,
		whereBindings: b.whereBindings,
		orderBy:       b.orderBy,
		groupBy:       b.groupBy,
		offset:        b.offset,
		limit:         b.limit,
	}
}
