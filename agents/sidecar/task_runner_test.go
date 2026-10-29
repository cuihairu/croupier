package sidecar

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// recordingReporter 捕获上报事件（随迁用例的最小替身；internal 侧另有完整
// mockTaskEventReporter 服务 LocalHandler 用例）。
type recordingReporter struct {
	mu     sync.Mutex
	events []*sdkv1.TaskEvent
}

func (m *recordingReporter) ReportTaskEvent(_ context.Context, event *sdkv1.TaskEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *recordingReporter) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func TestMarshalTaskInvokeV9(t *testing.T) {
	assert.Nil(t, marshalTaskInvoke(nil))

	req := &sdkv1.InvokeRequest{FunctionId: "fn.v9", Payload: []byte(`{}`)}
	data := marshalTaskInvoke(req)
	require.NotNil(t, data)

	decoded := &sdkv1.InvokeRequest{}
	require.NoError(t, proto.Unmarshal(data, decoded))
	assert.Equal(t, "fn.v9", decoded.GetFunctionId())
}

// 覆盖率补洞（group L）：task_runner.go marshalTaskInvoke 的 Marshal 失败分支
// （proto3 string 含无效 UTF-8 必败）。
func TestMarshalTaskInvoke_NilAndInvalidUTF8(t *testing.T) {
	assert.Nil(t, marshalTaskInvoke(nil))
	assert.Nil(t, marshalTaskInvoke(&sdkv1.InvokeRequest{
		FunctionId: "demo.ping",
		Metadata:   map[string]string{"k": "\xff\xfe"},
	}))

	data := marshalTaskInvoke(&sdkv1.InvokeRequest{FunctionId: "demo.ok"})
	require.NotNil(t, data)
	var back sdkv1.InvokeRequest
	require.NoError(t, proto.Unmarshal(data, &back))
	assert.Equal(t, "demo.ok", back.GetFunctionId())
}

func TestTaskRunnerReportEmptyPayloadV9(t *testing.T) {
	r := NewTaskRunner(func(context.Context, *sdkv1.InvokeRequest) ([]byte, error) {
		return nil, nil
	}, nil, nil)
	require.NoError(t, r.report(context.Background(), &sdkv1.TaskEvent{}))

	reporter := &recordingReporter{}
	r.SetReporter(reporter)
	event := &sdkv1.TaskEvent{TaskId: "t-v9"}
	require.NoError(t, r.report(context.Background(), event))
	assert.Equal(t, []byte("null"), event.Payload)
	assert.Equal(t, 1, reporter.count())
}

// ---- ARCH-015: duplicate Start constraint and terminal event replay ----

// flakyReporter fails the first failBefore ReportTaskEvent calls, then records
// successes. It lets tests prove terminal-event retry without a real transport.
type flakyReporter struct {
	mu         sync.Mutex
	failBefore int
	calls      int
	events     []*sdkv1.TaskEvent
}

func (f *flakyReporter) ReportTaskEvent(_ context.Context, event *sdkv1.TaskEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls <= f.failBefore {
		return errors.New("transport down")
	}
	f.events = append(f.events, event)
	return nil
}

func (f *flakyReporter) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

func (f *flakyReporter) totalCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// TestTaskRunner_DuplicateStartIdempotentNoop（ARCH-014 D1）：在途同 ID 的
// 第二次 Start 是幂等 no-op——不启动第二次执行、不产生第二条事件流。
func TestTaskRunner_DuplicateStartIdempotentNoop(t *testing.T) {
	var calls int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	executor := func(_ context.Context, _ *sdkv1.InvokeRequest) ([]byte, error) {
		atomic.AddInt32(&calls, 1)
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return []byte("ok"), nil
	}
	reporter := &recordingReporter{}
	r := NewTaskRunner(executor, reporter, nil)
	r.SetEventRetryPolicy(1, time.Millisecond)

	req := &sdkv1.InvokeRequest{Metadata: map[string]string{"taskId": "dup-noop"}}
	id1 := r.Start(req)
	<-started // first execution in-flight

	// Duplicate Start for the same in-flight ID: no second execution.
	id2 := r.Start(req)
	assert.Equal(t, id1, id2)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "duplicate Start must not launch a second execution")
	assert.Equal(t, 1, r.Count(), "duplicate Start must not add a tracking entry")

	close(release)
	require.Eventually(t, func() bool { return r.Count() == 0 }, 2*time.Second, 10*time.Millisecond)

	// Exactly one started + one completed; no second stream.
	assert.Equal(t, 2, reporter.count())
	assert.Equal(t, "started", reporter.events[0].GetType())
	assert.Equal(t, "completed", reporter.events[1].GetType())
	assert.Equal(t, "dup-noop", reporter.events[0].GetTaskId())
}

