// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package croupier

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
)

// drain 请求：置位状态、立即回空确认、幂等；drain 期间新 Invoke 被拒。
func TestDrainHandler_AcksIdempotentAndRejectsInvoke(t *testing.T) {
	handler := newTestRPCHandler(t)
	handler.manager.handlers["test.fn"] = func(ctx context.Context, payload []byte) ([]byte, error) {
		return []byte(`{}`), nil
	}

	// 占住在途计数：handleDrain 会在后台启动 drainAndRecover，在途清零后
	// draining 被异步清除——不占住的话断言与恢复 goroutine 产生调度竞争。
	handler.manager.inflightCalls.Add(1)
	releaseInflight := func() {
		handler.manager.inflightCalls.Add(-1)
		deadline := time.Now().Add(2 * time.Second)
		for handler.manager.draining.Load() && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
	}
	defer releaseInflight()

	drainReq, _ := proto.Marshal(&sdkv1.ProviderDrainRequest{SessionId: "s-1", Reason: "rolling-restart", RetryAfterMs: 1000})
	respBody, err := handler.handleDrain(context.Background(), protocol.MsgProviderDrainRequest, 1, drainReq)
	if err != nil {
		t.Fatalf("handleDrain: %v", err)
	}
	if err := proto.Unmarshal(respBody, &sdkv1.ProviderDrainResponse{}); err != nil {
		t.Fatalf("response must be ProviderDrainResponse: %v", err)
	}
	if !handler.manager.draining.Load() {
		t.Fatal("draining must be set after first drain request")
	}

	// 幂等：重复 drain 不重复触发恢复
	if _, err := handler.handleDrain(context.Background(), protocol.MsgProviderDrainRequest, 2, drainReq); err != nil {
		t.Fatalf("idempotent drain: %v", err)
	}

	// drain 期间新 Invoke 被拒：返回 provider is draining 错误 payload，handler 不执行
	called := false
	handler.manager.handlers["test.fn"] = func(ctx context.Context, payload []byte) ([]byte, error) {
		called = true
		return []byte(`{}`), nil
	}
	invokeReq, _ := proto.Marshal(&sdkv1.InvokeRequest{FunctionId: "test.fn"})
	respBody, err = handler.invoke(context.Background(), protocol.MsgInvokeRequest, 3, invokeReq)
	if err != nil {
		t.Fatalf("invoke during drain must not be transport error: %v", err)
	}
	if called {
		t.Fatal("handler must not run while draining")
	}
	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(respBody, resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(resp.Payload) != `{"error":"provider is draining"}` {
		t.Fatalf("unexpected draining payload: %s", resp.Payload)
	}

	// 恢复等待在途为 0 后 handleDisconnect：无 onDisconnect、Reconnect nil → 断开
	if handler.manager.config.Reconnect != nil && handler.manager.config.Reconnect.Enabled {
		handler.manager.config.Reconnect.Enabled = false
	}
	releaseInflight()
	handler.manager.drainAndRecover()
	if handler.manager.draining.Load() {
		t.Fatal("draining must clear after recovery")
	}
}

// 在途计数：invoke 期间 inflight>0，drainAndRecover 等待其清零。
func TestDrain_WaitsForInflightCalls(t *testing.T) {
	handler := newTestRPCHandler(t)
	handler.manager.config.Reconnect = nil
	handler.manager.handlers["slow.fn"] = func(ctx context.Context, payload []byte) ([]byte, error) {
		time.Sleep(200 * time.Millisecond)
		return []byte(`{}`), nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = handler.invoke(context.Background(), protocol.MsgInvokeRequest, 1, mustMarshal(t, &sdkv1.InvokeRequest{FunctionId: "slow.fn"}))
	}()
	time.Sleep(50 * time.Millisecond) // 等 invoke 进入 handler
	if n := handler.manager.inflightCalls.Load(); n != 1 {
		t.Fatalf("inflight = %d, want 1", n)
	}

	recovered := make(chan struct{})
	go func() {
		handler.manager.draining.Store(true)
		handler.manager.drainAndRecover()
		close(recovered)
	}()
	select {
	case <-recovered:
		t.Fatal("drainAndRecover must wait for in-flight call")
	case <-time.After(100 * time.Millisecond):
	}
	<-done
	select {
	case <-recovered:
	case <-time.After(time.Second):
		t.Fatal("drainAndRecover did not finish after in-flight completed")
	}
}

// drain 后经 handleDisconnect 触发 onDisconnect（对齐既有重连编排入口）。
func TestDrain_FiresOnDisconnectForReconnect(t *testing.T) {
	handler := newTestRPCHandler(t)
	handler.manager.config.Reconnect = &ReconnectConfig{Enabled: true}
	var fired atomic.Bool
	handler.manager.onDisconnect = func() { fired.Store(true) }
	handler.manager.connected = true
	handler.manager.draining.Store(true)

	handler.manager.drainAndRecover()

	if !fired.Load() {
		t.Fatal("onDisconnect must fire so client.go reconnect loop takes over")
	}
}

func mustMarshal(t *testing.T, m proto.Message) []byte {
	t.Helper()
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// 单线程游戏服兼容模式：InboundWorkers=1 时入站业务车道只有 1 个 worker
// （handler 串行执行、互不并发）；心跳走控制车道不受影响。
func TestInboundWorkers_SerialMode(t *testing.T) {
	cfg := DefaultClientConfig()
	cfg.InboundWorkers = 1
	manager, err := NewTCPManager(*cfg, map[string]FunctionHandler{})
	if err != nil {
		t.Fatalf("NewTCPManager: %v", err)
	}
	m := manager.(*TCPManager)
	// 未连接时 inbox 未创建；直接校验 transport 配置传递链
	if m.config.InboundWorkers != 1 {
		t.Fatalf("config.InboundWorkers = %d, want 1", m.config.InboundWorkers)
	}
}
