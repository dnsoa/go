package lru

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// 字符串大小的简单计算器
func stringSizer(v string) int {
	return len(v)
}

// 字节切片大小的计算器
func bytesSizer(v []byte) int {
	return len(v)
}

func TestByteLRUBasic(t *testing.T) {
	// 创建一个最大 100 字节的 LRU
	lru, err := NewByteLRU[string, string](
		WithMaxBytes[string, string](100),
		WithSizer[string, string](stringSizer),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}

	// 测试 Set 和 Get
	lru.Set("key1", "value1") // 6 bytes
	lru.Set("key2", "value2") // 6 bytes

	if v, ok := lru.Get("key1"); !ok || v != "value1" {
		t.Errorf("Get key1: expected (value1, true), got (%v, %v)", v, ok)
	}

	if v, ok := lru.Get("key2"); !ok || v != "value2" {
		t.Errorf("Get key2: expected (value2, true), got (%v, %v)", v, ok)
	}

	// 测试不存在的键
	if v, ok := lru.Get("nonexistent"); ok {
		t.Errorf("Get nonexistent: expected (, false), got (%v, %v)", v, ok)
	}

	// 测试当前字节数
	if lru.CurBytes() != 12 {
		t.Errorf("CurBytes: expected 12, got %d", lru.CurBytes())
	}

	// 测试 Len
	if lru.Len() != 2 {
		t.Errorf("Len: expected 2, got %d", lru.Len())
	}
}

func TestByteLRUEviction(t *testing.T) {
	evicted := []string{}
	onEvict := func(k string, v string) {
		evicted = append(evicted, k)
	}

	// 创建一个最大 15 字节的 LRU（只计算 value 大小）
	lru, err := NewByteLRU[string, string](
		WithMaxBytes[string, string](15),
		WithSizer[string, string](stringSizer),
		WithByteOnEvict[string, string](onEvict),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}

	lru.Set("key1", "value1") // 6 bytes (value1)
	lru.Set("key2", "value2") // 6 bytes (value2)
	lru.Set("key3", "value3") // 6 bytes, 需要淘汰 key1

	// key1 应该被淘汰
	if _, ok := lru.Get("key1"); ok {
		t.Error("key1 should have been evicted")
	}

	// 验证淘汰回调被调用
	if len(evicted) != 1 || evicted[0] != "key1" {
		t.Errorf("Expected key1 to be evicted, got %v", evicted)
	}

	// 验证当前字节数不超过限制
	if lru.CurBytes() > 15 {
		t.Errorf("CurBytes should not exceed 15, got %d", lru.CurBytes())
	}
}

func TestByteLRULargeValue(t *testing.T) {
	// 创建一个最大 10 字节的 LRU
	lru, err := NewByteLRU[string, string](
		WithMaxBytes[string, string](10),
		WithSizer[string, string](stringSizer),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}

	// 插入一个大于最大限制的值
	lru.Set("key", "this is a very long value")

	// 由于值太大，应该被立即淘汰
	if lru.Len() != 0 {
		t.Errorf("Len: expected 0 (value too large), got %d", lru.Len())
	}
}

func TestByteLRUNegativeSize(t *testing.T) {
	// 测试 sizer 返回负数的情况
	negativeSizer := func(v string) int { return -1 }
	lru, err := NewByteLRU[string, string](
		WithMaxBytes[string, string](100),
		WithSizer[string, string](negativeSizer),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}

	// 插入一个 sizer 返回负数的值，应该被忽略
	lru.Set("key", "value")

	if lru.Len() != 0 {
		t.Errorf("Len: expected 0 (negative size), got %d", lru.Len())
	}
}

func TestByteLRUUpdate(t *testing.T) {
	lru, err := NewByteLRU[string, string](
		WithMaxBytes[string, string](100),
		WithSizer[string, string](stringSizer),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}

	lru.Set("key1", "val1") // 4 bytes (value only)
	lru.Set("key2", "val2") // 4 bytes (value only)

	// 更新 key1 的值
	lru.Set("key1", "longervalue") // 11 bytes (value only)

	// 总大小应该是 4 + 11 = 15 bytes
	expectedBytes := 4 + 11
	if lru.CurBytes() != int64(expectedBytes) {
		t.Errorf("CurBytes: expected %d, got %d", expectedBytes, lru.CurBytes())
	}
}

