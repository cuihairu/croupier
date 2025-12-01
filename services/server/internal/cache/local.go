package cache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// LocalCache 本地内存缓存实现（使用 go-cache，类似 Java Caffeine）
type LocalCache struct {
	cache      *gocache.Cache
	defaultTTL time.Duration
	mu         sync.RWMutex
}

// NewLocalCache 创建本地缓存实例
func NewLocalCache(defaultTTL, cleanupInterval time.Duration) *LocalCache {
	if defaultTTL == 0 {
		defaultTTL = 5 * time.Minute
	}
	if cleanupInterval == 0 {
		cleanupInterval = 10 * time.Minute
	}

	return &LocalCache{
		cache:      gocache.New(defaultTTL, cleanupInterval),
		defaultTTL: defaultTTL,
	}
}

func (c *LocalCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, found := c.cache.Get(key)
	if !found {
		return nil, ErrCacheMiss
	}

	data, ok := val.([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid cache value type")
	}

	return data, nil
}

func (c *LocalCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.defaultTTL
	}
	c.cache.Set(key, value, ttl)
	return nil
}

func (c *LocalCache) Delete(ctx context.Context, key string) error {
	c.cache.Delete(key)
	return nil
}

func (c *LocalCache) DeletePattern(ctx context.Context, pattern string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 简单的通配符匹配：支持 * 结尾
	isPrefix := strings.HasSuffix(pattern, "*")
	prefix := strings.TrimSuffix(pattern, "*")

	items := c.cache.Items()
	for key := range items {
		if isPrefix && strings.HasPrefix(key, prefix) {
			c.cache.Delete(key)
		} else if key == pattern {
			c.cache.Delete(key)
		}
	}

	return nil
}

func (c *LocalCache) Exists(ctx context.Context, key string) (bool, error) {
	_, found := c.cache.Get(key)
	return found, nil
}

func (c *LocalCache) Close() error {
	c.cache.Flush()
	return nil
}

// Stats 返回缓存统计信息
func (c *LocalCache) Stats() (itemCount int) {
	return c.cache.ItemCount()
}
