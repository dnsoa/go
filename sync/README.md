# sync - 同步工具扩展库

本包提供了 Go 标准库 `sync` 包之外的有用同步原语，多数源自 gRPC 项目的实践。

> **注意**:
> - Go 1.21+ 标准库已内置 `sync.OnceFunc` 和 `sync.OnceValue`
> - Go 1.19+ 标准库已内置类型安全的原子操作（`atomic.Int64`、`atomic.Uint64` 等）
> 以上功能本包不再提供重复实现。

## 目录

- [Pool](#pool-对象池)
- [OnceInDuration](#onceinduration-时间窗口限流)
- [OnceEvent](#onceevent-一次性事件)
- [Unbounded](#unbounded-无界缓冲区)
- [PubSub](#pubsub-发布订阅)
- [CallbackSerializer](#callbackserializer-回调序列化)

---

## Pool - 对象池

类型安全的泛型对象池，支持自动 Reset 重置。

### 功能特点

- 泛型设计，类型安全
- 自动调用 `Reset()` 方法重置对象
- 零内存分配（可复用对象）

### 应用场景

```go
// bytes.Buffer 对象池
var bufPool = sync.NewPool(func() *bytes.Buffer {
    return &bytes.Buffer{}
})

// 使用
buf := bufPool.Get()
buf.WriteString("data")
// ... 使用完毕
bufPool.Put(buf)  // 自动 Reset
```

**典型场景**：
- 频繁创建/销毁的临时对象（如 bytes.Buffer）
- 减少 GC 压力
- 高并发下的对象复用

---

## OnceInDuration - 时间窗口限流

在指定时间窗口内只允许执行一次。

### 功能特点

- 基于原子操作实现，高效
- 自动处理定时器生命周期
- 支持 Reset 强制重置

### 应用场景

```go
var throttle sync.OnceInDuration

// 5秒内只执行一次
throttle.Do(5*time.Second, func() {
    sendAlert("系统异常")
})
```

**典型场景**：
- 告警去重/限流
- 防抖（debounce）
- 缓存刷新控制
- 限流发送通知

---

## OnceEvent - 一次性事件

表示一个只会发生一次的事件，可通过 channel 等待。

### 功能特点

- 提供可等待的 channel
- `HasFired()` 查询状态
- `Fire()` 返回是否是首次触发

### 应用场景

```go
var shutdown = sync.NewOnceEvent()

// 等待关闭信号
go func() {
    <-shutdown.Done()
    cleanup()
}()

// 触发关闭
shutdown.Fire()
```

**典型场景**：
- 优雅关闭通知
- 初始化完成信号
- 一次性事件广播

---

## Unbounded - 无界缓冲区

无界队列，生产者不会阻塞（直到内存耗尽）。

### 功能特点

- 泛型设计，类型安全
- Put 永不阻塞（除锁竞争）
- 优雅关闭支持
- 使用索引优化 backlog，避免频繁内存分配

### 应用场景

```go
buf := sync.NewUnbounded[Update]()

// 生产者
buf.Put(update)

// 消费者
for update := range buf.Get() {
    process(update)
    buf.Load()  // 加载下一个
}
```

**典型场景**：
- 生产者-消费者场景
- 事件流缓冲
- gRPC 内部使用（更新传递）

---

## PubSub - 发布订阅

类型安全的简单发布订阅系统。

### 功能特点

- 泛型设计，消息类型安全
- 保证消息顺序
- 新订阅者自动接收最新消息
- 优雅关闭

### 应用场景

```go
ps := sync.NewPubSub[Config](ctx)

// 订阅
type configSubscriber struct{ ch chan Config }
cancel := ps.Subscribe(&configSubscriber{ch: make(chan Config)})

// 发布
ps.Publish(Config{Debug: true})
```

**典型场景**：
- 配置变更通知
- 状态更新广播
- 事件分发

---

## CallbackSerializer - 回调序列化

保证回调按 FIFO 顺序串行执行。

### 功能特点

- 回调按提交顺序执行
- 优雅关闭，保证所有回调执行完毕
- Context 取消时停止接受新回调

### 应用场景

```go
cs := sync.NewCallbackSerializer(ctx)

// 提交回调
cs.Schedule(func(ctx context.Context) {
    // 安全地操作共享状态
})
```

**典型场景**：
- 串行化访问共享状态
- 避免锁竞争
- gRPC 内部使用（订阅通知序列化）

---

## 性能对比

| 组件 | 操作 | 时间 | 分配 |
|------|------|------|------|
| Pool | Get/Put | ~23 ns/op | 0 B/op |
| Unbounded | Sequential | ~100 ns/op | 0 B/op |
| Unbounded | Concurrent | ~195 ns/op | ~41 B/op |
| OnceInDuration | 快速路径（冷却中） | ~2 ns/op | 0 B/op |

---

## License

部分代码源自 [gRPC](https://github.com/grpc/grpc-go)，遵循 Apache License 2.0。
