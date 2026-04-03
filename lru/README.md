# lru

高性能泛型 LRU 缓存库，提供四种实现，适用于不同场景。

| 实现 | 淘汰策略 | 并发模型 | 适用场景 |
|------|----------|----------|----------|
| `SimpleLRU` | 条目数 | 单锁 | 简单场景、低并发 |
| `ShardLRU` | 条目数 | 分片锁 | 高并发、按个数淘汰 |
| `ByteLRU` | 字节数 | 单锁 | 需要按字节控制内存 |
| `ByteShardLRU` | 字节数 | 分片锁 | 高并发 + 按字节控制内存 |

## 特性

- 泛型实现，类型安全
- 分片锁减少并发争用
- 基于字节预算的淘汰策略
- 可选的淘汰回调 `onEvict`
- 过期条目批量清理 `EvictExpired`
- `string` / `[]byte` 类型自动推断 sizer

## 安装

```bash
go get github.com/dnsoa/go/lru
```

## 快速开始

### SimpleLRU — 按条目数淘汰

```go
lru := lru.NewSimpleLRU[string, int](1000, nil)
lru.Set("key", 42)
v, ok := lru.Get("key") // v=42, ok=true
```

### ShardLRU — 分片 + 按条目数淘汰

```go
lru := lru.NewShardLRU[int, string](
    lru.WithShardCount[int, string](32),
    lru.WithCapacity[int, string](10000),
    lru.WithLRUOnEvict[int, string](func(k int, v string) {
        log.Printf("evicted: %d", k)
    }),
)
lru.Set(1, "hello")
```

### ByteLRU — 按字节数淘汰

```go
lru, _ := lru.NewByteLRU[string, string](
    lru.WithMaxBytes[string, string](64 * 1024 * 1024), // 64MB
)
// string 类型自动使用 len() 作为 sizer
lru.Set("key", "value")

// 自定义 sizer
lru, _ = lru.NewByteLRU[string, MyValue](
    lru.WithMaxBytes[string, MyValue](1024),
    lru.WithSizer[string, MyValue](func(v MyValue) int {
        return v.Size()
    }),
)
```

### ByteShardLRU — 分片 + 按字节数淘汰

```go
lru, _ := lru.NewByteShardLRU[string, []byte](
    lru.WithTotalMaxBytes[string, []byte](256 * 1024 * 1024), // 256MB 总量
    // []byte 类型自动使用 len() 作为 sizer
)

// 或指定每分片上限
lru, _ = lru.NewByteShardLRU[string, []byte](
    lru.WithShardMaxBytes[string, []byte](8 * 1024 * 1024),   // 每分片 8MB
    lru.WithShardCountByte[string, []byte](64),                // 64 分片
    lru.WithShardByteOnEvict[string, []byte](func(k string, v []byte) {
        log.Printf("evicted: %s", k)
    }),
)
```

### 过期清理

```go
type entry struct {
    value     string
    expiredAt time.Time
}

lru, _ := lru.NewByteShardLRU[string, entry](
    lru.WithTotalMaxBytes[string, entry](64 * 1024 * 1024),
    lru.WithShardByteSizer[string, entry](func(e entry) int { return len(e.value) }),
)

// 定期清理过期条目
removed := lru.EvictExpired(func(e entry) bool {
    return time.Now().After(e.expiredAt)
})
```

## API

所有实现都提供以下基本操作：

| 方法 | 说明 |
|------|------|
| `Get(key) (V, bool)` | 获取值并提升到最近使用 |
| `Set(key, value)` | 设置键值对 |
| `Delete(key) bool` | 删除条目 |
| `Contains(key) bool` | 检查是否存在（不更新顺序） |
| `Len() int` | 条目数 |
| `Clear()` | 清空缓存（触发 onEvict） |

扩展方法（因实现而异）：

| 方法 | SimpleLRU | ShardLRU | ByteLRU | ByteShardLRU |
|------|-----------|----------|---------|--------------|
| `Stats()` | - | hitRate, shardLoad | count, curBytes, maxBytes | count, curBytes, maxBytes |
| `EvictExpired(fn)` | - | - | ✓ | ✓ |
| `OnEvict() / SetOnEvict()` | ✓ | - | ✓ | - |

## 选项参考

### ShardLRU

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithShardCount(n)` | 32 | 分片数，自动取整为 2 的幂 |
| `WithCapacity(n)` | 4096 | 总容量，平均分配给各分片 |
| `WithLRUOnEvict(fn)` | nil | 淘汰回调 |

### ByteLRU

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithMaxBytes(n)` | 必填 | 最大字节数 |
| `WithSizer(fn)` | `len()` for string/[]byte | 字节大小计算函数 |
| `WithByteOnEvict(fn)` | nil | 淘汰回调 |

### ByteShardLRU

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithTotalMaxBytes(n)` | - | 总字节数（与 WithShardMaxBytes 二选一） |
| `WithShardMaxBytes(n)` | 64MB/分片 | 每分片最大字节数 |
| `WithShardCountByte(n)` | 32 | 分片数 |
| `WithShardByteSizer(fn)` | `len()` for string/[]byte | 字节大小计算函数 |
| `WithShardByteOnEvict(fn)` | nil | 淘汰回调 |

## 基准测试

Intel i9-14900HX, 32 线程：

```
ShardLRU_Get        25 ns/op     0 B/op     0 allocs/op
ShardLRU_Set       105 ns/op     0 B/op     0 allocs/op
ShardLRU_Mixed      57 ns/op     3 B/op     0 allocs/op

ByteShardLRU_Get    26 ns/op     0 B/op     0 allocs/op
ByteShardLRU_Set   195 ns/op    48 B/op     1 allocs/op
```

## License

MIT
