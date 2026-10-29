// ARCH-017：ServiceContext.Close 冲刷留痕 + 释放连接池的语义验证。
// Close 须：停止/冲刷异步执行留痕写入器、取消 retention、关闭缓存、
// 关闭元库连接池；幂等可重复调用。
package svc

import (
	"context"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// countingCache 是 CacheStore 的计数替身，验证 Close 恰被调用一次。
type countingCache struct{ closed int }

func (c *countingCache) Get(ctx context.Context, key string) ([]byte, error) { return nil, nil }
func (c *countingCache) Set(ctx context.Context, key string, v []byte, ttl time.Duration) error {
	return nil
}
func (c *countingCache) Delete(ctx context.Context, key string) error { return nil }
func (c *countingCache) DeletePattern(ctx context.Context, pattern string) error {
	return nil
}
func (c *countingCache) Exists(ctx context.Context, key string) (bool, error) { return false, nil }
func (c *countingCache) Close() error                                         { c.closed++; return nil }

func TestServiceContextCloseReleasesPools(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	cache := &countingCache{}
	ctx := &ServiceContext{DB: db, Cache: cache}

	ctx.Close()
	require.Equal(t, 1, cache.closed, "Close 应关闭缓存")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.Error(t, sqlDB.Ping(), "元库连接池应已关闭")

	// 幂等：二次 Close 不 panic、缓存不重复关。
	require.NotPanics(t, func() { ctx.Close() })
	require.Equal(t, 1, cache.closed)
}

func TestServiceContextCloseNilReceiver(t *testing.T) {
	var ctx *ServiceContext
	require.NotPanics(t, func() { ctx.Close() })
}
