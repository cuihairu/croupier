package cache

import (
	"context"
	"testing"
	"time"
)

// TestNullCache_Get 测试 NullCache Get 方法
func TestNullCache_Get(t *testing.T) {
	cache := NewNullCache()
	ctx := context.Background()

	value, err := cache.Get(ctx, "test_key")
	if err != ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss, got %v", err)
	}

	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
	}
}

// TestNullCache_Set 测试 NullCache Set 方法
func TestNullCache_Set(t *testing.T) {
	cache := NewNullCache()
	ctx := context.Background()

	err := cache.Set(ctx, "test_key", []byte("test_value"), time.Hour)
	if err != nil {
		t.Errorf("Set should not return error, got %v", err)
	}

	// 验证设置后读取仍然是缓存未命中
	value, err := cache.Get(ctx, "test_key")
	if err != ErrCacheMiss {
		t.Errorf("After Set, Get should still return ErrCacheMiss, got %v", err)
	}

	if value != nil {
		t.Errorf("After Set, Get should return nil value, got %v", value)
	}
}

// TestNullCache_Delete 测试 NullCache Delete 方法
func TestNullCache_Delete(t *testing.T) {
	cache := NewNullCache()
	ctx := context.Background()

	err := cache.Delete(ctx, "test_key")
	if err != nil {
		t.Errorf("Delete should not return error, got %v", err)
	}
}

// TestNullCache_DeletePattern 测试 NullCache DeletePattern 方法
func TestNullCache_DeletePattern(t *testing.T) {
	cache := NewNullCache()
	ctx := context.Background()

	patterns := []string{
		"user:*",
		"session:*",
		"*:temp",
	}

	for _, pattern := range patterns {
		err := cache.DeletePattern(ctx, pattern)
		if err != nil {
			t.Errorf("DeletePattern(%q) should not return error, got %v", pattern, err)
		}
	}
}

// TestNullCache_Exists 测试 NullCache Exists 方法
func TestNullCache_Exists(t *testing.T) {
	cache := NewNullCache()
	ctx := context.Background()

	exists, err := cache.Exists(ctx, "test_key")
	if err != nil {
		t.Errorf("Exists should not return error, got %v", err)
	}

	if exists != false {
		t.Errorf("Expected exists to be false, got %v", exists)
	}

	// 先设置再检查
	_ = cache.Set(ctx, "test_key", []byte("value"), time.Hour)
	exists, err = cache.Exists(ctx, "test_key")
	if err != nil {
		t.Errorf("Exists after Set should not return error, got %v", err)
	}

	if exists != false {
		t.Errorf("Exists after Set should still return false, got %v", exists)
	}
}

// TestNullCache_Close 测试 NullCache Close 方法
func TestNullCache_Close(t *testing.T) {
	cache := NewNullCache()

	err := cache.Close()
	if err != nil {
		t.Errorf("Close should not return error, got %v", err)
	}

	// 多次关闭应该不报错
	err = cache.Close()
	if err != nil {
		t.Errorf("Close called twice should not return error, got %v", err)
	}
}

// TestNullCache_ConcurrentOperations 测试并发操作
func TestNullCache_ConcurrentOperations(t *testing.T) {
	cache := NewNullCache()
	ctx := context.Background()

	done := make(chan bool)

	// 并发读取
	for i := 0; i < 10; i++ {
		go func(idx int) {
			key := "key" + string(rune('0'+idx))
			_, _ = cache.Get(ctx, key)
			done <- true
		}(i)
	}

	// 并发写入
	for i := 0; i < 10; i++ {
		go func(idx int) {
			key := "key" + string(rune('0'+idx))
			_ = cache.Set(ctx, key, []byte("value"), time.Hour)
			done <- true
		}(i)
	}

	// 等待所有操作完成
	for i := 0; i < 20; i++ {
		<-done
	}
}

// TestNullCache_DifferentTTL 测试不同的 TTL 值
func TestNullCache_DifferentTTL(t *testing.T) {
	cache := NewNullCache()
	ctx := context.Background()

	ttls := []time.Duration{
		time.Second,
		time.Minute,
		time.Hour,
		24 * time.Hour,
		0,                // 无过期时间
		-1 * time.Second, // 负数
	}

	for _, ttl := range ttls {
		err := cache.Set(ctx, "key", []byte("value"), ttl)
		if err != nil {
			t.Errorf("Set with TTL %v should not return error, got %v", ttl, err)
		}
	}
}

// TestNullCache_EmptyKey 测试空键
func TestNullCache_EmptyKey(t *testing.T) {
	cache := NewNullCache()
	ctx := context.Background()

	// 空键应该被接受（虽然没有实际效果）
	err := cache.Set(ctx, "", []byte("value"), time.Hour)
	if err != nil {
		t.Errorf("Set with empty key should not return error, got %v", err)
	}

	value, err := cache.Get(ctx, "")
	if err != ErrCacheMiss {
		t.Errorf("Get with empty key should return ErrCacheMiss, got %v", err)
	}

	if value != nil {
		t.Errorf("Get with empty key should return nil, got %v", value)
	}
}

