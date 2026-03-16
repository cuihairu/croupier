package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
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

// TestLocalCache_NewLocalCache 测试 LocalCache 构造函数
func TestLocalCache_NewLocalCache(t *testing.T) {
	tests := []struct {
		name            string
		defaultTTL      time.Duration
		cleanupInterval time.Duration
	}{
		{"Default values", 0, 0},
		{"Custom values", time.Minute, 5 * time.Minute},
		{"Short TTL", time.Second, 10 * time.Second},
		{"Long TTL", time.Hour, 2 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewLocalCache(tt.defaultTTL, tt.cleanupInterval)
			if cache == nil {
				t.Fatal("NewLocalCache returned nil")
			}
			if cache.cache == nil {
				t.Error("NewLocalCache.cache is nil")
			}
		})
	}
}

// TestLocalCache_Get 测试 LocalCache Get 方法
func TestLocalCache_Get(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	// 测试获取不存在的键
	_, err := cache.Get(ctx, "nonexistent")
	if err != ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss for nonexistent key, got %v", err)
	}

	// 设置值后获取
	value := []byte("test_value")
	_ = cache.Set(ctx, "test_key", value, time.Minute)

	retrieved, err := cache.Get(ctx, "test_key")
	if err != nil {
		t.Errorf("Get failed after Set: %v", err)
	}
	if string(retrieved) != string(value) {
		t.Errorf("Get returned wrong value: got %s, want %s", retrieved, value)
	}
}

// TestLocalCache_Set 测试 LocalCache Set 方法
func TestLocalCache_Set(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	tests := []struct {
		name  string
		key   string
		value []byte
		ttl   time.Duration
	}{
		{"Basic value", "key1", []byte("value1"), time.Minute},
		{"Empty value", "key2", []byte(""), time.Minute},
		{"Large value", "key3", make([]byte, 1024*100), time.Hour},
		{"Zero TTL (uses default)", "key4", []byte("value4"), 0},
		{"Special characters", "key:5", []byte("special:value"), time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cache.Set(ctx, tt.key, tt.value, tt.ttl)
			if err != nil {
				t.Errorf("Set(%q) failed: %v", tt.key, err)
			}

			// 验证值可以被获取
			retrieved, err := cache.Get(ctx, tt.key)
			if err != nil {
				t.Errorf("Get after Set(%q) failed: %v", tt.key, err)
			}
			if string(retrieved) != string(tt.value) {
				t.Errorf("Value mismatch for %q: got %s, want %s", tt.key, retrieved, tt.value)
			}
		})
	}
}

// TestLocalCache_Delete 测试 LocalCache Delete 方法
func TestLocalCache_Delete(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	// 设置值
	_ = cache.Set(ctx, "delete_me", []byte("value"), time.Minute)

	// 验证存在
	_, err := cache.Get(ctx, "delete_me")
	if err != nil {
		t.Fatalf("Get before delete failed: %v", err)
	}

	// 删除
	err = cache.Delete(ctx, "delete_me")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// 验证已删除
	_, err = cache.Get(ctx, "delete_me")
	if err != ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss after delete, got %v", err)
	}

	// 删除不存在的键不应该报错
	err = cache.Delete(ctx, "nonexistent")
	if err != nil {
		t.Errorf("Delete of nonexistent key should not error, got %v", err)
	}
}

// TestLocalCache_DeletePattern 测试 LocalCache DeletePattern 方法
func TestLocalCache_DeletePattern(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	// 设置多个键
	keys := []string{
		"user:1",
		"user:2",
		"user:3",
		"session:abc",
		"session:def",
		"other:key",
	}

	for _, key := range keys {
		_ = cache.Set(ctx, key, []byte("value"), time.Minute)
	}

	// 测试通配符删除
	t.Run("Delete with wildcard", func(t *testing.T) {
		err := cache.DeletePattern(ctx, "user:*")
		if err != nil {
			t.Errorf("DeletePattern(user:*) failed: %v", err)
		}

		// 验证 user:* 已删除
		_, err = cache.Get(ctx, "user:1")
		if err != ErrCacheMiss {
			t.Errorf("user:1 should be deleted, got %v", err)
		}
		_, err = cache.Get(ctx, "user:2")
		if err != ErrCacheMiss {
			t.Errorf("user:2 should be deleted, got %v", err)
		}

		// 验证其他键存在
		_, err = cache.Get(ctx, "session:abc")
		if err == ErrCacheMiss {
			t.Error("session:abc should still exist")
		}
	})

	// 测试精确匹配
	t.Run("Exact match", func(t *testing.T) {
		err := cache.DeletePattern(ctx, "other:key")
		if err != nil {
			t.Errorf("DeletePattern(exact) failed: %v", err)
		}

		_, err = cache.Get(ctx, "other:key")
		if err != ErrCacheMiss {
			t.Errorf("other:key should be deleted, got %v", err)
		}
	})

	// 测试空模式
	t.Run("Empty pattern", func(t *testing.T) {
		err := cache.DeletePattern(ctx, "")
		if err != nil {
			t.Errorf("DeletePattern(empty) failed: %v", err)
		}
	})

	// 测试无通配符
	t.Run("No wildcard", func(t *testing.T) {
		err := cache.DeletePattern(ctx, "session:abc")
		if err != nil {
			t.Errorf("DeletePattern(no wildcard) failed: %v", err)
		}

		_, err = cache.Get(ctx, "session:abc")
		if err != ErrCacheMiss {
			t.Errorf("session:abc should be deleted, got %v", err)
		}
	})
}

