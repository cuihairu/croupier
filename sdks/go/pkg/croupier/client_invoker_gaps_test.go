package croupier

import (
	"context"
	"testing"
	"time"
)

// client.Connect 注册的断连回调：disconnectCh 已满时再次触发必须走 default
// 分支（非阻塞投递），不得阻塞回调方。
func TestClientDisconnectCallbackNonBlockingWhenChannelFull(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-dd"))
	c := NewClient(newTestClientConfig(agent.addr()))
	cl := c.(*client)
	if err := cl.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	tcp := cl.manager.(*TCPManager)
	tcp.mu.Lock()
	onDisconnect := tcp.onDisconnect
	tcp.mu.Unlock()
	if onDisconnect == nil {
		t.Fatal("Connect must register an onDisconnect callback")
	}

	done := make(chan struct{})
	go func() {
		onDisconnect() // 第一次投递填满 buffered channel
		onDisconnect() // 第二次命中 default 分支
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("callback blocked on full disconnect channel")
	}

	_ = cl.Close()
}

// tcpInvoker.connect 的快速路径：已连接时直接返回 nil。
func TestTCPInvokerConnectFastPathWhenConnected(t *testing.T) {
	inv := &tcpInvoker{}
	inv.connected = true
	if err := inv.connect(context.Background()); err != nil {
		t.Fatalf("connect fast path = %v, want nil", err)
	}
}
