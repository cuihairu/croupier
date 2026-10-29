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

// flakyReporter fails the first `failures` delivery attempts with err, then
// succeeds. Used to exercise the ARCH-014 D2 bounded retry.
type flakyReporter struct {
	mu       sync.Mutex
	failures int
	calls    int
	err      error
}

func (m *flakyReporter) ReportTaskEvent(_ context.Context, _ *sdkv1.TaskEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.calls <= m.failures {
		return m.err
	}
	return nil
}

func (m *flakyReporter) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func noopTaskExecutor(context.Context, *sdkv1.InvokeRequest) ([]byte, error) {
	return nil, nil
}

func invokeReqWithTaskID(id string) *sdkv1.InvokeRequest {
	return &sdkv1.InvokeRequest{Metadata: map[string]string{"taskId": id}}
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

// ARCH-014 D1: a duplicate Start for a taskId already in flight is an
// idempotent no-op — no second goroutine, no overwritten cancel registration.
func TestTaskRunnerDuplicateStartIsIdempotentV9(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	executor := func(_ context.Context, _ *sdkv1.InvokeRequest) ([]byte, error) {
		calls.Add(1)
		close(started)
		<-release
		return nil, nil
	}
	reporter := &recordingReporter{}
	r := NewTaskRunner(executor, reporter, nil)
	r.SetEventRetryPolicy(1, 0)

	req := invokeReqWithTaskID("t-dup")
	firstID := r.Start(req)
	assert.Equal(t, "t-dup", firstID)
	<-started

	// Second Start for the same running taskId: idempotent no-op.
	secondID := r.Start(req)
	assert.Equal(t, "t-dup", secondID)
	assert.Equal(t, 1, r.Count())
	assert.Equal(t, int32(1), calls.Load(), "duplicate Start must not launch a second goroutine")

	close(release)
	// Let the single goroutine finish and clean up.
	require.Eventually(t, func() bool { return r.Count() == 0 }, time.Second, 5*time.Millisecond)
	// The single goroutine reports exactly started + completed = 2 events; a
	// duplicate Start would have added another started/terminal pair.
	assert.Equal(t, 2, reporter.count(), "one started + one completed from the single goroutine")
}

// ARCH-014 D1 ownership guard: after Cancel and a same-ID re-Start, the old
// goroutine's cleanup must not delete the new task's slot.
func TestTaskRunnerOldCleanupPreservesReplacementV9(t *testing.T) {
	var calls atomic.Int32
	firstStarted := make(chan struct{})
	firstGate := make(chan struct{})
	executor := func(ctx context.Context, _ *sdkv1.InvokeRequest) ([]byte, error) {
		n := calls.Add(1)
		if n == 1 {
			close(firstStarted)
			<-ctx.Done() // Cancel already fired
			<-firstGate  // held until the test has re-Started the same taskId
			return nil, ctx.Err()
		}
		// The replacement task: stays in flight until the test ends.
		<-ctx.Done()
		return nil, ctx.Err()
	}
	r := NewTaskRunner(executor, &recordingReporter{}, nil)
	r.SetEventRetryPolicy(1, 0)

	req := invokeReqWithTaskID("t-gen")
	r.Start(req)
	<-firstStarted

	require.True(t, r.Cancel("t-gen"), "cancel of the first task")
	// Re-Start the same taskId: a new generation claims the slot.
	r.Start(req)
	assert.Equal(t, 1, r.Count(), "replacement task registered")

	// Release the old goroutine so its defer runs against the replacement slot.
	close(firstGate)
	time.Sleep(200 * time.Millisecond) // let the old goroutine's cleanup run

	// The replacement slot must survive the old goroutine's cleanup, and the
	// replacement task must still be cancellable (its slot was not leaked).
	assert.Equal(t, 1, r.Count(), "old cleanup must not delete the replacement slot")
	require.True(t, r.Cancel("t-gen"), "replacement task must remain cancellable")
}

// ARCH-014 D2: event delivery retries with bounded backoff and succeeds once
// the reporter recovers.
func TestTaskRunnerEventRetryThenSuccessV9(t *testing.T) {
	reporter := &flakyReporter{failures: 2, err: errors.New("transport down")}
	r := NewTaskRunner(noopTaskExecutor, reporter, nil)
	r.SetEventRetryPolicy(3, time.Millisecond)

	err := r.reportWithRetry(context.Background(), &sdkv1.TaskEvent{TaskId: "t", Type: "started"})
	require.NoError(t, err)
	assert.Equal(t, 3, reporter.callCount(), "2 failures then success = 3 attempts")
}

// ARCH-014 D2: when the retry window is exhausted the failure is surfaced, not
// silently dropped.
func TestTaskRunnerEventRetryExhaustedSurfacedV9(t *testing.T) {
	reporter := &flakyReporter{failures: 1 << 30, err: errors.New("transport down")}
	r := NewTaskRunner(noopTaskExecutor, reporter, nil)
	r.SetEventRetryPolicy(3, time.Millisecond)

	err := r.reportWithRetry(context.Background(), &sdkv1.TaskEvent{TaskId: "t", Type: "completed"})
	require.Error(t, err)
	assert.EqualError(t, err, "transport down")
	assert.Equal(t, 3, reporter.callCount(), "exactly maxAttempts attempts")
}

// ARCH-014 D2 end-to-end via run: a completed task whose terminal event cannot
// be delivered must not hang and must terminate (the row is reconciled by the
// server's D3 unknown path, not silently left running here).
func TestTaskRunnerRunSurfacesTerminalDeliveryFailureV9(t *testing.T) {
	reporter := &flakyReporter{failures: 1 << 30, err: errors.New("transport down")}
	executed := make(chan struct{})
	r := NewTaskRunner(func(context.Context, *sdkv1.InvokeRequest) ([]byte, error) {
		close(executed)
		return []byte(`{"ok":true}`), nil
	}, reporter, nil)
	r.SetEventRetryPolicy(2, time.Millisecond)

	r.Start(invokeReqWithTaskID("t-runfail"))
	<-executed
	// started (1 attempt) + completed (2 attempts) = 3 total delivery attempts.
	require.Eventually(t, func() bool { return reporter.callCount() == 3 }, time.Second, 5*time.Millisecond)
	// The goroutine must not hang on the exhausted retry: the slot is released.
	require.Eventually(t, func() bool { return r.Count() == 0 }, time.Second, 5*time.Millisecond)
}