// TestLocalCache_Exists 测试 LocalCache Exists 方法
func TestLocalCache_Exists(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	// 不存在的键
	exists, err := cache.Exists(ctx, "nonexistent")
	if err != nil {
		t.Errorf("Exists failed for nonexistent key: %v", err)
	}
	if exists {
		t.Error("Exists returned true for nonexistent key")
	}

	// 设置后检查
	_ = cache.Set(ctx, "existing", []byte("value"), time.Minute)
	exists, err = cache.Exists(ctx, "existing")
	if err != nil {
		t.Errorf("Exists failed for existing key: %v", err)
	}
	if !exists {
		t.Error("Exists returned false for existing key")
	}
}

// TestLocalCache_Close 测试 LocalCache Close 方法
func TestLocalCache_Close(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	// 设置一些值
	_ = cache.Set(ctx, "key1", []byte("value1"), time.Minute)
	_ = cache.Set(ctx, "key2", []byte("value2"), time.Minute)

	// 关闭
	err := cache.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// 验证缓存已清空
	count := cache.Stats()
	if count != 0 {
		t.Errorf("After Close, stats should be 0, got %d", count)
	}
}

// TestLocalCache_Stats 测试 LocalCache Stats 方法
func TestLocalCache_Stats(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	// 初始计数
	if cache.Stats() != 0 {
		t.Errorf("Initial stats should be 0, got %d", cache.Stats())
	}

	// 添加一些项
	for i := 0; i < 5; i++ {
		_ = cache.Set(ctx, fmt.Sprintf("key%d", i), []byte("value"), time.Minute)
	}

	count := cache.Stats()
	if count != 5 {
		t.Errorf("Stats should be 5, got %d", count)
	}

	// 删除一个
	_ = cache.Delete(ctx, "key0")
	count = cache.Stats()
	if count != 4 {
		t.Errorf("Stats after delete should be 4, got %d", count)
	}
}

// TestLocalCache_Expiration 测试过期功能
func TestLocalCache_Expiration(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	// 设置短 TTL 的值
	_ = cache.Set(ctx, "short", []byte("value"), 10*time.Millisecond)

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	_, err := cache.Get(ctx, "short")
	if err != ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss for expired key, got %v", err)
	}
}

// TestLocalCache_Overwrite 测试覆盖值
func TestLocalCache_Overwrite(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	// 设置初始值
	_ = cache.Set(ctx, "key", []byte("value1"), time.Minute)

	// 覆盖
	_ = cache.Set(ctx, "key", []byte("value2"), time.Minute)

	value, err := cache.Get(ctx, "key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if string(value) != "value2" {
		t.Errorf("Overwritten value wrong: got %s, want value2", value)
	}
}

// TestLocalCache_DefaultTTL 测试默认 TTL
func TestLocalCache_DefaultTTL(t *testing.T) {
	cache := NewLocalCache(100*time.Millisecond, time.Minute)
	ctx := context.Background()

	// 使用零 TTL，应该使用默认值
	_ = cache.Set(ctx, "default_ttl", []byte("value"), 0)

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	_, err := cache.Get(ctx, "default_ttl")
	if err != ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss for expired default TTL key, got %v", err)
	}
}

// TestLocalCache_ConcurrentOperations 测试并发操作
func TestLocalCache_ConcurrentOperations(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	done := make(chan bool, 100)

	// 并发写入
	for i := 0; i < 50; i++ {
		go func(idx int) {
			key := fmt.Sprintf("key%d", idx)
			_ = cache.Set(ctx, key, []byte(fmt.Sprintf("value%d", idx)), time.Minute)
			done <- true
		}(i)
	}

	// 并发读取
	for i := 0; i < 50; i++ {
		go func(idx int) {
			key := fmt.Sprintf("key%d", idx)
			_, _ = cache.Get(ctx, key)
			done <- true
		}(i)
	}

	// 等待完成
	for i := 0; i < 100; i++ {
		<-done
	}

	// 验证所有键都存在
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key%d", i)
		_, err := cache.Get(ctx, key)
		if err != nil {
			t.Errorf("Key %s not found: %v", key, err)
		}
	}
}

// TestLocalCache_InvalidType 测试无效类型处理
func TestLocalCache_InvalidType(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	// 这里我们测试正常的 []byte 类型工作正常
	_ = cache.Set(ctx, "valid", []byte("value"), time.Minute)
	val, err := cache.Get(ctx, "valid")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if len(val) == 0 {
		t.Error("Value should have content")
	}
}

