package models

import (
	"database/sql"
)

type Users struct {
	Name sql.NullString `db:"name"`
	Age  sql.NullInt64  `db:"age"`
}
