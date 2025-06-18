package lru

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{6, 8},
		{7, 8},
		{8, 8},
		{4096, 4096},
		{4097, 8192},
		{9999, 16384},
		{10000, 16384},
	}

	for _, test := range tests {
		result := nextPowerOfTwo(test.input)
		if result != test.expected {
			t.Errorf("nextPowerOfTwo(%d) = %d; expected %d", test.input, result, test.expected)
		}
	}
}

func TestLRUShardMapBasic(t *testing.T) {
	// 创建一个小容量的缓存，便于测试LRU淘汰
	lru := NewLRUShardMap(
		WithLRUShardCount[string, int](4),
		WithLRUCapacity[string, int](8),
	)

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
	if lru.Len() != 1 {
		t.Errorf("Len: expected 1, got %d", lru.Len())
	}

	// 测试 Clear
	lru.Clear()
	if lru.Len() != 0 {
		t.Errorf("Len after clear: expected 0, got %d", lru.Len())
	}
}

func TestLRUShardMapEviction(t *testing.T) {
	// 创建一个4个分片，每个分片容量为2的缓存（总容量为8）
	lru := NewLRUShardMap(
		WithLRUShardCount[int, int](4),
		WithLRUCapacity[int, int](8),
	)

	// 添加16个元素应触发淘汰
	for i := 0; i < 16; i++ {
		lru.Set(i, i*10)
	}

	// 验证一些最旧的元素已被淘汰
	var evicted int
	for i := 0; i < 16; i++ {
		if _, ok := lru.Get(i); !ok {
			evicted++
		}
	}

	// 应有一些元素被淘汰，但不应全部淘汰
	if evicted == 0 || evicted == 16 {
		t.Errorf("Expected some elements to be evicted, got %d evicted out of 16", evicted)
	}

	// 确保容量控制在指定范围内
	if lru.Len() > 8 {
		t.Errorf("Expected length <= 8, got %d", lru.Len())
	}
}

func TestLRUShardMapLRUOrder(t *testing.T) {
	lru := NewLRUShardMap(
		WithLRUShardCount[string, int](0),
		WithLRUCapacity[string, int](3),
	)

	lru.Set("key0", 0)
	lru.Set("key1", 1)
	lru.Set("key2", 2)
	lru.Set("key3", 3)

	// 访问key1使其成为最近使用的
	lru.Get("key1")

	// 添加一个新元素，触发淘汰
	lru.Set("key4", 4)

	if _, ok := lru.Get("key0"); ok {
		t.Error("key2 should have been evicted")
	}

	if _, ok := lru.Get("key1"); !ok {
		t.Error("key1 should exist")
	}

	if _, ok := lru.Get("key3"); !ok {
		t.Error("key3 should exist")
	}

	if _, ok := lru.Get("key4"); !ok {
		t.Error("key4 should exist")
	}
}

func TestLRUShardMapEdgeCases(t *testing.T) {
	// 测试创建时的边缘情况
	lru1 := NewLRUShardMap[string, int]()
	// 应该使用默认值创建
	if len(lru1.shards) != defaultLRUShardNUM || lru1.shards[0].capacity != defaultLRUCapacity/defaultLRUShardNUM {
		t.Error("Failed to use default values for invalid parameters")
	}

	// 测试零值和空值
	lru := NewLRUShardMap(
		WithLRUShardCount[string, *int](4),
		WithLRUCapacity[string, *int](8),
	)
	var nilPtr *int

	// 设置nil值
	lru.Set("nilValue", nilPtr)
	if v, ok := lru.Get("nilValue"); !ok || v != nilPtr {
		t.Errorf("Get nil value: expected (%v, true), got (%v, %v)", nilPtr, v, ok)
	}

	var intPtr = func(a int) *int {
		return &a
	}
	// 设置空字符串键
	lru.Set("", intPtr(42))
	if v, ok := lru.Get(""); !ok || *v != 42 {
		t.Errorf("Get empty key: expected (42, true), got (%v, %v)", v, ok)
	}

	// 使用不同类型测试
	lruStr := NewLRUShardMap(
		WithLRUShardCount[int, string](4),
		WithLRUCapacity[int, string](8),
	)
	lruStr.Set(0, "zero")
	if v, ok := lruStr.Get(0); !ok || v != "zero" {
		t.Errorf("Get with int key: expected (zero, true), got (%v, %v)", v, ok)
	}
}

func TestLRUShardMapConcurrent(t *testing.T) {
	lru := NewLRUShardMap(
		WithLRUShardCount[int, int](16),
		WithLRUCapacity[int, int](1000),
	)
	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 1000

	// 测试并发写入
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := base*numOps + j
				lru.Set(key, key*10)
			}
		}(i)
	}
	wg.Wait()

	// 测试并发读取
	errCh := make(chan error, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := base*numOps + j
				v, ok := lru.Get(key)
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

	// 测试并发混合操作（读、写、删除）
	wg = sync.WaitGroup{}
	stop := make(chan struct{})

	// 写入协程
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				key := i % 1000
				lru.Set(key, i)
				i++
				time.Sleep(time.Microsecond)
			}
		}
	}()

	// 读取协程
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				key := i % 1000
				lru.Get(key)
				i++
				time.Sleep(time.Microsecond)
			}
		}
	}()

	// 删除协程
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				key := i % 1000
				lru.Delete(key)
				i++
				time.Sleep(time.Microsecond * 10)
			}
		}
	}()

	// 允许并发操作一段时间
	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
	lru.Stats()

}