// TestNullCache_NilValue 测试空值
func TestNullCache_NilValue(t *testing.T) {
	cache := NewNullCache()
	ctx := context.Background()

	err := cache.Set(ctx, "key", nil, time.Hour)
	if err != nil {
		t.Errorf("Set with nil value should not return error, got %v", err)
	}
}

// TestNullCache_LargeValue 测试大值
func TestNullCache_LargeValue(t *testing.T) {
	cache := NewNullCache()
	ctx := context.Background()

	// 创建 1MB 的数据
	largeValue := make([]byte, 1024*1024)
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	err := cache.Set(ctx, "large_key", largeValue, time.Hour)
	if err != nil {
		t.Errorf("Set with large value should not return error, got %v", err)
	}
}

// TestCacheError_Error 测试 CacheError Error 方法
func TestCacheError_Error(t *testing.T) {
	err := &CacheError{Message: "test error"}
	if err.Error() != "test error" {
		t.Errorf("Error() returned wrong message: got %s, want 'test error'", err.Error())
	}

	// 空消息
	emptyErr := &CacheError{Message: ""}
	if emptyErr.Error() != "" {
		t.Errorf("Error() with empty message should return empty string, got %s", emptyErr.Error())
	}
}

// TestErrCacheMiss 测试预定义的缓存未命中错误
func TestErrCacheMiss(t *testing.T) {
	if ErrCacheMiss == nil {
		t.Fatal("ErrCacheMiss should not be nil")
	}

	if ErrCacheMiss.Message != "cache miss" {
		t.Errorf("ErrCacheMiss message wrong: got %s, want 'cache miss'", ErrCacheMiss.Message)
	}
}

// TestIsCacheMiss 测试 IsCacheMiss 函数
func TestIsCacheMiss(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrCacheMiss", ErrCacheMiss, true},
		{"CacheError pointer", &CacheError{Message: "cache miss"}, true}, // IsCacheMiss 检查类型，不检查消息
		{"nil error", nil, false},
		{"Other error", &TestError{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCacheMiss(tt.err)
			if result != tt.expected {
				t.Errorf("IsCacheMiss(%v) returned %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

// TestError 用于测试其他错误类型
type TestError struct{}

func (e *TestError) Error() string {
	return "test error"
}

// TestCacheStore_Interface 测试接口实现
func TestCacheStore_Interface(t *testing.T) {
	var cache CacheStore = NewNullCache()

	ctx := context.Background()

	// 验证所有接口方法都可以调用
	_, _ = cache.Get(ctx, "key")
	_ = cache.Set(ctx, "key", []byte("value"), time.Hour)
	_ = cache.Delete(ctx, "key")
	_ = cache.DeletePattern(ctx, "pattern*")
	_, _ = cache.Exists(ctx, "key")
	_ = cache.Close()

	// 验证 *NullCache 实现了接口
	var nullCache *NullCache = NewNullCache()
	_, _ = nullCache.Get(ctx, "key")
	_ = nullCache.Close()
}

// BenchmarkNullCache_Get Get 方法性能测试
func BenchmarkNullCache_Get(b *testing.B) {
	cache := NewNullCache()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(ctx, "test_key")
	}
}

// BenchmarkNullCache_Set Set 方法性能测试
func BenchmarkNullCache_Set(b *testing.B) {
	cache := NewNullCache()
	ctx := context.Background()
	value := []byte("test value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(ctx, "test_key", value, time.Hour)
	}
}

// BenchmarkNullCache_Exists Exists 方法性能测试
func BenchmarkNullCache_Exists(b *testing.B) {
	cache := NewNullCache()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Exists(ctx, "test_key")
	}
}

// BenchmarkNullCache_Concurrent 并发操作性能测试
func BenchmarkNullCache_Concurrent(b *testing.B) {
	cache := NewNullCache()
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cache.Get(ctx, "key")
		}
	})
}

// TestNullCache_ContextCancellation 测试上下文取消
func TestNullCache_ContextCancellation(t *testing.T) {
	cache := NewNullCache()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	// NullCache 不应该检查上下文，所以这些操作应该仍然成功
	err := cache.Set(ctx, "key", []byte("value"), time.Hour)
	if err != nil {
		t.Errorf("Set with cancelled context should not error in NullCache, got %v", err)
	}

	_, err = cache.Get(ctx, "key")
	if err != ErrCacheMiss {
		t.Errorf("Get with cancelled context should return ErrCacheMiss, got %v", err)
	}
}

// TestNullCache_ContextTimeout 测试上下文超时
func TestNullCache_ContextTimeout(t *testing.T) {
	cache := NewNullCache()

	ctx, cancel := context.WithTimeout(context.Background(), -time.Hour) // 已经过期
	defer cancel()

	// NullCache 不应该检查上下文超时
	err := cache.Set(ctx, "key", []byte("value"), time.Hour)
	if err != nil {
		t.Errorf("Set with timeout context should not error in NullCache, got %v", err)
	}
}

// TestNewNullCache 测试 NewNullCache 构造函数
func TestNewNullCache(t *testing.T) {
	cache := NewNullCache()
	if cache == nil {
		t.Fatal("NewNullCache returned nil")
	}

	// 验证返回的是 *NullCache 类型 - 由于 cache 已经是 *NullCache，直接比较即可
	if cache == nil {
		t.Error("NewNullCache should return non-nil *NullCache")
	}
}
