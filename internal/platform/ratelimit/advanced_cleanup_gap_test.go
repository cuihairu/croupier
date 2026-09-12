package ratelimit

import (
	"context"
	"testing"
	"time"
)

// TestSlidingWindowCleanupKeepsWindowWithFutureTimestamp：cleanup 巡检到
// 「仍有年轻时间戳」的窗口时必须保留该窗口（allOld=false 分支）。
// 构造法：直接注入一个未来时间戳——任何 tick 时它都不会过期，分支命中
// 与具体调度时机无关（不依赖精确时序）。
func TestSlidingWindowCleanupKeepsWindowWithFutureTimestamp(t *testing.T) {
	sw := NewSlidingWindowLimiter(5, 40*time.Millisecond)
	defer func() { _ = sw.Close() }()

	// 先经正常路径建窗，再注入一个未来时间戳使其在 cleanup 视角恒为年轻。
	if _, err := sw.Allow(context.Background(), "keep-me"); err != nil {
		t.Fatalf("Allow: %v", err)
	}
	sw.mu.Lock()
	if w, ok := sw.windows["keep-me"]; ok {
		w.mu.Lock()
		w.timestamps = []time.Time{time.Now().Add(time.Hour)}
		w.mu.Unlock()
	}
	sw.mu.Unlock()

	// 等待至少一个 cleanup tick（间隔 = windowSize*2 = 80ms），留 3 倍余量。
	time.Sleep(240 * time.Millisecond)

	if sw.GetStats("keep-me") == nil {
		t.Fatal("window with young timestamp must survive cleanup")
	}
}