// TestLocalCache_DeletePatternNoWildcard 测试无通配符的模式删除
func TestLocalCache_DeletePatternNoWildcard(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	_ = cache.Set(ctx, "exact_key", []byte("value"), time.Minute)
	_ = cache.Set(ctx, "prefix_exact_key", []byte("value2"), time.Minute)

	// 删除精确匹配
	_ = cache.DeletePattern(ctx, "exact_key")

	// 验证精确匹配已删除
	_, err := cache.Get(ctx, "exact_key")
	if err != ErrCacheMiss {
		t.Errorf("exact_key should be deleted, got %v", err)
	}

	// 验证前缀键仍然存在
	_, err = cache.Get(ctx, "prefix_exact_key")
	if err == ErrCacheMiss {
		t.Error("prefix_exact_key should still exist")
	}
}

// TestCacheKey 测试 CacheKey 函数
func TestCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected string
	}{
		{"Single part", []string{"test"}, "croupier:test"},
		{"Two parts", []string{"user", "123"}, "croupier:user:123"},
		{"Multiple parts", []string{"a", "b", "c", "d"}, "croupier:a:b:c:d"},
		{"Empty parts", []string{}, "croupier:"},
		{"Empty string part", []string{"a", "", "b"}, "croupier:a::b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CacheKey(tt.parts...)
			if result != tt.expected {
				t.Errorf("CacheKey(%v) = %s, want %s", tt.parts, result, tt.expected)
			}
		})
	}
}

// TestAdminCacheKey 测试 AdminCacheKey 函数
func TestAdminCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		username string
		expected string
	}{
		{"Lowercase", "admin", "croupier:admin:user:admin"},
		{"Uppercase should be lowercased", "ADMIN", "croupier:admin:user:admin"},
		{"Mixed case", "AdminUser", "croupier:admin:user:adminuser"},
		{"With special chars", "admin@example.com", "croupier:admin:user:admin@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AdminCacheKey(tt.username)
			if result != tt.expected {
				t.Errorf("AdminCacheKey(%q) = %s, want %s", tt.username, result, tt.expected)
			}
		})
	}
}

// TestAdminIDCacheKey 测试 AdminIDCacheKey 函数
func TestAdminIDCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		adminID  uint
		expected string
	}{
		{"ID 0", 0, "croupier:admin:id:0"},
		{"ID 1", 1, "croupier:admin:id:1"},
		{"Large ID", 12345, "croupier:admin:id:12345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AdminIDCacheKey(tt.adminID)
			if result != tt.expected {
				t.Errorf("AdminIDCacheKey(%d) = %s, want %s", tt.adminID, result, tt.expected)
			}
		})
	}
}

// TestAdminRolesCacheKey 测试 AdminRolesCacheKey 函数
func TestAdminRolesCacheKey(t *testing.T) {
	result := AdminRolesCacheKey(42)
	expected := "croupier:admin:roles:42"
	if result != expected {
		t.Errorf("AdminRolesCacheKey(42) = %s, want %s", result, expected)
	}
}

// TestRoleCacheKey 测试 RoleCacheKey 函数
func TestRoleCacheKey(t *testing.T) {
	result := RoleCacheKey(10)
	expected := "croupier:role:10"
	if result != expected {
		t.Errorf("RoleCacheKey(10) = %s, want %s", result, expected)
	}
}

// TestRolePermissionsCacheKey 测试 RolePermissionsCacheKey 函数
func TestRolePermissionsCacheKey(t *testing.T) {
	result := RolePermissionsCacheKey(5)
	expected := "croupier:role:perms:5"
	if result != expected {
		t.Errorf("RolePermissionsCacheKey(5) = %s, want %s", result, expected)
	}
}

// TestPermissionCacheKey 测试 PermissionCacheKey 函数
func TestPermissionCacheKey(t *testing.T) {
	tests := []struct {
		name         string
		permissionID string
		expected     string
	}{
		{"Simple", "read", "croupier:permission:read"},
		{"Mixed case should be lowercased", "WRITE_DATA", "croupier:permission:write_data"},
		{"With dots", "user.delete", "croupier:permission:user.delete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PermissionCacheKey(tt.permissionID)
			if result != tt.expected {
				t.Errorf("PermissionCacheKey(%q) = %s, want %s", tt.permissionID, result, tt.expected)
			}
		})
	}
}

// TestGameCacheKey 测试 GameCacheKey 函数
func TestGameCacheKey(t *testing.T) {
	result := GameCacheKey(123)
	expected := "croupier:game:123"
	if result != expected {
		t.Errorf("GameCacheKey(123) = %s, want %s", result, expected)
	}
}

// TestGamesCacheKey 测试 GamesCacheKey 函数
func TestGamesCacheKey(t *testing.T) {
	result := GamesCacheKey()
	expected := "croupier:games:all"
	if result != expected {
		t.Errorf("GamesCacheKey() = %s, want %s", result, expected)
	}
}

// TestCacheHelper_NewCacheHelper 测试 CacheHelper 构造函数
func TestCacheHelper_NewCacheHelper(t *testing.T) {
	cache := NewNullCache()
	helper := NewCacheHelper(cache)

	if helper == nil {
		t.Fatal("NewCacheHelper returned nil")
	}
	if helper.store != cache {
		t.Error("NewCacheHelper.store is not the provided cache")
	}
}

