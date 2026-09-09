package croupier

import (
	"context"
	"testing"
	"time"
)

// 覆盖 tcpInvoker.connect 慢路径获取写锁后的 double-check：
// 快照读取时未连接、等待写锁期间另一持有者完成连接 → 直接返回成功。
func TestTCPInvoker_Connect_DoubleCheckAfterWriteLock(t *testing.T) {
	i := newTCPInvoker(&InvokerConfig{
		Address:  "127.0.0.1:19090",
		Insecure: true,
	}).(*tcpInvoker)

	ready := make(chan struct{})
	done := make(chan error, 1)

	// 持锁者：先占用写锁让 connect 阻塞在慢路径，随后标记已连接并释放。
	go func() {
		i.mu.Lock()
		close(ready)
		time.Sleep(50 * time.Millisecond)
		i.connected = true
		i.mu.Unlock()
	}()

	<-ready
	go func() { done <- i.connect(context.Background()) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("connect should succeed via double-check, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("connect did not return")
	}

	i.mu.Lock()
	connected := i.connected
	i.mu.Unlock()
	if !connected {
		t.Fatal("connected flag must stay true")
	}
}
