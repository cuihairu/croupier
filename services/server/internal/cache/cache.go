package cache

import (
	"context"
	"time"
)

// CacheStore 定义缓存接口
type CacheStore interface {
	// Get 获取缓存值
	Get(ctx context.Context, key string) ([]byte, error)

	// Set 设置缓存值
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete 删除缓存
	Delete(ctx context.Context, key string) error

	// DeletePattern 删除匹配模式的所有缓存
	DeletePattern(ctx context.Context, pattern string) error

	// Exists 检查键是否存在
	Exists(ctx context.Context, key string) (bool, error)

	// Close 关闭缓存连接
	Close() error
}

// NullCache 空缓存实现（禁用缓存时使用）
type NullCache struct{}

func NewNullCache() *NullCache {
	return &NullCache{}
}

func (c *NullCache) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, ErrCacheMiss
}

func (c *NullCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return nil
}

func (c *NullCache) Delete(ctx context.Context, key string) error {
	return nil
}

func (c *NullCache) DeletePattern(ctx context.Context, pattern string) error {
	return nil
}

func (c *NullCache) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil
}

func (c *NullCache) Close() error {
	return nil
}

// ErrCacheMiss 缓存未命中错误
var ErrCacheMiss = &CacheError{Message: "cache miss"}

// CacheError 缓存错误
type CacheError struct {
	Message string
}

func (e *CacheError) Error() string {
	return e.Message
}

// IsCacheMiss 判断是否为缓存未命中
func IsCacheMiss(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*CacheError)
	return ok || err == ErrCacheMiss
}
