package dispatch

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	apperrors "github.com/cuihairu/croupier/internal/errors"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// ---- 候选集扩展（RemoteAgentSource）基件 ----

type stubRemoteAgentSource struct {
	calls    int
	sessions []*reg.AgentSession
	err      error
}

func (s *stubRemoteAgentSource) RemoteAgentSessions(_ context.Context, _, _ string, _ bool) ([]*reg.AgentSession, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.sessions, nil
}

func remoteSession(agentID, gameID, env string, functions ...string) *reg.AgentSession {
	fns := map[string]reg.FunctionMeta{}
	for _, f := range functions {
		fns[f] = reg.FunctionMeta{Enabled: true}
	}
	return &reg.AgentSession{
		AgentID:   agentID,
		GameID:    gameID,
		Env:       env,
		ExpireAt:  time.Now().Add(time.Hour),
		Functions: fns,
	}
}

// ---- 用例 ----

// 本地候选为空 → 远端目录兜底：选中的远端 agent 经转发路径执行。
func TestPickAgent_LocalEmpty_FallsBackToRemoteSource(t *testing.T) {
	d := NewDispatcher(nil) // 本地 registry 无 agent
	src := &stubRemoteAgentSource{sessions: []*reg.AgentSession{
		remoteSession("agent-remote", "", "", "test-func"),
	}}
	d.SetRemoteAgentSource(src)
	fwd := &stubRemoteForwarder{response: mustMarshalInvokeOK(t)}
	d.SetRemoteForwarder(fwd)

	resp, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err != nil {
		t.Fatalf("InvokeRequest: %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if src.calls == 0 {
		t.Fatal("remote source should be consulted when local candidates are empty")
	}
	if len(fwd.calls) != 1 || fwd.calls[0] != "agent-remote" {
		t.Fatalf("forward calls = %v, want [agent-remote]", fwd.calls)
	}
}

// 本地有候选时零开销承诺：远端目录绝不被查询。
func TestPickAgent_LocalCandidatesPresent_NeverQueriesRemote(t *testing.T) {
	d := newFailoverDispatcher(t, "agent-1")
	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-1": &fakeSessionCaller{respBody: mustMarshalInvokeOK(t)},
	}})
	src := &stubRemoteAgentSource{}
	d.SetRemoteAgentSource(src)

	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err != nil {
		t.Fatalf("InvokeRequest: %v", err)
	}
	if src.calls != 0 {
		t.Fatalf("remote source queried %d times with local candidates present", src.calls)
	}
}

// flakyForwarder 前 failN 次返回错误，之后成功——构造本地候选逐个失败
// 的 failover 场景。
type flakyForwarder struct {
	calls  []string
	failN  int
	failed int
}

func (f *flakyForwarder) Forward(_ context.Context, call *RemoteCall) ([]byte, error) {
	f.calls = append(f.calls, call.AgentID)
	if f.failed < f.failN {
		f.failed++
		return nil, errors.New("no route")
	}
	return []byte{}, nil
}

// failover 耗尽本地候选后（exclude 过滤为空），下一轮 pick 触发远端查询，
// 远端候选经转发成功。
func TestPickAgent_AllLocalTried_FallsBackToRemote(t *testing.T) {
	d := newFailoverDispatcher(t, "agent-1", "agent-2")
	src := &stubRemoteAgentSource{sessions: []*reg.AgentSession{
		remoteSession("agent-remote", "", "", "test-func"),
	}}
	d.SetRemoteAgentSource(src)
	// 前两次（本地 agent-1、agent-2）失败，第三次（远端）成功。
	fwd := &flakyForwarder{failN: 2}
	d.SetRemoteForwarder(fwd)

	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err != nil {
		t.Fatalf("InvokeRequest: %v", err)
	}
	if src.calls == 0 {
		t.Fatal("remote source should be consulted after local candidates are exhausted")
	}
	if len(fwd.calls) != 3 || fwd.calls[2] != "agent-remote" {
		t.Fatalf("forward calls = %v, want [agent-1 agent-2 agent-remote]", fwd.calls)
	}
}

// 远端目录出错 → 降级回 noLiveAgentError（503），不放大故障。
func TestPickAgent_RemoteSourceError_ReturnsNoLiveAgent(t *testing.T) {
	d := NewDispatcher(nil)
	src := &stubRemoteAgentSource{err: errors.New("owner table down")}
	d.SetRemoteAgentSource(src)

	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err == nil {
		t.Fatal("expected error when both local and remote candidates unavailable")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T: %v", err, err)
	}
	if appErr.HTTPStatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", appErr.HTTPStatusCode)
	}
}

// 远端快照与本地候选同款过滤：scope 不匹配 / 函数未注册的远端 agent
// 不进入候选。
func TestPickAgent_RemoteScopedAndFunctionFilter(t *testing.T) {
	d := NewDispatcher(nil)
	src := &stubRemoteAgentSource{sessions: []*reg.AgentSession{
		remoteSession("agent-wrong-game", "other-game", "prod", "test-func"),
		remoteSession("agent-no-fn", "", "", "other-func"),
		remoteSession("agent-good", "", "", "test-func"),
	}}
	d.SetRemoteAgentSource(src)
	fwd := &stubRemoteForwarder{response: mustMarshalInvokeOK(t)}
	d.SetRemoteForwarder(fwd)

	meta := map[string]string{"gameId": "", "env": ""} // 非受限 scope
	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{
		FunctionId: "test-func",
		Metadata:   meta,
	})
	if err != nil {
		t.Fatalf("InvokeRequest: %v", err)
	}
	if len(fwd.calls) != 1 || fwd.calls[0] != "agent-good" {
		t.Fatalf("forward calls = %v, want only [agent-good]", fwd.calls)
	}
}