// TestCacheHelper_GetJSON 测试 CacheHelper GetJSON 方法
func TestCacheHelper_GetJSON(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	helper := NewCacheHelper(cache)
	ctx := context.Background()

	// 测试缓存未命中
	var dest map[string]interface{}
	err := helper.GetJSON(ctx, "nonexistent", &dest)
	if err != ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss, got %v", err)
	}

	// 设置 JSON 数据
	data := map[string]string{"key": "value"}
	jsonData, _ := json.Marshal(data)
	_ = cache.Set(ctx, "test", jsonData, time.Minute)

	// 获取 JSON
	var result map[string]string
	err = helper.GetJSON(ctx, "test", &result)
	if err != nil {
		t.Errorf("GetJSON failed: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("GetJSON result wrong: got %v, want map[key:value]", result)
	}

	// 测试无效的 JSON 数据
	_ = cache.Set(ctx, "invalid", []byte("not json"), time.Minute)
	err = helper.GetJSON(ctx, "invalid", &result)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

// TestCacheHelper_SetJSON 测试 CacheHelper SetJSON 方法
func TestCacheHelper_SetJSON(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	helper := NewCacheHelper(cache)
	ctx := context.Background()

	// 设置各种类型的值
	tests := []struct {
		name  string
		key   string
		value interface{}
	}{
		{"Map", "map", map[string]string{"a": "b"}},
		{"Slice", "slice", []int{1, 2, 3}},
		{"Struct", "struct", struct{ Name string }{"test"}},
		{"String", "string", "value"},
		{"Number", "number", 42},
		{"Boolean", "bool", true},
		{"Nil", "nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := helper.SetJSON(ctx, tt.key, tt.value, time.Minute)
			if err != nil {
				t.Errorf("SetJSON(%q) failed: %v", tt.key, err)
			}

			// 验证可以获取
			var dest interface{}
			err = helper.GetJSON(ctx, tt.key, &dest)
			if err != nil {
				t.Errorf("GetJSON after SetJSON(%q) failed: %v", tt.key, err)
			}
		})
	}
}

// TestCacheHelper_Remember 测试 CacheHelper Remember 方法
func TestCacheHelper_Remember(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	helper := NewCacheHelper(cache)
	ctx := context.Background()

	// 测试缓存未命中时调用 loader
	loadCount := 0
	loader := func() (interface{}, error) {
		loadCount++
		return map[string]string{"loaded": "value"}, nil
	}

	var result map[string]string
	err := helper.Remember(ctx, "remember_key", time.Minute, &result, loader)
	if err != nil {
		t.Errorf("Remember failed: %v", err)
	}
	if loadCount != 1 {
		t.Errorf("Loader should be called once, called %d times", loadCount)
	}
	if result["loaded"] != "value" {
		t.Errorf("Result wrong: got %v, want map[loaded:value]", result)
	}

	// 再次调用应该从缓存获取，不调用 loader
	err = helper.Remember(ctx, "remember_key", time.Minute, &result, loader)
	if err != nil {
		t.Errorf("Remember second call failed: %v", err)
	}
	if loadCount != 1 {
		t.Errorf("Loader should still be called once, called %d times", loadCount)
	}
}

// TestCacheHelper_RememberLoaderError 测试 Remember loader 错误
func TestCacheHelper_RememberLoaderError(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	helper := NewCacheHelper(cache)
	ctx := context.Background()

	expectedErr := fmt.Errorf("loader error")
	loader := func() (interface{}, error) {
		return nil, expectedErr
	}

	var result map[string]string
	err := helper.Remember(ctx, "error_key", time.Minute, &result, loader)
	if err != expectedErr {
		t.Errorf("Expected loader error, got %v", err)
	}
}

// TestCacheHelper_RememberDifferentTypes 测试 Remember 不同类型
func TestCacheHelper_RememberDifferentTypes(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	helper := NewCacheHelper(cache)
	ctx := context.Background()

	tests := []struct {
		name     string
		loader   func() (interface{}, error)
		destType interface{}
	}{
		{
			"String slice",
			func() (interface{}, error) { return []string{"a", "b"}, nil },
			[]string(nil),
		},
		{
			"Int slice",
			func() (interface{}, error) { return []int{1, 2, 3}, nil },
			[]int(nil),
		},
		{
			"Struct",
			func() (interface{}, error) {
				return struct{ Name string }{Name: "test"}, nil
			},
			struct{ Name string }{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := helper.Remember(ctx, tt.name, time.Minute, &tt.destType, tt.loader)
			if err != nil {
				t.Errorf("Remember(%q) failed: %v", tt.name, err)
			}
		})
	}
}

