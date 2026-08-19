package cache

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cuihairu/croupier/internal/config"
)

func startMiniRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	return mr
}

func TestRedisCache_NewWithDefaults(t *testing.T) {
	mr := startMiniRedis(t)

	cache, err := NewRedisCache(mr.Addr(), "", 0, 0, 0)
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	if cache.defaultTTL != 5*time.Minute {
		t.Errorf("default TTL = %v, want 5m", cache.defaultTTL)
	}
}

func TestRedisCache_NewConnectionError(t *testing.T) {
	// Nothing listens on this port; Ping must fail fast.
	_, err := NewRedisCache("127.0.0.1:1", "", 0, 10, time.Minute)
	if err == nil {
		t.Fatal("expected connection error for unreachable redis")
	}
	if !strings.Contains(err.Error(), "failed to connect to redis") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRedisCache_GetSetDeleteExists(t *testing.T) {
	mr := startMiniRedis(t)
	ctx := context.Background()

	cache, err := NewRedisCache(mr.Addr(), "", 0, 10, time.Minute)
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	// Miss maps to ErrCacheMiss.
	if _, err := cache.Get(ctx, "missing"); err != ErrCacheMiss {
		t.Fatalf("expected ErrCacheMiss, got %v", err)
	}

	if err := cache.Set(ctx, "k1", []byte("v1"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := cache.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "v1" {
		t.Fatalf("Get = %q, want v1", got)
	}

	exists, err := cache.Exists(ctx, "k1")
	if err != nil || !exists {
		t.Fatalf("Exists(k1) = %v, %v; want true", exists, err)
	}
	exists, err = cache.Exists(ctx, "missing")
	if err != nil || exists {
		t.Fatalf("Exists(missing) = %v, %v; want false", exists, err)
	}

	if err := cache.Delete(ctx, "k1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := cache.Get(ctx, "k1"); err != ErrCacheMiss {
		t.Fatalf("expected ErrCacheMiss after delete, got %v", err)
	}
}

func TestRedisCache_SetUsesDefaultTTL(t *testing.T) {
	mr := startMiniRedis(t)
	ctx := context.Background()

	cache, err := NewRedisCache(mr.Addr(), "", 0, 10, 2*time.Hour)
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	if err := cache.Set(ctx, "ttlkey", []byte("v"), 0); err != nil {
		t.Fatalf("Set with zero ttl: %v", err)
	}
	ttl := mr.TTL("ttlkey")
	if ttl <= 0 || ttl > 2*time.Hour {
		t.Fatalf("TTL = %v, want the default 2h", ttl)
	}
}

func TestRedisCache_DeletePattern(t *testing.T) {
	mr := startMiniRedis(t)
	ctx := context.Background()

	cache, err := NewRedisCache(mr.Addr(), "", 0, 10, time.Minute)
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	for i := 0; i < 3; i++ {
		if err := cache.Set(ctx, "pattern:key:"+string(rune('a'+i)), []byte("v"), time.Minute); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	if err := cache.Set(ctx, "other:key", []byte("v"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if err := cache.DeletePattern(ctx, "pattern:key:*"); err != nil {
		t.Fatalf("DeletePattern: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := cache.Get(ctx, "pattern:key:"+string(rune('a'+i))); err != ErrCacheMiss {
			t.Fatalf("pattern key should be deleted, got %v", err)
		}
	}
	if exists, _ := cache.Exists(ctx, "other:key"); !exists {
		t.Fatal("unrelated key must survive pattern delete")
	}
}

func TestRedisCache_DeletePattern_BatchFlush(t *testing.T) {
	mr := startMiniRedis(t)
	ctx := context.Background()

	cache, err := NewRedisCache(mr.Addr(), "", 0, 10, time.Minute)
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	// More than 100 keys forces the mid-loop pipeline flush branch.
	for i := 0; i < 150; i++ {
		if err := cache.Set(ctx, "bulk:"+string(rune('0'+i%10))+":"+itoa(i), []byte("v"), time.Minute); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	if err := cache.DeletePattern(ctx, "bulk:*"); err != nil {
		t.Fatalf("DeletePattern: %v", err)
	}
	if exists, _ := cache.Exists(ctx, "bulk:0:0"); exists {
		t.Fatal("bulk keys should be deleted")
	}
}

func TestRedisCache_ClientAccessor(t *testing.T) {
	mr := startMiniRedis(t)

	cache, err := NewRedisCache(mr.Addr(), "", 0, 10, time.Minute)
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	if cache.Client() == nil {
		t.Fatal("Client() must expose the underlying client")
	}
	if err := cache.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestNewRedisCache_FromConfig(t *testing.T) {
	mr := startMiniRedis(t)

	store, err := NewCacheStore(config.CacheConfig{
		Enabled: true,
		Type:    "redis",
		Addr:    mr.Addr(),
	})
	if err != nil {
		t.Fatalf("NewCacheStore(redis): %v", err)
	}
	rc, ok := store.(*RedisCache)
	if !ok {
		t.Fatalf("expected *RedisCache, got %T", store)
	}
	if rc.client.Options().Addr != mr.Addr() {
		t.Fatalf("redis addr = %v", rc.client.Options().Addr)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := ""
	for i > 0 {
		digits = string(rune('0'+i%10)) + digits
		i /= 10
	}
	return digits
}
