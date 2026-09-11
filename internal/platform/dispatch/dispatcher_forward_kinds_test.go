package dispatch

import (
	"context"
	"errors"
	"strings"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// ---- C3 转发泛化（async/cancel/broadcast 三分支）基件 ----

// stubTaskRunWriter 记录 caller 侧 task_runs 写入（断言 caller/owner 职责切分）。
type stubTaskRunWriter struct{ runs []string }

func (w *stubTaskRunWriter) CreateRun(ctx context.Context, taskID, functionID, agentID, gameID, env, status string, inputPayload []byte) error {
	w.runs = append(w.runs, taskID)
	return nil
}

func (w *stubTaskRunWriter) CreateRunWithMeta(ctx context.Context, taskID, functionID, agentID, gameID, env, status, actor, addr, traceID string, inputPayload []byte) error {
	w.runs = append(w.runs, taskID)
	return nil
}

// stubTaskAgentLookup 固定返回 agentID，记录查询次数。
type stubTaskAgentLookup struct {
	agentID string
	calls   int
}

func (l *stubTaskAgentLookup) AgentForTask(ctx context.Context, taskID string) (string, error) {
	l.calls++
	return l.agentID, nil
}

// ---- async（StartTask）----

// 本地无该 agent session（无 resolver）→ 经转发完成；caller 职责保持：
// task_runs 行写入、服务端 task ID 随帧（metadata.taskId）、成功后
// registerTask 落在 caller 实例。
func TestStartTask_RemoteForwardOnLocalMiss(t *testing.T) {
	d := newFailoverDispatcher(t, "agent-1") // registry 有候选，本地连接 miss
	writer := &stubTaskRunWriter{}
	d.SetTaskRunWriter(writer)
	fwd := &stubRemoteForwarder{response: mustMarshal(t, &sdkv1.StartTaskResponse{TaskId: "resp-task"})}
	d.SetRemoteForwarder(fwd)

	resp, err := d.StartTaskRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err != nil {
		t.Fatalf("StartTaskRequest: %v", err)
	}
	if resp.GetTaskId() != "resp-task" {
		t.Fatalf("task id = %q, want resp-task", resp.GetTaskId())
	}
	if fwd.last == nil || fwd.last.MsgID != protocol.MsgStartTaskRequest {
		t.Fatalf("forward call = %+v, want MsgStartTaskRequest", fwd.last)
	}
	// caller 生成的服务端 task ID 必须随帧（owner 重建 InvokeRequest 沿用，
	// 否则 agent 事件与 task_runs 行失配）。
	if fwd.last.Metadata["taskId"] == "" {
		t.Fatalf("forward metadata missing taskId: %+v", fwd.last.Metadata)
	}
	if len(writer.runs) != 1 || writer.runs[0] == "" {
		t.Fatalf("task run rows = %v, want exactly one non-empty caller-generated id", writer.runs)
	}
	// registerTask 是 caller 职责：转发成功后本地路由可查。
	if got, ok := d.TaskAgentID("resp-task"); !ok || got != "agent-1" {
		t.Fatalf("TaskAgentID(resp-task) = %q, %v; want agent-1", got, ok)
	}
}

// owner 侧定向投递：不写 task_runs、不 registerTask（均为 caller 职责），
// 透传 metadata.taskId 给 agent。
func TestStartTaskOnAgent_NoRunRowNoRouting(t *testing.T) {
	d := NewDispatcher(nil)
	caller := &fakeSessionCaller{respBody: mustMarshal(t, &sdkv1.StartTaskResponse{TaskId: "srv-task"})}
	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{"agent-1": caller}})
	writer := &stubTaskRunWriter{}
	d.SetTaskRunWriter(writer)

	respBytes, err := d.StartTaskOnAgent(context.Background(), "agent-1", &sdkv1.InvokeRequest{
		FunctionId: "test-func",
		Metadata:   map[string]string{"taskId": "srv-task"},
	})
	if err != nil {
		t.Fatalf("StartTaskOnAgent: %v", err)
	}
	resp := &sdkv1.StartTaskResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.GetTaskId() != "srv-task" {
		t.Fatalf("task id = %q, want srv-task", resp.GetTaskId())
	}
	if len(writer.runs) != 0 {
		t.Fatalf("owner path must not write task_runs, got %v", writer.runs)
	}
	if routings, _ := d.ListTaskRoutings(); len(routings) != 0 {
		t.Fatalf("owner path must not register task routing, got %v", routings)
	}
}

// ---- cancel（CancelTask）----

