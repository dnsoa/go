package lru

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSimpleLRUBasic(t *testing.T) {
	lru := NewSimpleLRU[string, int](3, nil)

	// 测试 Set 和 Get
	lru.Set("key1", 1)
	lru.Set("key2", 2)

	if v, ok := lru.Get("key1"); !ok || v != 1 {
		t.Errorf("Get key1: expected (1, true), got (%v, %v)", v, ok)
	}

	if v, ok := lru.Get("key2"); !ok || v != 2 {
		t.Errorf("Get key2: expected (2, true), got (%v, %v)", v, ok)
	}

	// 测试不存在的键
	if v, ok := lru.Get("nonexistent"); ok {
		t.Errorf("Get nonexistent: expected (0, false), got (%v, %v)", v, ok)
	}

	// 测试更新现有键
	lru.Set("key1", 100)
	if v, ok := lru.Get("key1"); !ok || v != 100 {
		t.Errorf("Get after update: expected (100, true), got (%v, %v)", v, ok)
	}

	// 测试删除
	lru.Delete("key1")
	if v, ok := lru.Get("key1"); ok {
		t.Errorf("Get after delete: expected (0, false), got (%v, %v)", v, ok)
	}

	// 测试 Contains
	if lru.Contains("key1") {
		t.Error("Contains after delete: expected false, got true")
	}

	if !lru.Contains("key2") {
		t.Error("Contains existing key: expected true, got false")
	}

	// 测试 Len
	if lru.Len() != 1 || lru.Size() != 1 {
		t.Errorf("Len: expected 1, got %d", lru.Len())
	}

	// 测试 Clear
	lru.Clear()
	if lru.Len() != 0 {
		t.Errorf("Len after clear: expected 0, got %d", lru.Len())
	}
}

func TestSimpleLRUEviction(t *testing.T) {
	evicted := []string{}
	onEvict := func(k string, v int) {
		evicted = append(evicted, k)
	}

	lru := NewSimpleLRU[string, int](3, onEvict)

	lru.Set("key1", 1)
	lru.Set("key2", 2)
	lru.Set("key3", 3)

	// 添加第4个元素，应触发淘汰
	lru.Set("key4", 4)

	// key1 应该被淘汰
	if _, ok := lru.Get("key1"); ok {
		t.Error("key1 should have been evicted")
	}

	// 验证淘汰回调被调用
	if len(evicted) != 1 || evicted[0] != "key1" {
		t.Errorf("Expected key1 to be evicted, got %v", evicted)
	}

	// 验证容量
	if lru.Len() != 3 {
		t.Errorf("Expected length 3, got %d", lru.Len())
	}
}

func TestSimpleLRULRUOrder(t *testing.T) {
	lru := NewSimpleLRU[string, int](3, nil)

	lru.Set("key1", 1)
	lru.Set("key2", 2)
	lru.Set("key3", 3)

	// 访问 key1 使其成为最近使用的
	lru.Get("key1")

	// 添加新元素，应淘汰 key2（最久未使用）
	lru.Set("key4", 4)

	if _, ok := lru.Get("key2"); ok {
		t.Error("key2 should have been evicted")
	}

	if _, ok := lru.Get("key1"); !ok {
		t.Error("key1 should exist")
	}

	if _, ok := lru.Get("key4"); !ok {
		t.Error("key4 should exist")
	}
}

func TestSimpleLRUEdgeCases(t *testing.T) {
	// 测试零容量
	lru1 := NewSimpleLRU[string, int](0, nil)
	if lru1.Capacity() != defaultCapacity {
		t.Errorf("Expected default capacity, got %d", lru1.Capacity())
	}

	// 测试 nil 指针值
	lru := NewSimpleLRU[string, *int](3, nil)
	var nilPtr *int
	lru.Set("nil", nilPtr)
	if v, ok := lru.Get("nil"); !ok || v != nilPtr {
		t.Errorf("Get nil value: expected (%v, true), got (%v, %v)", nilPtr, v, ok)
	}

	// 测试空字符串键
	var val = 42
	lru.Set("", &val)
	if v, ok := lru.Get(""); !ok || *v != 42 {
		t.Errorf("Get empty key: expected (42, true), got (%v, %v)", v, ok)
	}
}