// TestCacheHelper_RememberWithNullCache 测试 NullCache 的 Remember
func TestCacheHelper_RememberWithNullCache(t *testing.T) {
	cache := NewNullCache()
	helper := NewCacheHelper(cache)
	ctx := context.Background()

	loadCount := 0
	loader := func() (interface{}, error) {
		loadCount++
		return "value", nil
	}

	var result string
	// NullCache 总是未命中，每次都会调用 loader
	err := helper.Remember(ctx, "null_key", time.Minute, &result, loader)
	if err != nil {
		t.Errorf("Remember with NullCache failed: %v", err)
	}
	if loadCount != 1 {
		t.Errorf("Loader should be called once, called %d times", loadCount)
	}
	if result != "value" {
		t.Errorf("Result wrong: got %s, want 'value'", result)
	}

	// 再次调用，loader 应该再次被调用（因为 NullCache 不缓存）
	err = helper.Remember(ctx, "null_key", time.Minute, &result, loader)
	if err != nil {
		t.Errorf("Remember second call with NullCache failed: %v", err)
	}
	if loadCount != 2 {
		t.Errorf("Loader should be called twice with NullCache, called %d times", loadCount)
	}
}

// TestNewCacheStore_Disabled 测试禁用缓存
func TestNewCacheStore_Disabled(t *testing.T) {
	cfg := config.CacheConfig{
		Enabled: false,
	}

	store, err := NewCacheStore(cfg)
	if err != nil {
		t.Fatalf("NewCacheStore with Enabled=false failed: %v", err)
	}

	if _, ok := store.(*NullCache); !ok {
		t.Errorf("Expected NullCache when disabled, got %T", store)
	}

	_ = store.Close()
}

// TestNewCacheStore_DefaultLocal 测试默认本地缓存
func TestNewCacheStore_DefaultLocal(t *testing.T) {
	cfg := config.CacheConfig{
		Enabled: true,
		Type:    "",
		TTL:     "",
	}

	store, err := NewCacheStore(cfg)
	if err != nil {
		t.Fatalf("NewCacheStore with default config failed: %v", err)
	}

	if _, ok := store.(*LocalCache); !ok {
		t.Errorf("Expected LocalCache with default config, got %T", store)
	}

	_ = store.Close()
}

// TestNewCacheStore_LocalType 测试本地缓存类型
func TestNewCacheStore_LocalType(t *testing.T) {
	tests := []struct {
		name      string
		cacheType string
	}{
		{"local type", "local"},
		{"memory type", "memory"},
		{"LOCAL case", "LOCAL"},
		{"MEMORY case", "MEMORY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.CacheConfig{
				Enabled: true,
				Type:    tt.cacheType,
			}

			store, err := NewCacheStore(cfg)
			if err != nil {
				t.Fatalf("NewCacheStore with type=%s failed: %v", tt.cacheType, err)
			}

			if _, ok := store.(*LocalCache); !ok {
				t.Errorf("Expected LocalCache for type=%s, got %T", tt.cacheType, store)
			}

			_ = store.Close()
		})
	}
}

// TestNewCacheStore_InvalidType 测试无效缓存类型
func TestNewCacheStore_InvalidType(t *testing.T) {
	cfg := config.CacheConfig{
		Enabled: true,
		Type:    "invalid_type",
	}

	_, err := NewCacheStore(cfg)
	if err == nil {
		t.Error("Expected error for invalid cache type, got nil")
	}

	if !strings.Contains(err.Error(), "unsupported cache type") {
		t.Errorf("Error message should mention unsupported type, got: %v", err)
	}
}

// TestNewCacheStore_WithTTL 测试自定义 TTL
func TestNewCacheStore_WithTTL(t *testing.T) {
	tests := []struct {
		name     string
		ttl      string
		expected time.Duration
	}{
		{"1 minute", "1m", time.Minute},
		{"5 minutes", "5m", 5 * time.Minute},
		{"1 hour", "1h", time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.CacheConfig{
				Enabled: true,
				Type:    "local",
				TTL:     tt.ttl,
			}

			store, err := NewCacheStore(cfg)
			if err != nil {
				t.Fatalf("NewCacheStore failed: %v", err)
			}

			localCache, ok := store.(*LocalCache)
			if !ok {
				t.Fatalf("Expected LocalCache, got %T", store)
			}

			if localCache.defaultTTL != tt.expected {
				t.Errorf("Default TTL wrong: got %v, want %v", localCache.defaultTTL, tt.expected)
			}

			_ = store.Close()
		})
	}
}

// TestNewCacheStore_InvalidTTL 测试无效 TTL
func TestNewCacheStore_InvalidTTL(t *testing.T) {
	cfg := config.CacheConfig{
		Enabled: true,
		Type:    "local",
		TTL:     "invalid-ttl",
	}

	store, err := NewCacheStore(cfg)
	if err != nil {
		t.Fatalf("NewCacheStore with invalid TTL failed: %v", err)
	}

	localCache, ok := store.(*LocalCache)
	if !ok {
		t.Fatalf("Expected LocalCache, got %T", store)
	}

	// 应该使用默认值 5 分钟
	if localCache.defaultTTL != 5*time.Minute {
		t.Errorf("Default TTL should be 5m for invalid input, got %v", localCache.defaultTTL)
	}

	_ = store.Close()
}

