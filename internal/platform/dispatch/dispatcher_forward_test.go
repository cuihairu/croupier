package dispatch

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	apperrors "github.com/cuihairu/croupier/internal/errors"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// ---- 测试基件 ----

type stubRemoteForwarder struct {
	calls    []string
	last     *RemoteCall
	response []byte
	err      error
}

func (s *stubRemoteForwarder) Forward(_ context.Context, call *RemoteCall) ([]byte, error) {
	s.calls = append(s.calls, call.AgentID)
	s.last = call
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
		Metadata:   map[string]string{"targetServiceId": "service-1"},
	})
	if err == nil {
		t.Fatal("expected error for unreachable targeted agent")
	}
	if len(fwd.calls) != 1 {
		t.Fatalf("targeted routing should not failover, forward calls = %d", len(fwd.calls))
	}
	// pinned 生效的回归锚：错误是单次尝试的原始错误（含 forward 失败原因），
	// 不是「换候选后无 agent」的 failover exhausted 包装。键名 bug
	// （routePinned 读 target_service_id 旧键）会让本断言翻车。
	if !strings.Contains(err.Error(), "no route") || strings.Contains(err.Error(), "failover exhausted") {
		t.Fatalf("pinned routing should surface single-attempt error, got: %v", err)
	}
}

// hash 路由同样粘性：hashKey 选中的 agent 失败直接返回，不进入 failover。
func TestInvokeRequest_RoutePinnedByHashKey_SingleAttempt(t *testing.T) {
	d := newFailoverDispatcher(t, "agent-1", "agent-2")
	fwd := &stubRemoteForwarder{err: errors.New("no route")}
	d.SetRemoteForwarder(fwd)

	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{
		FunctionId: "test-func",
		Metadata:   map[string]string{"hashKey": "player-42"},
	})
	if err == nil {
		t.Fatal("expected error for unreachable hash-pinned agent")
	}
	if len(fwd.calls) != 1 {
		t.Fatalf("hash routing should not failover, forward calls = %d", len(fwd.calls))
	}
	if strings.Contains(err.Error(), "failover exhausted") {
		t.Fatalf("hash pinned should be single attempt, got failover: %v", err)
	}
}

// failover 耗尽映射 503 service_unavailable（与首轮 noLiveAgent 同语义），
// 而不是普通 error 落 500；cause 链保留 errAgentUnreachable。
func TestInvokeRequest_FailoverExhausted_ReturnsServiceUnavailable(t *testing.T) {
	d := newFailoverDispatcher(t, "agent-1", "agent-2")
	fwd := &stubRemoteForwarder{err: errors.New("no route")}
	d.SetRemoteForwarder(fwd)

	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err == nil {
		t.Fatal("expected error after failover exhausted")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("failover exhausted should be AppError, got %T: %v", err, err)
	}
	if appErr.HTTPStatusCode != http.StatusServiceUnavailable {
		t.Fatalf("failover exhausted should map to 503, got %d", appErr.HTTPStatusCode)
	}
	if !errors.Is(err, errAgentUnreachable) {
		t.Fatalf("error should carry errAgentUnreachable: %v", err)
	}
}

func mustMarshalInvokeOK(t *testing.T) []byte {
	t.Helper()
	// InvokeResponse 空负载即视为成功（Code 空串 = 正常返回）。
	return []byte{}
}
