# sqldb

`sqldb` 是对 `database/sql` 的轻量封装，提供：

- 简单的链式查询构建（`Table/Where/OrderBy/Limit/Offset`）
- 通用 `Scan`（结构体、结构体切片、基础类型切片）
- 跨方言占位符转换（MySQL/SQLite 的 `?`、PostgreSQL 的 `$1...`）
- 可选 SQL 调试日志与 trace 输出
- 基于 `fs.FS` 的数据库迁移（`MigrateUp/MigrateDown/MigrateTo`）

## 支持数据库

- MySQL（driver: `mysql`, `nrmysql`）
- PostgreSQL（driver: `postgres`, `pgx` 等）
- SQLite（driver: `sqlite3`, `sqlite`）

## 快速开始

```go
db, err := sqldb.Open("sqlite3", ":memory:",
	sqldb.WithDebug(true),
	sqldb.WithTraceSQL(true),
)
if err != nil {
	panic(err)
}
defer db.Close()

_, _ = db.Exec("CREATE TABLE users (name TEXT, age INTEGER)")
_, _ = db.Table("users").Insert(map[string]any{"name": "alice", "age": 18})
```

## 查询与扫描

### 1) 扫描到结构体

```go
type User struct {
	Name string `db:"name"`
	Age  int    `db:"age"`
}

var u User
err := db.QueryScan(&u, "SELECT name, age FROM users WHERE name = ?", "alice")
```

### 2) 扫描到切片

```go
var users []User
err := db.QueryScan(&users, "SELECT name, age FROM users ORDER BY age DESC")

var ages []int
err = db.QueryScan(&ages, "SELECT age FROM users ORDER BY age DESC")
```

### 3) 使用 Builder

```go
var users []User
err := db.Table("users").
	Select("name", "age").
	Where("age", ">", 10).
	OrderBy("age", "DESC").
	Limit(20).
	Offset(0).
	Scan(&users)
```

## 事务

```go
err := db.Transaction(func(tx *sqldb.Tx) error {
	if _, err := tx.Exec("UPDATE users SET age = age + 1 WHERE name = ?", "alice"); err != nil {
		return err
	}
	return nil
})
```

## 迁移（migrate）

`sqldb` 内置了迁移引擎，支持基于 `fs.FS` 的迁移文件执行。

迁移文件命名示例：

- `1-init.up.sql`
- `1-init.down.sql`
- `2-user.up.sql`
- `2-user.down.sql`

```go
import (
	"context"
	"os"

	"github.com/dnsoa/go/sqldb"
)

db, _ := sqldb.Open("sqlite3", "app.db")
defer db.Close()

migrations := os.DirFS("migrations")

// 一键升级到最新版本
_ = db.MigrateUp(context.Background(), migrations)

// 迁移到指定版本
_ = db.MigrateTo(context.Background(), migrations, "1-init")

// 回滚到初始状态
_ = db.MigrateDown(context.Background(), migrations)
```

也可以创建可复用的迁移器并自定义迁移表名与回调：

```go
m, err := db.NewMigrator(
	migrations,
	sqldb.WithMigrationTable("schema_migrations"),
	sqldb.WithMigrationBefore(func(ctx context.Context, tx *sql.Tx, version string) error {
		return nil
	}),
	sqldb.WithMigrationAfter(func(ctx context.Context, tx *sql.Tx, version string) error {
		return nil
	}),
)
if err != nil {
	panic(err)
}

_ = m.Up(context.Background())
```

## 扫描字段映射规则

按如下优先级匹配列名：

1. `sql` tag
2. `db` tag
3. 字段名小写

示例：

```go
type User struct {
	Name string `sql:"name"`
	Age  int    `db:"age"`
}
```

## 调试选项

- `WithDebug(true)`：记录 SQL、参数与耗时
- `WithLog(fn)`：自定义日志函数
- `WithTraceSQL(true)`：打印展开参数后的 SQL（仅用于调试）

## 注意事项

- `Where/OrderBy/GroupBy` 的列名、操作符、排序方向是表达式输入，建议只传可信常量。
- `FormatSQL` 仅用于日志展示，不应用于执行 SQL。
- PostgreSQL 场景下内部会将 `?` 占位符转换为 `$1,$2...`。

## 结构体生成工具（可选）

可用 `tables-to-go` 从数据库结构生成模型：

```bash
go install github.com/fraenky8/tables-to-go@master
tables-to-go -v -t pg -h 127.0.0.1 -s public -d postgres -u postgres -p admin -pn models -of ./models
```