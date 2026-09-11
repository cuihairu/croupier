package main

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/cluster"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// fakeLocalDispatcher 记录三类定向投递的调用与入参（localInvoker 分发验证）。
type fakeLocalDispatcher struct {
	invokeAgentIDs []string
	startAgentIDs  []string
	cancelCalls    [][2]string // {agentID, taskID}

	lastInvokeReq *sdkv1.InvokeRequest
	lastStartReq  *sdkv1.InvokeRequest

	invokeErr error
	startErr  error
	cancelErr error
}

func (f *fakeLocalDispatcher) InvokeRequestOnAgent(_ context.Context, agentID string, req *sdkv1.InvokeRequest) ([]byte, error) {
	f.invokeAgentIDs = append(f.invokeAgentIDs, agentID)
	f.lastInvokeReq = req
	return []byte("invoke-resp"), f.invokeErr
}

func (f *fakeLocalDispatcher) StartTaskOnAgent(_ context.Context, agentID string, req *sdkv1.InvokeRequest) ([]byte, error) {
	f.startAgentIDs = append(f.startAgentIDs, agentID)
	f.lastStartReq = req
	return []byte("start-resp"), f.startErr
}

func (f *fakeLocalDispatcher) CancelTaskOnAgent(_ context.Context, agentID, taskID string) ([]byte, error) {
	f.cancelCalls = append(f.cancelCalls, [2]string{agentID, taskID})
	return nil, f.cancelErr
}

// Kind 为空 = invoke（旧 caller 兼容）：走 InvokeRequestOnAgent，caller
// 身份注入 metadata（forwarded_by/game/env）。
func TestLocalInvoker_EmptyKindRoutesToInvoke(t *testing.T) {
	fake := &fakeLocalDispatcher{}
	li := localInvoker{dispatcher: fake}

	res, err := li.InvokeLocal(context.Background(), &cluster.ForwardedInvoke{
		AgentID:    "agent-1",
		FunctionID: "demo.fn",
		Caller:     cluster.CallerContext{Username: "alice", GameID: "demo", Env: "prod"},
	})
	if err != nil {
		t.Fatalf("InvokeLocal: %v", err)
	}
	if !res.OK || string(res.Payload) != "invoke-resp" {
		t.Fatalf("result = %+v", res)
	}
	if len(fake.invokeAgentIDs) != 1 || fake.invokeAgentIDs[0] != "agent-1" {
		t.Fatalf("invoke calls = %v", fake.invokeAgentIDs)
	}
	if len(fake.startAgentIDs) != 0 || len(fake.cancelCalls) != 0 {
		t.Fatalf("start/cancel must not fire for invoke kind")
	}
	meta := fake.lastInvokeReq.GetMetadata()
	if meta["forwarded_by"] != "alice" || meta["gameId"] != "demo" || meta["env"] != "prod" {
		t.Fatalf("metadata = %v, want forwarded_by/gameId/env injected", meta)
	}
	if meta["agentId"] != "" {
		t.Fatalf("forward flag must not leak to agent: %v", meta)
	}
}

// kind=start_task：走 StartTaskOnAgent，metadata（含 caller 生成的
// taskId）随帧透传给 owner 侧重建的 InvokeRequest。
func TestLocalInvoker_StartTaskKindRoutesToStart(t *testing.T) {
	fake := &fakeLocalDispatcher{}
	li := localInvoker{dispatcher: fake}

	res, err := li.InvokeLocal(context.Background(), &cluster.ForwardedInvoke{
		Kind:       cluster.ForwardKindStartTask,
		AgentID:    "agent-1",
		FunctionID: "demo.fn",
		Metadata:   map[string]string{"taskId": "srv-task-1"},
		Caller:     cluster.CallerContext{Username: "bob"},
	})
	if err != nil {
		t.Fatalf("InvokeLocal: %v", err)
	}
	if !res.OK || string(res.Payload) != "start-resp" {
		t.Fatalf("result = %+v", res)
	}
	if len(fake.startAgentIDs) != 1 || fake.startAgentIDs[0] != "agent-1" {
		t.Fatalf("start calls = %v", fake.startAgentIDs)
	}
	if len(fake.invokeAgentIDs) != 0 || len(fake.cancelCalls) != 0 {
		t.Fatalf("invoke/cancel must not fire for start_task kind")
	}
	if fake.lastStartReq.GetMetadata()["taskId"] != "srv-task-1" {
		t.Fatalf("taskId not carried: %v", fake.lastStartReq.GetMetadata())
	}
	if fake.lastStartReq.GetMetadata()["forwarded_by"] != "bob" {
		t.Fatalf("forwarded_by not injected: %v", fake.lastStartReq.GetMetadata())
	}
}

// kind=cancel_task：走 CancelTaskOnAgent(agentID, taskID)；缺 TaskID 拒绝。
func TestLocalInvoker_CancelKindRoutesToCancel(t *testing.T) {
	fake := &fakeLocalDispatcher{}
	li := localInvoker{dispatcher: fake}

	res, err := li.InvokeLocal(context.Background(), &cluster.ForwardedInvoke{
		Kind:    cluster.ForwardKindCancel,
		AgentID: "agent-1",
		TaskID:  "task-9",
	})
	if err != nil {
		t.Fatalf("InvokeLocal: %v", err)
	}
	if !res.OK {
		t.Fatalf("result = %+v", res)
	}
	if len(fake.cancelCalls) != 1 || fake.cancelCalls[0] != [2]string{"agent-1", "task-9"} {
		t.Fatalf("cancel calls = %v", fake.cancelCalls)
	}
	if len(fake.invokeAgentIDs) != 0 || len(fake.startAgentIDs) != 0 {
		t.Fatalf("invoke/start must not fire for cancel kind")
	}

	// 缺 taskId：owner 拒绝（caller 侧帧构造缺陷，不应静默成功）。
	res, err = li.InvokeLocal(context.Background(), &cluster.ForwardedInvoke{
		Kind:    cluster.ForwardKindCancel,
		AgentID: "agent-1",
	})
	if err != nil {
		t.Fatalf("InvokeLocal: %v", err)
	}
	if res.OK || res.Error == "" {
		t.Fatalf("cancel without taskId must fail, got %+v", res)
	}
}

// 投递失败：ForwardedResult.OK=false + Error（不是 transport error——
// caller 侧把它翻译成调用错误）。
func TestLocalInvoker_DeliveryFailureMapsToNotOK(t *testing.T) {
	fake := &fakeLocalDispatcher{invokeErr: context.DeadlineExceeded}
	li := localInvoker{dispatcher: fake}

	res, err := li.InvokeLocal(context.Background(), &cluster.ForwardedInvoke{
		AgentID: "agent-1", FunctionID: "demo.fn",
	})
	if err != nil {
		t.Fatalf("InvokeLocal must return result, not error: %v", err)
	}
	if res.OK || res.Error == "" {
		t.Fatalf("result = %+v, want OK=false with error", res)
	}
}