func TestLRUShardMapComplex(t *testing.T) {
	// 创建一个大容量的缓存用于复杂场景测试
	lru := NewLRUShardMap(
		WithLRUShardCount[string, interface{}](8),
		WithLRUCapacity[string, interface{}](100),
	)

	// 添加不同类型的值
	lru.Set("int", 42)
	lru.Set("string", "hello")
	lru.Set("slice", []int{1, 2, 3})
	lru.Set("map", map[string]int{"a": 1, "b": 2})
	lru.Set("struct", struct{ Name string }{"test"})

	// 验证复杂类型的存取
	if v, ok := lru.Get("slice"); !ok {
		t.Error("Get slice: not found")
	} else {
		slice := v.([]int)
		if len(slice) != 3 || slice[0] != 1 {
			t.Errorf("Get slice: wrong value %v", slice)
		}
	}

	// 模拟缓存热点访问模式
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("temp%d", i)
		lru.Set(key, i)

		// 频繁访问某些键，确保它们不被淘汰
		if i%3 == 0 {
			lru.Get("int")
		}
		if i%4 == 0 {
			lru.Get("string")
		}
	}

	// 验证频繁访问的热点键没有被淘汰
	if _, ok := lru.Get("int"); !ok {
		t.Error("热点键 'int' 被错误淘汰")
	}
	if _, ok := lru.Get("string"); !ok {
		t.Error("热点键 'string' 被错误淘汰")
	}

	// 随机读写压力测试
	for i := 0; i < 1000; i++ {
		op := rand.IntN(3) // 0: Set, 1: Get, 2: Del
		key := fmt.Sprintf("k%d", rand.IntN(50))

		switch op {
		case 0: // 设置
			lru.Set(key, rand.IntN(10000))
		case 1: // 获取
			lru.Get(key)
		case 2: // 删除
			lru.Delete(key)
		}
	}
	hitRate, shardLoad := lru.Stats()
	_, _ = hitRate, shardLoad
	// t.Logf("Hit Rate: %.2f%%, Shard Load: %v", hitRate*100, shardLoad)
}

func TestLRUShardMapSlowOnEvict(t *testing.T) {
	evictedKeys := make(chan string, 100)

	// 模拟耗时回调 (500ms)
	slowOnEvict := func(key string, value int) {
		evictedKeys <- key
	}

	lru := NewLRUShardMap(
		WithLRUShardCount[string, int](2),
		WithLRUCapacity[string, int](4),
		WithLRUOnEvict(slowOnEvict))

	// 步骤1: 填满缓存
	for i := 0; i < 4; i++ {
		key := fmt.Sprintf("key%d", i)
		lru.Set(key, i)
	}

	lru.Set("extra", 999)

	operationsDone := atomic.Int32{}

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("worker%d-key%d", id, j)

				switch j % 3 {
				case 0:
					lru.Set(key, j)
				case 1:
					lru.Get("extra")
				case 2:
					lru.Delete(fmt.Sprintf("key%d", j%4))
				}

				operationsDone.Add(1)
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	evictions := 0
	select {
	case <-evictedKeys:
		evictions++
	case <-time.After(1 * time.Second):
		t.Error("Eviction callback did not complete")
	}

	if evictions == 0 {
		t.Error("Expected at least one eviction callback")
	}
	lru.Set("final-test", 123)
	if val, ok := lru.Get("final-test"); !ok || val != 123 {
		t.Error("Cache operations failed after eviction")
	}
}

func BenchmarkLRUShardMap_Get(b *testing.B) {
	lru := NewLRUShardMap(
		WithLRUCapacity[int, int](10000),
	)
	for i := 0; i < 5000; i++ {
		lru.Set(i, i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			lru.Get(i % 5000)
			i++
		}
	})
}

func BenchmarkLRUShardMap_Set(b *testing.B) {
	lru := NewLRUShardMap(
		WithLRUShardCount[int, int](16),
		WithLRUCapacity[int, int](10000),
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			lru.Set(i%5000, i)
			i++
		}
	})
}

func BenchmarkLRUShardMap_Mixed(b *testing.B) {
	lru := NewLRUShardMap(
		WithLRUShardCount[int, int](32),
		WithLRUCapacity[int, int](100000),
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {

		for pb.Next() {
			key := rand.IntN(1000000)
			switch rand.IntN(10) {
			case 0: // 10% 写入
				lru.Set(key, key*10)
			case 1: // 10% 删除
				lru.Delete(key)
			default: // 80% 读取
				lru.Get(key)
			}
		}
	})
}