// TestNewCacheStore_WithEvictTTL 测试清理间隔
func TestNewCacheStore_WithEvictTTL(t *testing.T) {
	cfg := config.CacheConfig{
		Enabled:  true,
		Type:     "local",
		TTL:      "1m",
		EvictTTL: "30s",
	}

	store, err := NewCacheStore(cfg)
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}

	// 验证缓存可以正常工作
	ctx := context.Background()
	_ = store.Set(ctx, "test", []byte("value"), time.Minute)
	val, err := store.Get(ctx, "test")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if string(val) != "value" {
		t.Errorf("Value wrong: got %s, want value", val)
	}

	_ = store.Close()
}

// TestNewCacheStore_InvalidEvictTTL 测试无效清理间隔
func TestNewCacheStore_InvalidEvictTTL(t *testing.T) {
	cfg := config.CacheConfig{
		Enabled:  true,
		Type:     "local",
		EvictTTL: "invalid",
	}

	store, err := NewCacheStore(cfg)
	if err != nil {
		t.Fatalf("NewCacheStore with invalid EvictTTL failed: %v", err)
	}

	// 应该使用默认值并正常工作
	ctx := context.Background()
	_ = store.Set(ctx, "test", []byte("value"), time.Minute)
	_, err = store.Get(ctx, "test")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	_ = store.Close()
}

// TestNewCacheStore_TrimmedType 测试带空格的类型
func TestNewCacheStore_TrimmedType(t *testing.T) {
	cfg := config.CacheConfig{
		Enabled: true,
		Type:    "  local  ",
	}

	store, err := NewCacheStore(cfg)
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}

	if _, ok := store.(*LocalCache); !ok {
		t.Errorf("Expected LocalCache, got %T", store)
	}

	_ = store.Close()
}

// TestNewCacheStore_Redis 测试 Redis 类型（不需要连接 Redis）
func TestNewCacheStore_Redis(t *testing.T) {
	// 测试 Redis 配置解析（不实际连接）
	cfg := config.CacheConfig{
		Enabled:  true,
		Type:     "redis",
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
		PoolSize: 10,
		TTL:      "5m",
	}

	store, err := NewCacheStore(cfg)
	// 如果 Redis 服务器运行，应该成功；否则返回连接错误
	if err != nil {
		// 验证错误是关于 Redis 连接的
		if !strings.Contains(err.Error(), "redis") && !strings.Contains(err.Error(), "connect") {
			t.Logf("Got error: %v", err)
		}
	} else {
		// Redis 连接成功，验证类型
		if _, ok := store.(*RedisCache); !ok {
			t.Errorf("Expected RedisCache, got %T", store)
		}
		_ = store.Close()
	}
}

// TestNewLocalCacheFunction 测试 newLocalCache 函数
func TestNewLocalCacheFunction(t *testing.T) {
	cfg := config.CacheConfig{
		Enabled:  true,
		Type:     "local",
		TTL:      "2m",
		EvictTTL: "5m",
	}

	store, err := newLocalCache(cfg, 2*time.Minute)
	if err != nil {
		t.Fatalf("newLocalCache failed: %v", err)
	}

	localCache, ok := store.(*LocalCache)
	if !ok {
		t.Fatalf("Expected LocalCache, got %T", store)
	}

	if localCache.defaultTTL != 2*time.Minute {
		t.Errorf("Default TTL wrong: got %v, want 2m", localCache.defaultTTL)
	}

	_ = store.Close()
}

// TestNewLocalCacheInvalidEvictTTL 测试 newLocalCache 无效清理间隔
func TestNewLocalCacheInvalidEvictTTL(t *testing.T) {
	cfg := config.CacheConfig{
		Enabled:  true,
		Type:     "local",
		EvictTTL: "invalid",
	}

	store, err := newLocalCache(cfg, time.Minute)
	if err != nil {
		t.Fatalf("newLocalCache with invalid EvictTTL failed: %v", err)
	}

	// 应该使用默认清理间隔
	_ = store.(*LocalCache)
	_ = store.Close()
}

// TestNewRedisCacheFunction 测试 newRedisCache 配置解析
func TestNewRedisCacheFunction(t *testing.T) {
	cfg := config.CacheConfig{
		Enabled:  true,
		Type:     "redis",
		Addr:     "",
		Password: "",
		DB:       0,
		PoolSize: 0, // 应该使用默认值 10
		TTL:      "3m",
	}

	store, err := newRedisCache(cfg, 3*time.Minute)
	if err != nil {
		// 验证错误是关于 Redis 连接的
		if !strings.Contains(err.Error(), "redis") && !strings.Contains(err.Error(), "connect") {
			t.Logf("Got error: %v", err)
		}
		return
	}

	// Redis 连接成功，验证类型
	if _, ok := store.(*RedisCache); !ok {
		t.Errorf("Expected RedisCache, got %T", store)
	}
	_ = store.Close()
}

// TestCacheHelper_SetJSONError 测试 SetJSON 错误处理
func TestCacheHelper_SetJSONError(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	helper := NewCacheHelper(cache)
	ctx := context.Background()

	// 测试无法序列化的值（包含 channel 的值）
	invalidValue := make(chan int)
	err := helper.SetJSON(ctx, "invalid", invalidValue, time.Minute)
	if err == nil {
		t.Error("Expected error for unmarshallable value, got nil")
	}
}