// 内存 map 与本地 taskStore 都 miss（跨实例取消落到非发起实例）→
// TaskAgentLookup 从共享 task_runs 解析 agent，再经转发投递取消。
func TestCancelTask_TaskAgentLookupFallback(t *testing.T) {
	d := NewDispatcher(nil) // taskStore 为空 memory store、routing 空
	lookup := &stubTaskAgentLookup{agentID: "agent-remote"}
	d.SetTaskAgentLookup(lookup)
	fwd := &stubRemoteForwarder{response: []byte{}}
	d.SetRemoteForwarder(fwd)

	if err := d.CancelTask(context.Background(), "task-x"); err != nil {
		t.Fatalf("CancelTask: %v", err)
	}
	if lookup.calls != 1 {
		t.Fatalf("lookup calls = %d, want 1", lookup.calls)
	}
	if fwd.last == nil || fwd.last.AgentID != "agent-remote" {
		t.Fatalf("forward call = %+v, want agent-remote", fwd.last)
	}
	if fwd.last.MsgID != protocol.MsgCancelTaskRequest || fwd.last.TaskID != "task-x" {
		t.Fatalf("forward call = %+v, want cancel_task + task-x", fwd.last)
	}
}

// lookup 未装配（单实例）且本地 miss → 维持 "task not tracked" 语义。
func TestCancelTask_NoLookupStillNotTracked(t *testing.T) {
	d := NewDispatcher(nil)
	err := d.CancelTask(context.Background(), "task-ghost")
	if err == nil || !strings.Contains(err.Error(), "not tracked") {
		t.Fatalf("error = %v, want task not tracked", err)
	}
}

// ---- broadcast（local ∪ remote）----

func broadcastDispatcher(t *testing.T, fwdErr error) (*Dispatcher, *stubRemoteForwarder) {
	t.Helper()
	d := newFailoverDispatcher(t, "agent-local")
	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-local": &fakeSessionCaller{respBody: []byte{}},
	}})
	d.SetRemoteAgentSource(&stubRemoteAgentSource{sessions: []*reg.AgentSession{
		remoteSession("agent-remote", "", "", "test-func"),
	}})
	fwd := &stubRemoteForwarder{response: []byte{}, err: fwdErr}
	d.SetRemoteForwarder(fwd)
	return d, fwd
}

// broadcast 语义是投给所有活跃 agent：远端持有的一半必须并入候选集
// （本地有候选也查远端，与 pick 的「本地空才查」不同）。
func TestInvokeBroadcast_LocalUnionRemote(t *testing.T) {
	d, fwd := broadcastDispatcher(t, nil)

	result, err := d.InvokeBroadcast(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err != nil {
		t.Fatalf("InvokeBroadcast: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("total = %d, want 2 (local + remote)", result.Total)
	}
	if len(result.Successes) != 2 {
		t.Fatalf("successes = %d, want 2", len(result.Successes))
	}
	if len(fwd.calls) != 1 || fwd.calls[0] != "agent-remote" {
		t.Fatalf("forward calls = %v, want [agent-remote]", fwd.calls)
	}
}

// 远端目录把本地 agent 也回灌（agent 迁移窗口的竞态）→ 按 AgentID 去重、
// 本地优先，不重复投递。
func TestInvokeBroadcast_RemoteEchoDedup(t *testing.T) {
	d := newFailoverDispatcher(t, "agent-local")
	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-local": &fakeSessionCaller{respBody: []byte{}},
	}})
	d.SetRemoteAgentSource(&stubRemoteAgentSource{sessions: []*reg.AgentSession{
		remoteSession("agent-local", "", "", "test-func"), // 与本地同 ID
		remoteSession("agent-remote", "", "", "test-func"),
	}})
	fwd := &stubRemoteForwarder{response: []byte{}}
	d.SetRemoteForwarder(fwd)

	result, err := d.InvokeBroadcast(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err != nil {
		t.Fatalf("InvokeBroadcast: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("total = %d, want 2 after dedup", result.Total)
	}
	if len(fwd.calls) != 1 || fwd.calls[0] != "agent-remote" {
		t.Fatalf("forward calls = %v, want [agent-remote] only", fwd.calls)
	}
}

// 远端转发失败与其他 agent 失败同语义：落 Failures[]，整单不报错
// （HTTP 200 半失败语义保持）。
func TestInvokeBroadcast_RemoteFailureLandsInFailures(t *testing.T) {
	d, _ := broadcastDispatcher(t, errors.New("owner down"))

	result, err := d.InvokeBroadcast(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	if err != nil {
		t.Fatalf("InvokeBroadcast must not fail wholesale on remote error: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("total = %d, want 2", result.Total)
	}
	if len(result.Successes) != 1 || result.Successes[0].AgentID != "agent-local" {
		t.Fatalf("successes = %+v, want agent-local", result.Successes)
	}
	if len(result.Failures) != 1 || result.Failures[0].AgentID != "agent-remote" {
		t.Fatalf("failures = %+v, want agent-remote", result.Failures)
	}
}
