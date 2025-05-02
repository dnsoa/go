package sqldb

import (
	"context"
	"database/sql"
	"fmt"
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
	return b.buildWhere("", column, operator, value)
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

func (r *builder) buildSelect() string {
	query := `SELECT ` + strings.Join(r.columns, `, `) + ` FROM ` + r.table + ``

	return query + r.buildClauses()
}

// builds query string clauses
func (r *builder) buildClauses() string {
	clauses := ""
	// for _, j := range r.join {
	// 	clauses += j
	// }

	// build where clause
	if len(r.whereBindings) > 0 {
		clauses += composeWhere(r.whereBindings, r.startBindingsAt)
	}

	if r.groupBy != "" {
		clauses += " GROUP BY " + r.groupBy
	}

	// if r.having != "" {
	// 	clauses += " HAVING " + r.having
	// }

	clauses += composeOrderBy(r.orderBy)

	if r.limit > 0 {
		clauses += " LIMIT " + strconv.FormatInt(r.limit, 10)
	}

	if r.offset > 0 {
		clauses += " OFFSET " + strconv.FormatInt(r.offset, 10)
	}

	return clauses
}

// composes WHERE clause string for particular query stmt
func composeWhere(whereBindings []map[string]any, startedAt int) string {
	where := " WHERE "
	i := startedAt
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
				i++
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
		values = append(values, strconv.FormatInt(int64(v), 10))
	case float64:
		values = append(values, fmt.Sprintf("%g", v))
	case int64:
		values = append(values, strconv.FormatInt(v, 10))
	case uint64:
		values = append(values, strconv.FormatUint(v, 10))
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
	ins := &inserter{
		Table: b.table,
		Data:  data,
	}
	return b.db.Exec(ins.SQL(), ins.Args()...)
}

func (b *builder) insertMap(data map[string]any) (sql.Result, error) {
	columns, values, bindings := prepareBindings(data)
	query := `INSERT INTO ` + b.table + ` (` + strings.Join(columns, ", ") + `) VALUES (` + strings.Join(bindings, ", ") + `)`
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
