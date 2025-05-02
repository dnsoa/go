package test

import (
	"context"
	"database/sql"
	"strconv"
	"testing"

	"test/models"

	"github.com/dnsoa/go/sqldb"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func testInsert(t *testing.T, db *sqldb.DB) {
	r := require.New(t)
	_, err := db.Exec("DROP TABLE IF EXISTS test1")
	r.NoError(err)
	_, err = db.Exec("CREATE TABLE test1 (name TEXT, id INTEGER)")
	r.NoError(err)
	t.Run("insert map", func(t *testing.T) {
		res, err := db.Table("test1").Insert(map[string]any{
			"name": "foo",
			"id":   20,
		})
		r.NoError(err)
		r.NotNil(res)
		affected, err := res.RowsAffected()
		r.NoError(err)
		r.Equal(int64(1), affected)
	})
	t.Run("insert struct", func(t *testing.T) {
		res, err := db.Table("test1").Insert(models.Test{
			Name: "foo1",
			ID:   20,
		})
		r.NoError(err)
		r.NotNil(res)
		affected, err := res.RowsAffected()
		r.NoError(err)
		r.Equal(int64(1), affected)

		res, err = db.Table("test1").Insert(&models.Test{
			Name: "foo2",
			ID:   30,
		})
		r.NoError(err)
		r.NotNil(res)
		affected, err = res.RowsAffected()
		r.NoError(err)
		r.Equal(int64(1), affected)
	})
	t.Run("insert slice", func(t *testing.T) {
		res, err := db.Table("test1").Insert([]models.Test{
			{Name: "foo3", ID: 40},
			{Name: "foo4", ID: 50},
		})
		r.NoError(err)
		r.NotNil(res)
		affected, err := res.RowsAffected()
		r.NoError(err)
		r.Equal(int64(2), affected)
	})
}

func testUpdate(t *testing.T, db *sqldb.DB) {
	r := require.New(t)
	updateData := map[string]any{
		"age": 55,
	}
	res, err := db.Table("users").Where("name", "=", "foo").Update(updateData)
	r.NoError(err)
	affected, err := res.RowsAffected()
	r.NoError(err)
	r.Equal(int64(1), affected)

	rows, err := db.Query("SELECT * FROM users where name = ?", "foo")
	r.NoError(err)
	defer rows.Close()
	r.True(rows.Next())
	var name string
	var age int
	err = rows.Scan(&name, &age)
	r.NoError(err)
	r.Equal("foo", name)
	r.Equal(55, age)
}

func testSelect(t *testing.T, db *sqldb.DB) {
	r := require.New(t)
	t.Run("Scan slice model", func(t *testing.T) {
		var models []models.Users
		err := db.QueryScan(&models, "SELECT * FROM users order by age desc")
		r.NoError(err)
		r.Len(models, 4)
		r.Equal("foo", models[0].Name.String)
		r.Equal(int64(55), models[0].Age.Int64)
	})
	t.Run("Scan slice tmp struct", func(t *testing.T) {
		var ages []struct {
			Age int `db:"age"`
		}
		err := db.QueryScan(&ages, "SELECT age FROM users order by age desc")
		r.NoError(err)
		r.Len(ages, 4)
		r.Equal(55, ages[0].Age)
	})
	t.Run("Scan slice int", func(t *testing.T) {
		var ages []int
		err := db.QueryScan(&ages, "SELECT age FROM users order by age desc")
		r.NoError(err)
		r.Len(ages, 4)
		r.Equal(55, ages[0])
	})
	t.Run("Scan slice string", func(t *testing.T) {
		var names []string
		err := db.QueryScan(&names, "SELECT name FROM users order by age desc")
		r.NoError(err)
		r.Len(names, 4)
		r.Equal("foo", names[0])
	})
}

func testQuery(t *testing.T, db *sqldb.DB) {
	r := require.New(t)
	rows, err := db.Query("SELECT * FROM users where name = ?", "foo")
	r.NoError(err)
	defer rows.Close()
	r.True(rows.Next())
	var name string
	var age int
	err = rows.Scan(&name, &age)
	r.NoError(err)
	r.Equal("foo", name)
	r.Equal(10, age)
}

func testQueryContext(t *testing.T, db *sqldb.DB, ctx context.Context) {
	r := require.New(t)
	rows, err := db.QueryContext(ctx, "SELECT * FROM users where name = ?", "foo")
	r.NoError(err)
	defer rows.Close()
	r.True(rows.Next())
	var name string
	var age int
	err = rows.Scan(&name, &age)
	r.NoError(err)
	r.Equal("foo", name)
	r.Equal(10, age)
}

func testQueryRow(t *testing.T, db *sqldb.DB) {
	r := require.New(t)
	row := db.QueryRow("SELECT * FROM users where name = ?", "foo")
	var name string
	var age int
	err := row.Scan(&name, &age)
	r.NoError(err)
	r.Equal("foo", name)
	r.Equal(10, age)
}
func testQueryRowContext(t *testing.T, db *sqldb.DB, ctx context.Context) {
	r := require.New(t)
	row := db.QueryRowContext(ctx, "SELECT * FROM users where name = ?", "foo")
	var name string
	var age int
	err := row.Scan(&name, &age)
	r.NoError(err)
	r.Equal("foo", name)
	r.Equal(10, age)
}

func testExec(t *testing.T, db *sqldb.DB) {
	r := require.New(t)
	res, err := db.Exec("INSERT INTO users (name, age) VALUES (?, ?)", "luci", 25)
	r.NoError(err)
	affected, err := res.RowsAffected()
	r.NoError(err)
	r.Equal(int64(1), affected)
}

func testExecContext(t *testing.T, db *sqldb.DB, ctx context.Context) {
	r := require.New(t)
	res, err := db.ExecContext(ctx, "DELETE FROM users where name = ?", "luci")
	r.NoError(err)
	affected, err := res.RowsAffected()
	r.NoError(err)
	r.Equal(int64(1), affected)
}

func testGet(t *testing.T, db *sqldb.DB) {
	r := require.New(t)
	var age int
	err := sqldb.Scan(db, &age, "SELECT age FROM users where name = ?", "foo")
	r.NoError(err)
	r.Equal(55, age)
	var model models.Users
	err = sqldb.Scan(db, &model, "SELECT * FROM users where name = ?", "foo")
	r.NoError(err)
	r.Equal("foo", model.Name.String)
	r.Equal(int64(55), model.Age.Int64)
	err = db.Table("users").Where("name", "=", "foo").Scan(&model)
	r.NoError(err)
	r.Equal("foo", model.Name.String)
	r.Equal(int64(55), model.Age.Int64)
}

func TestPage(t *testing.T) {
	r := require.New(t)
	db, err := sqldb.Open("sqlite3", ":memory:", sqldb.WithDebug(true))
	r.NoError(err)
	_, err = db.Exec("CREATE TABLE users (name TEXT, age INTEGER)")
	r.NoError(err)
	for i := 0; i < 10; i++ {
		_, err = db.Exec("INSERT INTO users (name, age) VALUES (?, ?)", "name"+strconv.Itoa(i), i)
		r.NoError(err)
	}
	t1 := db.Table("users").Where("age", ">", 2)
	count, err := t1.Count()
	r.NoError(err)
	r.Equal(7, count)
	var users []models.Users
	err = t1.Limit(5).Offset(2).Scan(&users)
	var count2 int
	r.NoError(err)
	r.Len(users, 5)
	err = db.Table("users").Select("count(*)").Scan(&count2)
	r.NoError(err)
	r.Equal(10, count2)
	var u1 models.Users
	err = db.Table("users").Where("age", "=", 1).Scan(&u1)
	r.NoError(err)
	r.Equal("name1", u1.Name.String)
	r.Equal(int64(1), u1.Age.Int64)
	var name2 []struct {
		Name string
	}
	err = db.Table("users").Select("name").Scan(&name2)
	r.NoError(err)
	r.Len(name2, 10)
	var name3 []string
	err = db.Table("users").Select("name").Scan(&name3)
	r.NoError(err)
	r.Len(name3, 10)
	var age1 []int
	err = db.Table("users").Select("age").Where("age", ">", 7).Scan(&age1)
	r.NoError(err)
	r.Len(age1, 2)
}

func TestDB(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	opts := []sqldb.Option{
		sqldb.WithTraceSQL(true),
		sqldb.WithDebug(true),
	}
	t.Run("sqlite3", func(t *testing.T) {
		db, err := sqldb.Open("sqlite3", ":memory:", opts...)
		r.NoError(err)
		_, err = db.Exec("CREATE TABLE users (name TEXT, age INTEGER)")
		r.NoError(err)
		testInsert(t, db)
		res, err := db.Table("users").Insert(map[string]any{
			"name": "foo",
			"age":  10,
		})
		r.NoError(err)
		r.NotNil(res)
		affected, err := res.RowsAffected()
		r.NoError(err)
		r.Equal(int64(1), affected)
		res, err = db.Table("users").Insert(&models.Users{
			Name: sql.NullString{String: "foo1", Valid: true},
			Age:  sql.NullInt64{Int64: 20, Valid: true},
		})
		r.NoError(err)
		r.NotNil(res)
		affected, err = res.RowsAffected()
		r.NoError(err)
		r.Equal(int64(1), affected)
		res, err = db.Table("users").Insert([]*models.Users{
			{Name: sql.NullString{String: "foo2", Valid: true}, Age: sql.NullInt64{Int64: 30, Valid: true}},
			{Name: sql.NullString{String: "foo3", Valid: true}, Age: sql.NullInt64{Int64: 40, Valid: true}},
		})
		r.NoError(err)
		r.NotNil(res)
		affected, err = res.RowsAffected()
		r.NoError(err)
		r.Equal(int64(2), affected)
		testQuery(t, db)
		testQueryContext(t, db, ctx)
		testQueryRow(t, db)
		testQueryRowContext(t, db, ctx)
		testExec(t, db)
		testExecContext(t, db, ctx)
		testUpdate(t, db)
		testSelect(t, db)
		testGet(t, db)
	})
	t.Run("mysql", func(t *testing.T) {
		db, err := sqldb.Open("mysql", "root:admin@tcp(127.0.0.1:3306)/test")
		r.NoError(err)
		_, err = db.Exec("DROP TABLE IF EXISTS users")
		r.NoError(err)
		_, err = db.Exec("CREATE TABLE users (name TEXT, age INTEGER)")
		r.NoError(err)
		res, err := db.Table("users").Insert(map[string]any{
			"name": "foo",
			"age":  10,
		})
		r.NoError(err)
		r.NotNil(res)
		affected, err := res.RowsAffected()
		r.NoError(err)
		r.Equal(int64(1), affected)
		res, err = db.Table("users").Insert(&models.Users{
			Name: sql.NullString{String: "foo1", Valid: true},
			Age:  sql.NullInt64{Int64: 20, Valid: true},
		})
		r.NoError(err)
		r.NotNil(res)
		affected, err = res.RowsAffected()
		r.NoError(err)
		r.Equal(int64(1), affected)
		res, err = db.Table("users").Insert([]*models.Users{
			{Name: sql.NullString{String: "foo2", Valid: true}, Age: sql.NullInt64{Int64: 30, Valid: true}},
			{Name: sql.NullString{String: "foo3", Valid: true}, Age: sql.NullInt64{Int64: 40, Valid: true}},
		})
		r.NoError(err)
		r.NotNil(res)
		affected, err = res.RowsAffected()
		r.NoError(err)
		r.Equal(int64(2), affected)
		testInsert(t, db)
		testQuery(t, db)
		testQueryContext(t, db, ctx)
		testQueryRow(t, db)
		testQueryRowContext(t, db, ctx)
		testExec(t, db)
		testExecContext(t, db, ctx)
		testUpdate(t, db)
		testSelect(t, db)
		testGet(t, db)
	})
	t.Run("postgres", func(t *testing.T) {
		db, err := sqldb.Open("pgx", "user=postgres host=localhost port=5432 password=admin dbname=postgres sslmode=disable")
		r.NoError(err)
		_, err = db.Exec("DROP TABLE IF EXISTS users")
		r.NoError(err)
		_, err = db.Exec("CREATE TABLE users (name TEXT, age INTEGER)")
		r.NoError(err)
		res, err := db.Table("users").Insert(map[string]any{
			"name": "foo",
			"age":  10,
		})
		r.NoError(err)
		r.NotNil(res)
		affected, err := res.RowsAffected()
		r.NoError(err)
		r.Equal(int64(1), affected)
		res, err = db.Table("users").Insert(&models.Users{
			Name: sql.NullString{String: "foo1", Valid: true},
			Age:  sql.NullInt64{Int64: 20, Valid: true},
		})
		r.NoError(err)
		r.NotNil(res)
		affected, err = res.RowsAffected()
		r.NoError(err)
		r.Equal(int64(1), affected)
		res, err = db.Table("users").Insert([]*models.Users{
			{Name: sql.NullString{String: "foo2", Valid: true}, Age: sql.NullInt64{Int64: 30, Valid: true}},
			{Name: sql.NullString{String: "foo3", Valid: true}, Age: sql.NullInt64{Int64: 40, Valid: true}},
		})
		r.NoError(err)
		r.NotNil(res)
		affected, err = res.RowsAffected()
		r.NoError(err)
		r.Equal(int64(2), affected)
		testInsert(t, db)
		testQuery(t, db)
		testQueryContext(t, db, ctx)
		testQueryRow(t, db)
		testQueryRowContext(t, db, ctx)
		testExec(t, db)
		testExecContext(t, db, ctx)
		testUpdate(t, db)
		testSelect(t, db)
		testGet(t, db)
	})
}

func BenchmarkInsert(b *testing.B) {
	exec := func(db *sqldb.DB, b *testing.B) {
		_, err := db.Exec("DROP TABLE IF EXISTS users")
		if err != nil {
			b.Fatal(err)
		}
		_, err = db.Exec("CREATE TABLE users (name TEXT, age INTEGER)")
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err = db.Table("users").Insert(map[string]any{
				"name": "foo",
				"age":  20,
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	}
	b.Run("sqlite3", func(b *testing.B) {
		db, err := sqldb.Open("sqlite3", ":memory:")
		if err != nil {
			b.Fatal(err)
		}
		exec(db, b)
	})
	b.Run("postgres", func(b *testing.B) {
		db, err := sqldb.Open("pgx", "user=postgres password=admin host=localhost port=5432 dbname=postgres sslmode=disable")
		if err != nil {
			b.Fatal(err)
		}
		exec(db, b)
	})
	b.Run("mysql", func(b *testing.B) {
		db, err := sqldb.Open("mysql", "root:admin@tcp(127.0.0.1:3306)/test")
		if err != nil {
			b.Fatal(err)
		}
		exec(db, b)
	})
}