// TestTaskRunner_CancelKeepsEntryUntilExecutorReturns（ARCH-014 D1）：Cancel
// 不急切删除跟踪项；项存活到执行器真正返回，收尾期同 ID Start 仍视为在途
// no-op。
func TestTaskRunner_CancelKeepsEntryUntilExecutorReturns(t *testing.T) {
	var calls int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	executor := func(ctx context.Context, _ *sdkv1.InvokeRequest) ([]byte, error) {
		atomic.AddInt32(&calls, 1)
		started <- struct{}{}
		<-release
		return nil, ctx.Err()
	}
	r := NewTaskRunner(executor, nil, nil)
	r.SetEventRetryPolicy(1, time.Millisecond)

	req := &sdkv1.InvokeRequest{Metadata: map[string]string{"taskId": "cancel-keep"}}
	id := r.Start(req)
	<-started

	assert.True(t, r.Cancel(id))
	assert.Equal(t, 1, r.Count(), "Cancel must not eagerly delete the tracking entry")

	// Same-ID Start during cancel wind-down: still in-flight → no-op.
	id2 := r.Start(req)
	assert.Equal(t, id, id2)
	assert.Equal(t, 1, r.Count())
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))

	close(release)
	require.Eventually(t, func() bool { return r.Count() == 0 }, 2*time.Second, 10*time.Millisecond)
}

// TestTaskRunner_OldCleanupDoesNotDeleteNewSameIDTask（ARCH-015 验收项）：旧
// 协程收尾的清理只删除自身代际的项——若同 ID 已有新代际项，旧清理不得删除
// 它。直接注入同 ID 新代际项，验证旧 run 的 deferred cleanup 跳过。
func TestTaskRunner_OldCleanupDoesNotDeleteNewSameIDTask(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	executor := func(_ context.Context, _ *sdkv1.InvokeRequest) ([]byte, error) {
		started <- struct{}{}
		<-release
		return nil, nil
	}
	r := NewTaskRunner(executor, nil, nil)
	r.SetEventRetryPolicy(1, time.Millisecond)

	id := "same-id"
	r.Start(&sdkv1.InvokeRequest{Metadata: map[string]string{"taskId": id}})
	<-started // gen-1 task in-flight

	// 同 ID 出现新代际项（正常情况下 Start 在途 no-op 使此不成立，但代际守卫
	// 必须防御住：旧清理不得删它）。
	r.mu.Lock()
	r.nextGen++
	gen2 := r.nextGen
	r.tasks[id] = &taskEntry{generation: gen2, cancel: func() {}}
	r.mu.Unlock()

	close(release) // gen-1 任务结束；其 cleanup 必须保留 gen-2 项
	require.Eventually(t, func() bool {
		r.mu.RLock()
		defer r.mu.RUnlock()
		entry, ok := r.tasks[id]
		return ok && entry.generation == gen2
	}, 2*time.Second, 10*time.Millisecond)
}

// TestTaskRunner_TerminalEventRetriedUntilAck（ARCH-014 D2）：终态事件在传输
// 故障后重试至 Server ACK（nil 响应）。started 为尽力而为单发；completed 带
// 界重试。
func TestTaskRunner_TerminalEventRetriedUntilAck(t *testing.T) {
	var calls int32
	executor := func(_ context.Context, _ *sdkv1.InvokeRequest) ([]byte, error) {
		atomic.AddInt32(&calls, 1)
		return []byte("ok"), nil
	}
	reporter := &flakyReporter{failBefore: 2} // started 失败（忽略）+ completed 第 1 次失败
	r := NewTaskRunner(executor, reporter, nil)
	r.SetEventRetryPolicy(3, time.Millisecond)

	r.Start(&sdkv1.InvokeRequest{Metadata: map[string]string{"taskId": "retry-ack"}})
	require.Eventually(t, func() bool { return r.Count() == 0 }, 2*time.Second, 10*time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
	// calls: 1=started(fail), 2=completed(fail), 3=completed(ok).
	assert.GreaterOrEqual(t, reporter.totalCalls(), 3)
	assert.Equal(t, 1, reporter.count())
	assert.Equal(t, "completed", reporter.events[0].GetType())
	assert.Equal(t, "retry-ack", reporter.events[0].GetTaskId())
}

// TestTaskRunner_TerminalEventGivesUpAfterBound（ARCH-014 D2）：重试达到上界
// 后丢弃终态事件（交 Server 收割器 D3），不无限阻塞任务收尾。
func TestTaskRunner_TerminalEventGivesUpAfterBound(t *testing.T) {
	executor := func(_ context.Context, _ *sdkv1.InvokeRequest) ([]byte, error) {
		return []byte("ok"), nil
	}
	reporter := &flakyReporter{failBefore: 1 << 30} // 恒失败
	r := NewTaskRunner(executor, reporter, nil)
	r.SetEventRetryPolicy(2, time.Millisecond) // 2 次尝试后放弃

	r.Start(&sdkv1.InvokeRequest{Metadata: map[string]string{"taskId": "retry-giveup"}})
	require.Eventually(t, func() bool { return r.Count() == 0 }, 2*time.Second, 10*time.Millisecond)

	// started(1) + completed 尝试(2) = 3 次调用；无成功上报。
	assert.Equal(t, 3, reporter.totalCalls())
	assert.Equal(t, 0, reporter.count())
}
