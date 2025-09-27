package sync

import (
	"sync/atomic"
	"time"
)

// 文件说明: OnceInDuration 在给定时长内只允许函数执行一次，基于原子操作与可复用 timer 实现，适用于限流或去抖场景。
//
// 用法示例:
//   var o sync.OnceInDuration
//   o.Do(time.Second*5, func(){ /* 执行逻辑 */ })
//
// 注意:
// - duration 必须大于 0
// - f 不能为 nil

type OnceInDuration struct {
	done  uint32
	timer atomic.Pointer[time.Timer]
	// 引入代际 token，避免旧定时器在新一轮执行期间将状态提前清零
	gen atomic.Uint64 // 当前代际计数，每次成功执行前递增
}

func (o *OnceInDuration) Do(duration time.Duration, f func()) {
	if duration <= 0 {
		panic("duration must be greater than zero")
	}
	if f == nil {
		panic("nil function provided")
	}

	// 快速路径：已经在冷却中，直接返回
	if atomic.LoadUint32(&o.done) == 1 {
		return
	}
	// 尝试设置为运行态（从 0 -> 1），只有第一个成功的 goroutine 会继续执行
	if !atomic.CompareAndSwapUint32(&o.done, 0, 1) {
		return
	}

	// 进入新一轮执行：递增代际，失效旧的定时器触发
	curr := o.gen.Add(1)

	// 冷却周期从 f 返回时开始。使用新的 timer，并原子替换旧的 timer，停止旧的以避免泄漏。
	defer func() {
		// 在闭包中捕获本轮代际
		myGen := curr
		newT := time.AfterFunc(duration, func() {
			if o.gen.Load() == myGen {
				atomic.StoreUint32(&o.done, 0)
			}
		})
		// 原子交换并停止旧的 timer（若有）
		if old := o.timer.Swap(newT); old != nil {
			_ = old.Stop()
		}
	}()

	f()
}

// Reset 强制清零并停止当前定时器（若存在）。
func (o *OnceInDuration) Reset() {
	atomic.StoreUint32(&o.done, 0)
	if t := o.timer.Swap(nil); t != nil {
		_ = t.Stop()
	}
}