func TestSimpleLRUConcurrent(t *testing.T) {
	// SimpleLRU 不是并发安全的，需要外部加锁
	lru := NewSimpleLRU[int, int](1000, nil)
	var mu sync.Mutex
	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 1000

	// 测试并发写入（使用外部锁保护）
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := base*numOps + j
				mu.Lock()
				lru.Set(key, key*10)
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	// 测试并发读取（使用外部锁保护）
	errCh := make(chan error, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := base*numOps + j
				mu.Lock()
				v, ok := lru.Get(key)
				mu.Unlock()
				if ok && v != key*10 {
					errCh <- fmt.Errorf("key %d: expected %d, got %d", key, key*10, v)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}
}

func TestSimpleLRUOnEvict(t *testing.T) {
	evictedKeys := make(chan string, 10)
	onEvict := func(k string, v int) {
		evictedKeys <- k
	}

	lru := NewSimpleLRU[string, int](2, onEvict)

	lru.Set("key1", 1)
	lru.Set("key2", 2)
	lru.Set("key3", 3) // 应淘汰 key1

	select {
	case key := <-evictedKeys:
		if key != "key1" {
			t.Errorf("Expected key1 to be evicted, got %s", key)
		}
	case <-time.After(1 * time.Second):
		t.Error("Eviction callback not called")
	}

	// 测试删除时的回调
	lru.Delete("key2")

	select {
	case key := <-evictedKeys:
		if key != "key2" {
			t.Errorf("Expected key2 to be evicted, got %s", key)
		}
	case <-time.After(1 * time.Second):
		t.Error("Eviction callback not called on delete")
	}
}

func TestSimpleLRUSlowOnEvict(t *testing.T) {
	evictedKeys := make(chan string, 100)
	operationsDone := atomic.Int32{}

	// 模拟耗时回调
	slowOnEvict := func(key string, value int) {
		evictedKeys <- key
		time.Sleep(10 * time.Millisecond)
	}

	lru := NewSimpleLRU[string, int](2, slowOnEvict)

	// 填满缓存
	lru.Set("key1", 1)
	lru.Set("key2", 2)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				key := fmt.Sprintf("worker%d-key%d", id, j)
				lru.Set(key, j)
				operationsDone.Add(1)
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	if operationsDone.Load() == 0 {
		t.Error("No operations completed")
	}

	// 确保至少有一些淘汰发生
	select {
	case <-evictedKeys:
		// OK
	case <-time.After(1 * time.Second):
		t.Error("No evictions occurred")
	}
}

func TestSimpleLRUComplex(t *testing.T) {
	lru := NewSimpleLRU[string, interface{}](10, nil)

	// 添加不同类型的值
	lru.Set("int", 42)
	lru.Set("string", "hello")
	lru.Set("slice", []int{1, 2, 3})
	lru.Set("map", map[string]int{"a": 1, "b": 2})

	// 验证复杂类型的存取
	if v, ok := lru.Get("slice"); !ok {
		t.Error("Get slice: not found")
	} else {
		slice := v.([]int)
		if len(slice) != 3 || slice[0] != 1 {
			t.Errorf("Get slice: wrong value %v", slice)
		}
	}

	// 随机读写压力测试
	for i := 0; i < 100; i++ {
		op := rand.IntN(3)
		key := fmt.Sprintf("k%d", rand.IntN(20))

		switch op {
		case 0:
			lru.Set(key, rand.IntN(1000))
		case 1:
			lru.Get(key)
		case 2:
			lru.Delete(key)
		}
	}

	// 验证容量控制
	if lru.Len() > lru.Capacity() {
		t.Errorf("Length %d exceeds capacity %d", lru.Len(), lru.Capacity())
	}
}

func BenchmarkSimpleLRU_Get(b *testing.B) {
	lru := NewSimpleLRU[int, int](10000, nil)
	for i := 0; i < 5000; i++ {
		lru.Set(i, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lru.Get(i % 5000)
	}
}

func BenchmarkSimpleLRU_Set(b *testing.B) {
	lru := NewSimpleLRU[int, int](10000, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lru.Set(i%5000, i)
	}
}

func BenchmarkSimpleLRU_Mixed(b *testing.B) {
	lru := NewSimpleLRU[int, int](100000, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := rand.IntN(1000000)
		switch rand.IntN(10) {
		case 0:
			lru.Set(key, key*10)
		case 1:
			lru.Delete(key)
		default:
			lru.Get(key)
		}
	}
}
