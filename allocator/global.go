package allocator

import "sync/atomic"

var defaultAllocator atomic.Pointer[Allocator]

func init() {
	defaultAllocator.Store(New())
}

// Default 返回当前全局默认分配器实现。
func Default() *Allocator {
	return defaultAllocator.Load()
}

// SetDefault 原子替换全局默认分配器，并返回替换前的实现。
// 建议仅在初始化阶段、首次使用包级 Get 或 Release 前调用。
// 传入 nil 时保持当前默认实现不变。
func SetDefault(pool *Allocator) *Allocator {
	prev := Default()
	if pool == nil {
		return prev
	}
	defaultAllocator.Store(pool)
	return prev
}

// ResetDefault 将全局默认分配器重置为一个新的 *Allocator。
func ResetDefault() *Allocator {
	return SetDefault(New())
}

// Release 回收通过全局接口获取的 Buffer，并返回底层分配器的错误。
func Release(buf *Buffer) error {
	return Default().Release(buf)
}

// Get 使用当前全局默认分配器获取 Buffer。
func Get(size int) *Buffer {
	return Default().Get(size)
}
