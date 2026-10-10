package serverstatus

import (
	"sync"
	"time"
)

// Cache 状态查询结果的进程内 TTL 缓存（防告警风暴打爆状态源；设计 §2）。
// 只缓存成功结果——超时/错误不缓存，下次再查（fail-open 语义不受缓存影响）；
// TTL 内的维护窗变更最多延迟 TTL 生效（已知边界，默认 60s）。
type Cache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]cacheEntry
	now     func() time.Time
}

type cacheEntry struct {
	status    ServerStatus
	expiresAt time.Time
}

// NewCache 创建 TTL 缓存。ttl<=0 取默认 60s。
func NewCache(ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &Cache{
		ttl:     ttl,
		entries: map[string]cacheEntry{},
		now:     time.Now,
	}
}

// Get 命中未过期条目返回 (status, true)；过期/不存在返回 false（并惰性清理）。
func (c *Cache) Get(ref ServerRef) (ServerStatus, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	e, ok := c.entries[ref.CacheKey()]
	if !ok {
		return ServerStatus{}, false
	}
	if !now.Before(e.expiresAt) {
		delete(c.entries, ref.CacheKey())
		return ServerStatus{}, false
	}
	return e.status, true
}

// Set 写入成功结果（TTL 自写入时刻起算）。
func (c *Cache) Set(ref ServerRef, st ServerStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[ref.CacheKey()] = cacheEntry{status: st, expiresAt: c.now().Add(c.ttl)}
}