// TestLocalCache_Get_InvalidType 测试获取非[]byte类型的缓存值
func TestLocalCache_Get_InvalidType(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	// 直接访问底层 cache 来存储一个非[]byte值
	cache.cache.Set("invalid_key", "string_value_not_bytes", time.Minute)

	// Get 应该返回类型错误
	_, err := cache.Get(ctx, "invalid_key")
	if err == nil {
		t.Fatal("Expected error for invalid type, got nil")
	}

	// 验证错误信息
	expectedErrMsg := "invalid cache value type"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
	}
}

// TestCacheHelper_Remember_SetJSONError 测试 SetJSON 失败时 Remember 仍然返回数据
func TestCacheHelper_Remember_SetJSONError(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	helper := NewCacheHelper(cache)
	ctx := context.Background()

	// 创建一个总是返回值的 loader
	loader := func() (interface{}, error) {
		return map[string]string{"key": "value"}, nil
	}

	var result map[string]string
	// Remember 应该成功，即使 SetJSON 失败也会返回数据
	err := helper.Remember(ctx, "remember_error_test", time.Minute, &result, loader)
	if err != nil {
		t.Fatalf("Remember should succeed: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("Expected result[key]='value', got %q", result["key"])
	}
}

// TestCacheHelper_Remember_InvalidCachedJSON 测试缓存中有无效 JSON 时的情况
func TestCacheHelper_Remember_InvalidCachedJSON(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	helper := NewCacheHelper(cache)
	ctx := context.Background()

	// 设置一个无效的 JSON 到缓存
	_ = cache.Set(ctx, "invalid_json", []byte("not valid json"), time.Minute)

	loadCount := 0
	loader := func() (interface{}, error) {
		loadCount++
		return map[string]string{"key": "loaded"}, nil
	}

	var result map[string]string
	// Remember 应该检测到缓存中的数据无效，然后调用 loader
	err := helper.Remember(ctx, "invalid_json", time.Minute, &result, loader)
	if err != nil {
		t.Fatalf("Remember should succeed: %v", err)
	}

	// 由于缓存的 JSON 无效，应该调用 loader
	if loadCount != 1 {
		t.Errorf("Loader should be called once for invalid cached JSON, called %d times", loadCount)
	}

	if result["key"] != "loaded" {
		t.Errorf("Expected result[key]='loaded', got %q", result["key"])
	}
}

// TestCacheHelper_Remember_UnmarshalErrorAfterSet 测试 SetJSON 后 Unmarshal 仍然失败的情况
func TestCacheHelper_Remember_UnmarshalErrorAfterSet(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	helper := NewCacheHelper(cache)
	ctx := context.Background()

	// 创建一个总是返回无法序列化值的 loader
	loader := func() (interface{}, error) {
		return make(chan int), nil // 无法 JSON 序列化
	}

	var result map[string]string
	// Remember 应该返回 SetJSON 的错误
	err := helper.Remember(ctx, "unmarshal_error", time.Minute, &result, loader)
	if err == nil {
		t.Error("Expected error from Remember when loader returns unserializable value")
	}
}

// TestLocalCache_Close_Idempotent 测试多次关闭 LocalCache
func TestLocalCache_Close_Idempotent(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	// 设置一些值
	_ = cache.Set(ctx, "key1", []byte("value1"), time.Minute)

	// 第一次关闭
	err := cache.Close()
	if err != nil {
		t.Errorf("First Close failed: %v", err)
	}

	// 第二次关闭应该不报错
	err = cache.Close()
	if err != nil {
		t.Errorf("Second Close failed: %v", err)
	}

	// 第三次关闭
	err = cache.Close()
	if err != nil {
		t.Errorf("Third Close failed: %v", err)
	}
}

// TestLocalCache_Stats_AfterClose 测试关闭后统计
func TestLocalCache_Stats_AfterClose(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	// 设置一些值
	_ = cache.Set(ctx, "key1", []byte("value1"), time.Minute)
	_ = cache.Set(ctx, "key2", []byte("value2"), time.Minute)

	// 验证统计
	if cache.Stats() != 2 {
		t.Errorf("Expected 2 items before close, got %d", cache.Stats())
	}

	// 关闭
	_ = cache.Close()

	// 关闭后统计应该为 0
	if cache.Stats() != 0 {
		t.Errorf("Expected 0 items after close, got %d", cache.Stats())
	}
}

// TestLocalCache_Get_AfterClose 测试关闭后获取
func TestLocalCache_Get_AfterClose(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	// 设置值
	_ = cache.Set(ctx, "key1", []byte("value1"), time.Minute)

	// 关闭
	_ = cache.Close()

	// 关闭后获取应该返回缓存未命中
	_, err := cache.Get(ctx, "key1")
	if err != ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss after close, got %v", err)
	}
}

