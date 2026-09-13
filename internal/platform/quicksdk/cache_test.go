package quicksdk

import (
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	c := newCache(5 * time.Minute)
	defer c.Close()

	resp := &Response{Status: true, Message: "ok"}
	c.Set("key1", resp)

	got, found := c.Get("key1")
	if !found {
		t.Fatal("expected to find key1")
	}
	if got.Message != "ok" {
		t.Errorf("expected message 'ok', got %q", got.Message)
	}
}

func TestCache_GetMissing(t *testing.T) {
	c := newCache(5 * time.Minute)
	defer c.Close()

	_, found := c.Get("nonexistent")
	if found {
		t.Error("expected not found for missing key")
	}
}

func TestCache_Delete(t *testing.T) {
	c := newCache(5 * time.Minute)
	defer c.Close()

	c.Set("key1", &Response{Status: true})
	c.Delete("key1")

	_, found := c.Get("key1")
	if found {
		t.Error("expected key1 to be deleted")
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	c := newCache(50 * time.Millisecond)
	defer c.Close()

	c.Set("key1", &Response{Status: true})

	// Should exist immediately
	_, found := c.Get("key1")
	if !found {
		t.Fatal("expected key1 to exist immediately after set")
	}

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	_, found = c.Get("key1")
	if found {
		t.Error("expected key1 to expire")
	}
}

func TestCache_Overwrite(t *testing.T) {
	c := newCache(5 * time.Minute)
	defer c.Close()

	c.Set("key1", &Response{Status: true, Message: "first"})
	c.Set("key1", &Response{Status: true, Message: "second"})

	got, found := c.Get("key1")
	if !found {
		t.Fatal("expected to find key1")
	}
	if got.Message != "second" {
		t.Errorf("expected message 'second', got %q", got.Message)
	}
}

func TestCache_Close(t *testing.T) {
	c := newCache(5 * time.Minute)
	c.Set("key1", &Response{Status: true})

	// Close should not panic
	c.Close()
}

func TestCache_Cleanup(t *testing.T) {
	// Use a short TTL so items expire quickly
	c := newCache(50 * time.Millisecond)

	c.Set("expired", &Response{Status: true})

	// Wait for item to expire and cleanup to run
	// Cleanup runs every minute, but we test the mechanism directly
	time.Sleep(100 * time.Millisecond)

	c.mu.Lock()
	now := time.Now()
	for k, item := range c.items {
		if now.After(item.expireAt) {
			delete(c.items, k)
		}
	}
	c.mu.Unlock()

	_, found := c.Get("expired")
	if found {
		t.Error("expected expired item to be cleaned up")
	}

	c.Close()
}

func TestCache_CleanupTickerRemovesExpired(t *testing.T) {
	// 真正走 cleanup() 的 ticker 分支（区别于上方手动模拟的 TestCache_Cleanup）：
	// 生产 ticker 周期为 1 分钟，此处经 cleanupInterval 注入口缩短为毫秒级，
	// 验证后台循环会把过期项从 items 物理删除。
	c := &cache{
		items:           make(map[string]*cacheItem),
		ttl:             30 * time.Millisecond,
		done:            make(chan struct{}),
		cleanupInterval: 10 * time.Millisecond,
	}
	// newCache 会启动 cleanup goroutine，此处手动构造 struct 故需自行启动。
	go c.cleanup()
	defer c.Close()

	c.Set("expired", &Response{Status: true})
	// 手动塞入未过期项，验证 ticker 循环只删过期项、不误删存活项。
	c.mu.Lock()
	c.items["not-expired"] = &cacheItem{value: &Response{Status: true}, expireAt: time.Now().Add(time.Hour)}
	c.mu.Unlock()

	// 轮询等待 ticker 至少触发一轮：过期项必须从 items map 物理消失
	//（Get 的惰性过期路径不触碰 items，物理消失只能由 ticker 分支造成）。
	deadline := time.Now().Add(3 * time.Second)
	for {
		c.mu.RLock()
		_, expiredGone := c.items["expired"]
		c.mu.RUnlock()
		if !expiredGone {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expired item was not removed by cleanup ticker within deadline")
		}
		time.Sleep(5 * time.Millisecond)
	}

	c.mu.RLock()
	_, stillThere := c.items["not-expired"]
	c.mu.RUnlock()
	if !stillThere {
		t.Error("cleanup ticker must not remove items that have not expired")
	}
}
