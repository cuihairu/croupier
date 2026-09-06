package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// failingPipelineHook 仅让 pipeline 执行失败，单命令（SCAN/SET）透传，
// 用于触发 DeletePattern 每 100 键批量删除时 pipe.Exec 的错误分支。
type failingPipelineHook struct{}

func (failingPipelineHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (failingPipelineHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook { return next }

func (failingPipelineHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		return errors.New("pipeline forced failure")
	}
}

func TestRedisCache_GetCommandErrorV2(t *testing.T) {
	mr := startMiniRedis(t)
	ctx := context.Background()

	cache, err := NewRedisCache(mr.Addr(), "", 0, 10, time.Minute)
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	mr.SetError("LOADING server is busy")
	_, err = cache.Get(ctx, "k1")
	if err == nil {
		t.Fatal("expected command error from Get")
	}
	if errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expected non-miss error, got ErrCacheMiss")
	}
}

func TestRedisCache_ExistsCommandErrorV2(t *testing.T) {
	mr := startMiniRedis(t)
	ctx := context.Background()

	cache, err := NewRedisCache(mr.Addr(), "", 0, 10, time.Minute)
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	mr.SetError("LOADING server is busy")
	if _, err := cache.Exists(ctx, "k1"); err == nil {
		t.Fatal("expected command error from Exists")
	}
}

func TestRedisCache_DeletePatternScanErrorV2(t *testing.T) {
	mr := startMiniRedis(t)
	ctx := context.Background()

	cache, err := NewRedisCache(mr.Addr(), "", 0, 10, time.Minute)
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	mr.SetError("LOADING server is busy")
	if err := cache.DeletePattern(ctx, "any:*"); err == nil {
		t.Fatal("expected scan error from DeletePattern")
	}
}

func TestRedisCache_DeletePatternPipelineFlushErrorV2(t *testing.T) {
	mr := startMiniRedis(t)
	ctx := context.Background()

	cache, err := NewRedisCache(mr.Addr(), "", 0, 10, time.Minute)
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	for i := 0; i < 150; i++ {
		if err := cache.Set(ctx, "flush:"+itoa(i), []byte("v"), time.Minute); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}

	cache.Client().AddHook(failingPipelineHook{})
	err = cache.DeletePattern(ctx, "flush:*")
	if err == nil {
		t.Fatal("expected mid-loop pipeline flush error from DeletePattern")
	}
	if got := err.Error(); got != "pipeline forced failure" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRedisCache_DeletePatternExactBatchReturnsNilV2(t *testing.T) {
	mr := startMiniRedis(t)
	ctx := context.Background()

	cache, err := NewRedisCache(mr.Addr(), "", 0, 10, time.Minute)
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	// 恰好 100 个键：第 100 个键触发批内 flush，循环结束后 count%100 == 0，
	// 跳过剩余 flush 并直接 return nil。
	for i := 0; i < 100; i++ {
		if err := cache.Set(ctx, "exact:"+itoa(i), []byte("v"), time.Minute); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	if err := cache.DeletePattern(ctx, "exact:*"); err != nil {
		t.Fatalf("DeletePattern exact batch: %v", err)
	}
	if exists, _ := cache.Exists(ctx, "exact:0"); exists {
		t.Fatal("keys should be deleted after exact-batch pattern delete")
	}
}
