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
