# allocator - 内存池分配器

高效的字节切片内存池，减少频繁的内存分配和 GC 压力。

## 功能特点

- **分级池管理**: 使用 `sync.Pool` 管理不同 2^n 大小的 buffer
- **自动扩容**: 请求大小自动向上取整到 2^n
- **可选清零**: 回收时可选择是否清零数据
- **内存统计**: 实时查看池中占用的内存大小
- **自动清理**: 支持定期清理池中的旧对象

## 安装

```bash
go get github.com/dnsoa/go/allocator
```

## 基本使用

```go
import "github.com/dnsoa/go/allocator"

alloc := allocator.New()

// 获取 buffer
buf := alloc.Get(1024) // 返回 *Buffer，实际容量是 1024 (2^10)

// 使用 buffer
copy(*buf, someData)

// 使用完后回收
alloc.Put(buf)
```

## 配置选项

```go
// 创建时启用回收时清零（适合处理敏感数据）
alloc := allocator.New(allocator.WithZeroOnPut(true))

// 启用自动清理（每小时清理一次）
alloc := allocator.New(allocator.WithAutoClean(time.Hour))
defer alloc.StopAutoClean()
```

## Buffer 类型

`Buffer` 是 `[]byte` 的别名，提供了丰富的操作方法：

```go
var buf allocator.Buffer

// 追加各种类型
buf = buf.AppendString("hello")
buf = buf.AppendInt(42)
buf = buf.AppendHex(data)
buf = buf.AppendBase64(data)

// 实现了 io.Writer 接口
buf.Write([]byte("more data"))

// 零拷贝转换为 string
str := buf.String() // 使用 unsafe 优化，无内存分配

// 重置复用
buf = buf.Reset()
```

## 内存限制

默认配置：

| 配置 | 值 |
|------|-----|
| 最大大小 | 256KB (2^18) |
| 池化大小 | 1, 2, 4, ..., 256KB |

超过最大大小的请求会直接分配，不会被池化。

## API 设计说明

**重要**: `Get()` 返回 `*Buffer` 而不是 `[]byte`，这是为了：

1. 支持正确回收 - 只有 `*Buffer` 可以通过 `Put()` 回收
2. 避免混淆 - 明确区分可回收和不可回收的内存

如果需要 `[]byte`，解引用即可：
```go
buf := alloc.Get(1024)
bytes := *buf  // 获取 []byte
// ... 使用 bytes
alloc.Put(buf) // 回收 *Buffer
```

## 性能对比

| 操作 | 时间 | 分配 |
|------|------|------|
| Get(4KB) + Put | ~50 ns/op | 0 B/op |
| Buffer.AppendString | ~20 ns/op | 0 B/op |
| Buffer.String | ~0 ns/op | 0 B/op (零拷贝) |

## License

MIT License，源自 [xtaci/kcp-go](https://github.com/xtaci/kcp-go) 项目。
