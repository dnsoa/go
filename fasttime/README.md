# fasttime

高性能时间缓存库，通过后台 goroutine 定期更新时间戳来减少系统调用开销，适用于高频时间获取场景。

## 特性

- 通过 `atomic.Int64` 缓存时间，读取零锁、零分配
- 默认 200ms 更新间隔，可配置高精度模式（10ms）
- 全部返回 UTC 时间
- 内置 XID 生成器（兼容 [rs/xid](https://github.com/rs/xid) 规范）

## 安装

```bash
go get github.com/dnsoa/go/fasttime
```

## 使用

### 时间获取

```go
package main

import (
    "fmt"
    "github.com/dnsoa/go/fasttime"
)

func main() {
    // 替代 time.Now()，性能更高
    now := fasttime.Now()

    // 直接获取 Unix 时间戳（纳秒/秒）
    nano := fasttime.UnixNano()
    sec  := fasttime.UnixTime()

    // 按天/小时聚合
    day  := fasttime.UnixDate()
    hour := fasttime.UnixHour()

    // 时间差计算
    elapsed := fasttime.Since(startTime)
    remain  := fasttime.Until(deadline)
}
```

### XID 生成

```go
// 生成全局唯一 ID（12 字节，20 字符 base32 表示）
id := fasttime.NewXID()
fmt.Println(id.String()) // e.g. "bm55gpjkd48c2qe6l6d0"

// 从时间戳生成
id = fasttime.NewXIDWithTime(time.Now().Unix())

// 解析
id, err := fasttime.ParseXID("bm55gpjkd48c2qe6l6d0")

// 提取字段
id.Time()    // time.Time
id.Machine() // 3 字节机器标识
id.Pid()     // 进程 ID
id.Counter() // 计数器

// 支持 JSON/Text 序列化
data, _ := json.Marshal(id)
json.Unmarshal(data, &id)
```

### 高精度模式

设置环境变量启用 10ms 更新间隔：

```bash
export FASTTIME_HIGH_PRECISION=true
```

## XID 结构

```
| 4 字节时间戳 | 3 字节机器 ID | 2 字节 PID | 3 字节计数器 |
```

- **时间戳**：Unix 秒级时间（大端序）
- **机器 ID**：主机名哈希，获取失败时使用随机字节
- **PID**：进程 ID
- **计数器**：随机初始值，原子递增，并发安全

## 精度说明

| 模式 | 更新间隔 | 适用场景 |
|---|---|---|
| 默认 | 200ms | 日志、监控、ID 生成等毫秒级精度无需保证的场景 |
| 高精度 | 10ms | 对时间精度有更高要求的场景 |

返回的时间可能比实际时间**滞后**最多一个更新间隔。

## 性能

相比标准库 `time.Now()`，避免了每次调用的系统调用开销：

```
BenchmarkUnixTimestamp   →  ~1ns/op   (fasttime.UnixTime)
BenchmarkTimeNowUnix     →  ~25ns/op  (time.Now().Unix())
BenchmarkUnixNano        →  ~1ns/op   (fasttime.UnixNano)
BenchmarkXID             →  ~3ns/op   (NewXID, 使用缓存时间)
```
