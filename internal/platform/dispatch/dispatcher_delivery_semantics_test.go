package dispatch

import (
	"context"
	"testing"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// 投递语义回归（CR-FIX-10）：busy 快拒是「原样回给调用方的终态响应」，
// 不携带 errAgentUnreachable——dispatch 层禁止因 busy 换实例自动重试。
// 非幂等命令（Ops/写操作）在实例 A 上被拒绝后换实例 B 重试 = 隐式重复
// 执行入口；两条用例把「busy 不触发 failover」钉死为回归契约。
//
// busy 帧形态：Invoke → InvokeResponse{Payload: busy 文案}；StartTask →
// 空 body 响应帧（unmarshal 后即空 StartTaskResponse）。两类响应都是
// 成功完成的协议往返，绝不能被归类为「agent 不可达」。

const busyInvokePayload = `{"error":"inbound queue full, retry on another instance"}`

// 实例 busy：响应原样透传给调用方，且绝不换下一候选（agent-2 不得被
// 触达）。若未来把 busy 归类为可 failover 错误，本用例会因 fwd.calls
// 变成两个候选而红。
func TestInvokeRequest_BusyResponseDoesNotFailover(t *testing.T) {
	d := newFailoverDispatcher(t, "agent-1", "agent-2")
	fwd := &stubRemoteForwarder{
		response: mustMarshal(t, &sdkv1.InvokeResponse{Payload: []byte(busyInvokePayload)}),
	}
	d.SetRemoteForwarder(fwd)

	resp, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err != nil {
		t.Fatalf("busy response must not surface as dispatch error: %v", err)
	}
	if string(resp.GetPayload()) != busyInvokePayload {
		t.Fatalf("busy payload must pass through verbatim, got %q", string(resp.GetPayload()))
	}
	if len(fwd.calls) != 1 || fwd.calls[0] != "agent-1" {
		t.Fatalf("forward calls = %v, want exactly [agent-1] (no cross-instance retry on busy)", fwd.calls)
	}
}

// StartTask 的 busy 形态（空 body 响应帧）同样原样返回、不换实例：
// 任务派发是典型的非幂等 Ops 命令，busy 换实例重试可能在第二个实例上
// 重复执行副作用。
func TestStartTaskRequest_BusyResponseDoesNotFailover(t *testing.T) {
	d := newFailoverDispatcher(t, "agent-1", "agent-2")
	fwd := &stubRemoteForwarder{response: mustMarshal(t, &sdkv1.StartTaskResponse{})}
	d.SetRemoteForwarder(fwd)

	resp, err := d.StartTaskRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err != nil {
		t.Fatalf("busy (empty-body) response must not surface as dispatch error: %v", err)
	}
	if resp.GetTaskId() != "" {
		t.Fatalf("empty-body busy response must pass through verbatim, got task id %q", resp.GetTaskId())
	}
	if len(fwd.calls) != 1 || fwd.calls[0] != "agent-1" {
		t.Fatalf("forward calls = %v, want exactly [agent-1] (no cross-instance retry on busy)", fwd.calls)
	}
}