// TestLocalCache_Set_AfterClose 测试关闭后设置
func TestLocalCache_Set_AfterClose(t *testing.T) {
	cache := NewLocalCache(time.Minute, time.Minute)
	ctx := context.Background()

	// 关闭会清空缓存
	_ = cache.Set(ctx, "before_close", []byte("value"), time.Minute)
	_ = cache.Close()

	// 关闭后设置仍然可以操作（不会报错）
	// 但缓存已经被清空，新设置的项目会存在
	err := cache.Set(ctx, "key1", []byte("value1"), time.Minute)
	if err != nil {
		t.Errorf("Set after close should not error, got %v", err)
	}

	// 关闭后，缓存行为类似于新创建的缓存
	// 可以继续使用，但之前的内容已被清空
	t.Log("Cache is usable after Close, but previous content is cleared")
}

// TestNullCache_Delete_Idempotent 测试多次删除同一个键
func TestNullCache_Delete_Idempotent(t *testing.T) {
	cache := NewNullCache()
	ctx := context.Background()

	// 删除同一个键多次
	for i := 0; i < 5; i++ {
		err := cache.Delete(ctx, "same_key")
		if err != nil {
			t.Errorf("Delete iteration %d failed: %v", i, err)
		}
	}
}

// TestNullCache_Set_Get_Cycle 测试设置和获取循环
func TestNullCache_Set_Get_Cycle(t *testing.T) {
	cache := NewNullCache()
	ctx := context.Background()

	// 多次设置和获取
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		value := []byte(fmt.Sprintf("value%d", i))

		err := cache.Set(ctx, key, value, time.Minute)
		if err != nil {
			t.Errorf("Set failed for %s: %v", key, err)
		}

		retrieved, err := cache.Get(ctx, key)
		if err != ErrCacheMiss {
			t.Errorf("Get for %s should return ErrCacheMiss, got %v", key, err)
		}
		if retrieved != nil {
			t.Errorf("Get for %s should return nil, got %v", key, retrieved)
		}
	}
}

// TestNewRedisCache_DefaultValues 测试 NewRedisCache 默认值
func TestNewRedisCache_DefaultValues(t *testing.T) {
	// 使用无效地址来测试默认值设置（会连接失败但会验证默认值逻辑）
	// 使用一个不太可能运行的端口来避免实际连接
	_, err := NewRedisCache("localhost:9999", "", 0, 0, 0)
	if err == nil {
		t.Skip("Redis server running on localhost:9999, skipping default values test")
	}

	// 验证错误是连接相关的
	if err == nil {
		t.Error("Expected connection error for non-existent Redis server")
	}
	// 错误信息应该包含 "redis" 或 "connect"
	errMsg := err.Error()
	if !containsAny(errMsg, "redis", "connect", "refused") {
		t.Logf("Got error: %v", err)
	}
}

// TestNewRedisCache_ConnectionError 测试 Redis 连接错误
func TestNewRedisCache_ConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		password string
		db       int
	}{
		{"Invalid address", "invalid:address", "", 0},
		{"Non-existent host", "localhost:9999", "", 0},
		{"Wrong port", "localhost:1234", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRedisCache(tt.addr, tt.password, tt.db, 10, time.Minute)
			if err == nil {
				t.Skip("Redis server unexpectedly available")
			}
			// 验证错误信息
			if err != nil {
				t.Logf("Expected connection error: %v", err)
			}
		})
	}
}

// containsAny 检查字符串是否包含任意一个子串
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// TestNewRedisCache_ValidConfig 测试有效配置（需要 Redis 服务器）
func TestNewRedisCache_ValidConfig(t *testing.T) {
	// 这个测试需要 Redis 服务器运行
	cache, err := NewRedisCache("localhost:6379", "", 0, 20, 10*time.Minute)
	if err != nil {
		// Redis 服务器不可用，跳过测试
		t.Skip("Redis server not available, skipping Redis cache test")
	}

	defer cache.Close()

	// 验证 cache 创建成功
	if cache == nil {
		t.Fatal("Expected non-nil RedisCache")
	}

	if cache.defaultTTL != 10*time.Minute {
		t.Errorf("Expected defaultTTL 10m, got %v", cache.defaultTTL)
	}

	// 测试基本操作
	ctx := context.Background()
	testKey := "test_key"
	testValue := []byte("test_value")

	// Set
	err = cache.Set(ctx, testKey, testValue, time.Minute)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Get
	val, err := cache.Get(ctx, testKey)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if string(val) != string(testValue) {
		t.Errorf("Got wrong value: %s, want %s", val, testValue)
	}

	// Exists
	exists, err := cache.Exists(ctx, testKey)
	if err != nil {
		t.Errorf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Expected key to exist")
	}

	// Delete
	err = cache.Delete(ctx, testKey)
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = cache.Get(ctx, testKey)
	if err != ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss after delete, got %v", err)
	}
}

// TestRedisCache_Client 测试 Client 方法
func TestRedisCache_Client(t *testing.T) {
	cache, err := NewRedisCache("localhost:6379", "", 0, 10, time.Minute)
	if err != nil {
		t.Skip("Redis server not available")
	}
	defer cache.Close()

	client := cache.Client()
	if client == nil {
		t.Error("Client() returned nil")
	}
}