func TestByteLRULRUOrder(t *testing.T) {
	// 创建一个最大 18 字节的 LRU
	lru, err := NewByteLRU[string, string](
		WithMaxBytes[string, string](18),
		WithSizer[string, string](stringSizer),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}

	lru.Set("key1", "value1") // 6 bytes
	lru.Set("key2", "value2") // 6 bytes
	lru.Set("key3", "value3") // 6 bytes

	// 访问 key1 使其成为最近使用的
	lru.Get("key1")

	// 添加新元素，应淘汰 key2（最久未使用）
	lru.Set("key4", "value4") // 6 bytes

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

func TestByteLRUDelete(t *testing.T) {
	evicted := []string{}
	onEvict := func(k string, v string) {
		evicted = append(evicted, k)
	}

	lru, err := NewByteLRU[string, string](
		WithMaxBytes[string, string](100),
		WithSizer[string, string](stringSizer),
		WithByteOnEvict[string, string](onEvict),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}

	lru.Set("key1", "value1")
	lru.Set("key2", "value2")

	// 删除 key1
	if !lru.Delete("key1") {
		t.Error("Delete key1 should return true")
	}

	if _, ok := lru.Get("key1"); ok {
		t.Error("key1 should be deleted")
	}

	// 验证删除回调被调用
	if len(evicted) != 1 || evicted[0] != "key1" {
		t.Errorf("Expected key1 to be evicted on delete, got %v", evicted)
	}

	// 删除不存在的键
	if lru.Delete("nonexistent") {
		t.Error("Delete nonexistent should return false")
	}
}

func TestByteLRUContains(t *testing.T) {
	lru, err := NewByteLRU[string, string](
		WithMaxBytes[string, string](100),
		WithSizer[string, string](stringSizer),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}

	lru.Set("key1", "value1")

	if !lru.Contains("key1") {
		t.Error("Contains key1: expected true, got false")
	}

	if lru.Contains("nonexistent") {
		t.Error("Contains nonexistent: expected false, got true")
	}
}

func TestByteLRUClear(t *testing.T) {
	evicted := []string{}
	onEvict := func(k string, v string) {
		evicted = append(evicted, k)
	}

	lru, err := NewByteLRU[string, string](
		WithMaxBytes[string, string](100),
		WithSizer[string, string](stringSizer),
		WithByteOnEvict[string, string](onEvict),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}

	lru.Set("key1", "value1")
	lru.Set("key2", "value2")
	lru.Set("key3", "value3")

	lru.Clear()

	if lru.Len() != 0 {
		t.Errorf("Len after clear: expected 0, got %d", lru.Len())
	}

	if lru.CurBytes() != 0 {
		t.Errorf("CurBytes after clear: expected 0, got %d", lru.CurBytes())
	}

	// 验证所有条目的回调都被调用
	if len(evicted) != 3 {
		t.Errorf("Expected 3 evictions on clear, got %d", len(evicted))
	}
}

func TestByteLRUStats(t *testing.T) {
	lru, err := NewByteLRU[string, string](
		WithMaxBytes[string, string](100),
		WithSizer[string, string](stringSizer),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}

	lru.Set("key1", "value1")
	lru.Set("key2", "value2")

	count, curBytes, maxBytes := lru.Stats()

	if count != 2 {
		t.Errorf("Stats count: expected 2, got %d", count)
	}

	if curBytes != 12 {
		t.Errorf("Stats curBytes: expected 12, got %d", curBytes)
	}

	if maxBytes != 100 {
		t.Errorf("Stats maxBytes: expected 100, got %d", maxBytes)
	}
}

func TestByteLRUConcurrent(t *testing.T) {
	lru, err := NewByteLRU[int, string](
		WithMaxBytes[int, string](10000),
		WithSizer[int, string](func(v string) int { return len(v) }),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}
	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 100

	// 并发写入
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := base*numOps + j
				lru.Set(key, fmt.Sprintf("value%d", key))
			}
		}(i)
	}
	wg.Wait()

	// 并发读取
	errCh := make(chan error, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := base*numOps + j
				v, ok := lru.Get(key)
				if ok && v != fmt.Sprintf("value%d", key) {
					errCh <- fmt.Errorf("key %d: expected %s, got %s", key, fmt.Sprintf("value%d", key), v)
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

func TestByteLRUWithBytes(t *testing.T) {
	// 测试使用字节切片作为值
	lru, err := NewByteLRU[string, []byte](
		WithMaxBytes[string, []byte](100),
		WithSizer[string, []byte](bytesSizer),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}

	lru.Set("key1", []byte{1, 2, 3, 4, 5}) // 5 bytes
	lru.Set("key2", make([]byte, 10))      // 10 bytes

	if lru.CurBytes() != 15 {
		t.Errorf("CurBytes: expected 15, got %d", lru.CurBytes())
	}

	if v, ok := lru.Get("key1"); !ok || len(v) != 5 {
		t.Errorf("Get key1: expected 5 bytes, got %d", len(v))
	}
}

func TestByteLRUMultipleEvictions(t *testing.T) {
	evicted := []string{}
	onEvict := func(k string, v string) {
		evicted = append(evicted, k)
	}

	// 创建一个最大 9 字节的 LRU
	lru, err := NewByteLRU[string, string](
		WithMaxBytes[string, string](9),
		WithSizer[string, string](stringSizer),
		WithByteOnEvict[string, string](onEvict),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}

	// 添加条目，每个 value 2 bytes
	lru.Set("a", "v1") // 2 bytes
	lru.Set("b", "v2") // 2 bytes
	lru.Set("c", "v3") // 2 bytes
	lru.Set("d", "v4") // 2 bytes
	lru.Set("e", "v5") // 2 bytes, 需要淘汰多个

	// 验证淘汰顺序（从最老的开始）
	if len(evicted) < 1 {
		t.Error("Expected at least one eviction")
	}

	// 验证当前字节数不超过限制
	if lru.CurBytes() > 9 {
		t.Errorf("CurBytes should not exceed 9, got %d", lru.CurBytes())
	}
}

func TestByteLRUOnEvict(t *testing.T) {
	evictedKeys := make(chan string, 10)
	onEvict := func(k string, v string) {
		evictedKeys <- k
	}

	lru, err := NewByteLRU[string, string](
		WithMaxBytes[string, string](10),
		WithSizer[string, string](stringSizer),
		WithByteOnEvict[string, string](onEvict),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}

	lru.Set("key1", "value1") // 6 bytes
	lru.Set("key2", "value2") // 6 bytes, 需要淘汰 key1

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

func TestByteLRUSetOnEvict(t *testing.T) {
	lru, err := NewByteLRU[string, string](
		WithMaxBytes[string, string](5),
		WithSizer[string, string](stringSizer),
	)
	if err != nil {
		t.Fatalf("NewByteLRU failed: %v", err)
	}

	evicted := []string{}
	lru.SetOnEvict(func(k string, v string) {
		evicted = append(evicted, k)
	})

	lru.Set("key1", "v1")    // 2 bytes
	lru.Set("key2", "value") // 5 bytes, 总共 7 > 5, 淘汰 key1

	if len(evicted) != 1 || evicted[0] != "key1" {
		t.Errorf("Expected key1 to be evicted with new callback, got %v", evicted)
	}
}

func TestByteLRUOptionsErrors(t *testing.T) {
	// 测试缺少 maxBytes
	_, err := NewByteLRU[string, string](
		WithSizer[string, string](stringSizer),
	)
	if err == nil {
		t.Error("Expected error for missing maxBytes")
	}

	// 测试无效的 maxBytes
	_, err = NewByteLRU[string, string](
		WithMaxBytes[string, string](0),
		WithSizer[string, string](stringSizer),
	)
	if err == nil {
		t.Error("Expected error for zero maxBytes")
	}

	// 测试无效的 maxBytes (负数)
	_, err = NewByteLRU[string, string](
		WithMaxBytes[string, string](-1),
		WithSizer[string, string](stringSizer),
	)
	if err == nil {
		t.Error("Expected error for negative maxBytes")
	}

	// 测试 nil sizer
	_, err = NewByteLRU[string, string](
		WithMaxBytes[string, string](100),
		WithSizer[string, string](nil),
	)
	if err == nil {
		t.Error("Expected error for nil sizer")
	}

	// 测试非 string/[]byte 类型且没有设置 sizer
	type CustomType struct{ data string }
	_, err = NewByteLRU[string, CustomType](
		WithMaxBytes[string, CustomType](100),
	)
	if err == nil {
		t.Error("Expected error for missing sizer with non-default type")
	}
}

func TestByteLRUDefaultSizer(t *testing.T) {
	// 测试 string 类型使用默认 sizer
	t.Run("string default sizer", func(t *testing.T) {
		lru, err := NewByteLRU[string, string](
			WithMaxBytes[string, string](100),
		)
		if err != nil {
			t.Fatalf("NewByteLRU failed: %v", err)
		}

		lru.Set("key1", "value1") // 6 bytes
		lru.Set("key2", "value2") // 6 bytes

		if lru.CurBytes() != 12 {
			t.Errorf("CurBytes: expected 12, got %d", lru.CurBytes())
		}
	})

	// 测试 []byte 类型使用默认 sizer
	t.Run("[]byte default sizer", func(t *testing.T) {
		lru, err := NewByteLRU[string, []byte](
			WithMaxBytes[string, []byte](100),
		)
		if err != nil {
			t.Fatalf("NewByteLRU failed: %v", err)
		}

		lru.Set("key1", []byte{1, 2, 3, 4, 5}) // 5 bytes
		lru.Set("key2", make([]byte, 10))      // 10 bytes

		if lru.CurBytes() != 15 {
			t.Errorf("CurBytes: expected 15, got %d", lru.CurBytes())
		}
	})

	// 测试 string 类型使用默认 sizer 并触发淘汰
	t.Run("string default sizer with eviction", func(t *testing.T) {
		evicted := []string{}
		lru, err := NewByteLRU[string, string](
			WithMaxBytes[string, string](10),
			WithByteOnEvict[string, string](func(k, v string) {
				evicted = append(evicted, k)
			}),
		)
		if err != nil {
			t.Fatalf("NewByteLRU failed: %v", err)
		}

		lru.Set("key1", "value1") // 6 bytes
		lru.Set("key2", "value2") // 6 bytes, 淘汰 key1

		if len(evicted) != 1 || evicted[0] != "key1" {
			t.Errorf("Expected key1 to be evicted, got %v", evicted)
		}
	})
}

func BenchmarkByteLRU_Get(b *testing.B) {
	lru, _ := NewByteLRU[int, string](
		WithMaxBytes[int, string](10000),
		WithSizer[int, string](func(v string) int { return len(v) }),
	)
	for i := 0; i < 5000; i++ {
		lru.Set(i, fmt.Sprintf("value%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lru.Get(i % 5000)
	}
}

func BenchmarkByteLRU_Set(b *testing.B) {
	lru, _ := NewByteLRU[int, string](
		WithMaxBytes[int, string](10000),
		WithSizer[int, string](func(v string) int { return len(v) }),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lru.Set(i%5000, fmt.Sprintf("value%d", i))
	}
}

func BenchmarkByteLRU_WithEviction(b *testing.B) {
	lru, _ := NewByteLRU[int, string](
		WithMaxBytes[int, string](1000),
		WithSizer[int, string](func(v string) int { return len(v) }),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lru.Set(i, fmt.Sprintf("value%d", i))
	}
}
