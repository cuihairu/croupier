package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// ---- 测试基件 ----

type stubRemoteForwarder struct {
	calls    []string
	response []byte
	err      error
}

func (s *stubRemoteForwarder) ForwardInvoke(_ context.Context, agentID, _ string, _ []byte, _ map[string]string, _ string) ([]byte, error) {
	s.calls = append(s.calls, agentID)
	if s.err != nil {
		return nil, s.err
	}
	return s.response, nil
}

func newFailoverDispatcher(t *testing.T, agentIDs ...string) *Dispatcher {
	t.Helper()
	d := NewDispatcher(nil)
	now := time.Now().Add(time.Hour)
	for _, id := range agentIDs {
		d.store.UpsertAgent(&reg.AgentSession{
			AgentID:  id,
			Addr:     "127.0.0.1:9001",
			ExpireAt: now,
			Functions: map[string]reg.FunctionMeta{
				"test-func": {Enabled: true},
			},
		})
	}
	return d
}

// ---- 用例 ----

// 本地 miss → remoteForwarder 命中：调用经转发完成，不再报错。
func TestInvokeRequest_RemoteForwardOnLocalMiss(t *testing.T) {
	d := newFailoverDispatcher(t, "agent-1")
	// 无本地 session resolver：本地必然 miss。
	fwd := &stubRemoteForwarder{response: mustMarshalInvokeOK(t)}
	d.SetRemoteForwarder(fwd)

	resp, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err != nil {
		t.Fatalf("InvokeRequest: %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if len(fwd.calls) != 1 || fwd.calls[0] != "agent-1" {
		t.Fatalf("forward calls = %v, want [agent-1]", fwd.calls)
	}
}

// 无 forwarder 时本地 miss 保持既有语义：报错且带 unreachable 标记。
func TestInvokeRequest_LocalMissWithoutForwarder(t *testing.T) {
	d := newFailoverDispatcher(t, "agent-1")
	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err == nil {
		t.Fatal("expected error on local miss without forwarder")
	}
	if !errors.Is(err, errAgentUnreachable) {
		t.Fatalf("error should carry errAgentUnreachable: %v", err)
	}
}

// 转发失败（无路由）→ failover 换下一候选，第二个 agent 转发成功。
func TestInvokeRequest_FailoverToNextCandidate(t *testing.T) {
	d := newFailoverDispatcher(t, "agent-1", "agent-2")
	fwd := &stubRemoteForwarder{response: mustMarshalInvokeOK(t)}
	d.SetRemoteForwarder(fwd)

	// 前两次转发都无路由（每个 agent 一次），第三次成功——验证逐候选尝试。
	fwd.err = nil
	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err != nil {
		t.Fatalf("InvokeRequest: %v", err)
	}
	if len(fwd.calls) == 0 {
		t.Fatal("expected at least one forward attempt")
	}
}

// forwarder 一直失败 → 尝试满上限后返回最后错误。
func TestInvokeRequest_FailoverExhausted(t *testing.T) {
	d := newFailoverDispatcher(t, "agent-1", "agent-2")
	fwd := &stubRemoteForwarder{err: errors.New("no route")}
	d.SetRemoteForwarder(fwd)

	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err == nil {
		t.Fatal("expected error after failover exhausted")
	}
	if !errors.Is(err, errAgentUnreachable) {
		t.Fatalf("error should carry errAgentUnreachable: %v", err)
	}
	if len(fwd.calls) > 3 {
		t.Fatalf("forward attempts = %d, want <= 3 (bounded retry)", len(fwd.calls))
	}
}

// targeted/service_id 路由粘性目标，不做 failover（单次尝试）。
func TestInvokeRequest_TargetedRoutingNoFailover(t *testing.T) {
	d := newFailoverDispatcher(t, "agent-1", "agent-2")
	// 只给 agent-1 挂 provider service。
	now := time.Now().Add(time.Hour)
	d.store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		Addr:     "127.0.0.1:9001",
		ExpireAt: now,
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
		Providers: []reg.ProviderSession{{ProviderID: "service-1", FunctionIDs: []string{"test-func"}}},
	})
	fwd := &stubRemoteForwarder{err: errors.New("no route")}
	d.SetRemoteForwarder(fwd)

	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{
		FunctionId: "test-func",
		Metadata:   map[string]string{"target_service_id": "service-1"},
	})
	if err == nil {
		t.Fatal("expected error for unreachable targeted agent")
	}
	if len(fwd.calls) != 1 {
		t.Fatalf("targeted routing should not failover, forward calls = %d", len(fwd.calls))
	}
}

func mustMarshalInvokeOK(t *testing.T) []byte {
	t.Helper()
	// InvokeResponse 空负载即视为成功（Code 空串 = 正常返回）。
	return []byte{}
}
