# goenv

轻量 dotenv + 环境变量读取工具：

- 从 `.env` 文件加载到进程环境变量
- 从环境变量读取并转换成常用类型（string/bool/int/duration）

## 安装

```bash
go get github.com/dnsoa/go/env
```

## 加载 `.env`

默认读取当前目录的 `.env`：

```go
package main

import (
	"log"

	"github.com/dnsoa/go/env"
)

func main() {
	if err := env.Load(); err != nil {
		log.Fatal(err)
	}
}
```

指定文件（支持多个，按顺序加载）：

```go
_ = env.Load(".env", ".env.local")
```

### 覆盖已有环境变量

`Load` 默认不会覆盖已存在的环境变量。需要覆盖时用：

```go
_ = env.Overload(".env")
```

或：

```go
_ = env.LoadWithOptions(env.LoadOptions{Overload: true}, ".env")
```

## 读取配置

```go
port, err := env.Int[int]("PORT", 3000)
if err != nil {
	// PORT 已设置但不是合法整数时返回 error
}

timeout, err := env.Duration("TIMEOUT", 5*time.Second)
```

### string

```go
name := env.String("NAME", "default")
```

注意：`Get/String` 会对值做“仅空格字符（' '）”的前后裁剪（trim）。如果你需要完全保留原始值，使用 `GetRaw`：

```go
raw := env.GetRaw("NAME", "")
```

### bool

```go
debug := env.Bool("DEBUG", false)
```

如需严格解析（支持 `true/false/1/0` 等，解析失败返回 error），用 `ParseBool`：

```go
debug, err := env.ParseBool("DEBUG", false)
```

## Marshal

`Marshal` 会把当前进程的环境变量输出成 dotenv 格式字符串（按 key 排序）。

```go
s, _ := env.Marshal()
```

如需避免把纯数字输出成不带引号（例如保留前导零），使用 `MarshalWithOptions`：

```go
s, _ := env.MarshalWithOptions(env.MarshalOptions{QuoteAll: true})
```
