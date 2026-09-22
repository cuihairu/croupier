package cache

import (
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
)

// newRedisCache 的连接失败分支：helper 层对 NewRedisCache 的包装错误必须
// 透传（含 "failed to create redis cache" 上下文）。127.0.0.1:1 是保留的
// 拒连地址，Ping 必然失败（与 redis_coverage_test.go:39 同手法），确定性
// 无时序依赖。
func TestNewRedisCacheHelper_ConnectionFailure(t *testing.T) {
	cfg := config.CacheConfig{
		Addr: "127.0.0.1:1",
	}
	store, err := newRedisCache(cfg, time.Minute)
	if err == nil {
		if store != nil {
			defer func() { _ = store.Close() }()
		}
		t.Fatal("连接不可达的 redis 地址应返回错误")
	}
	if !strings.Contains(err.Error(), "failed to create redis cache") {
		t.Fatalf("错误应携带 helper 层包装上下文, got: %v", err)
	}
	if store != nil {
		t.Fatal("失败时不应返回 store 实例")
	}
}
