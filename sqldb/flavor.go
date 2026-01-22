package sqldb

import (
	"strings"
	"time"
)

func nanotime() int64 {
	return time.Since(globalStart).Nanoseconds()
}

var globalStart = time.Now()

// Since returns the amount of time that has elapsed since t. t should be
// the result of a call to Now() on the same machine.
func timeSince(t int64) time.Duration {
	return time.Duration(nanotime() - t)
}

// Supported drivers.
const (
	invalidFlavor Flavor = iota

	MySQL
	PostgreSQL
	SQLite
)

// Flavor is the flag to control the format of compiled sql.
type Flavor int

// String returns the name of f.
func (f Flavor) String() string {
	switch f {
	case MySQL:
		return "MySQL"
	case PostgreSQL:
		return "PostgreSQL"
	case SQLite:
		return "SQLite"
	}

	return "<invalid>"
}

func (f Flavor) tableQuote(prefix string, table string) string {
	tableQuote := "`"
	switch f {
	case PostgreSQL:
		tableQuote = "\""
	}

	if strings.Contains(table, ".") {
		return tableQuote + strings.ReplaceAll(table, ".", tableQuote+"."+tableQuote) + tableQuote
	}

	return tableQuote + prefix + table + tableQuote
}

func (f Flavor) columnQuote(column string) string {
	columnQuote := ""
	switch f {
	case PostgreSQL:
		columnQuote = "\""
	default:
		columnQuote = "`"
	}
	if column == "*" {
		return "*"
	}
	if strings.ContainsRune(column, '.') {
		if strings.ContainsRune(column, '*') {
			return columnQuote + strings.ReplaceAll(column, ".", columnQuote+".")
		}
		return columnQuote + strings.ReplaceAll(column, ".", columnQuote+"."+columnQuote) + columnQuote
	} else if strings.Contains(column, "(") || strings.Contains(column, " ") {
		return column
	}

	return columnQuote + column + columnQuote
}
