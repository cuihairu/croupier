package dispatch

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// fakeSessionCaller implements transport.SessionCaller with canned responses.
type fakeSessionCaller struct {
	mu       sync.Mutex
	respBody []byte
	err      error
	calls    int
}

func (f *fakeSessionCaller) Call(ctx context.Context, msgID uint32, reqBody []byte) (uint32, []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return 0, nil, f.err
	}
	return msgID, f.respBody, nil
}

func (f *fakeSessionCaller) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// fakeSessionResolver implements AgentSessionResolver backed by a static map.
type fakeSessionResolver struct {
	callers map[string]transport.SessionCaller
}

func (f *fakeSessionResolver) ResolveAgentConn(agentID string) (transport.SessionCaller, bool) {
	caller, ok := f.callers[agentID]
	return caller, ok
}

func mustMarshal(t *testing.T, m proto.Message) []byte {
	t.Helper()
	data, err := proto.Marshal(m)
	require.NoError(t, err)
	return data
}

func registerTestAgent(t *testing.T, d *Dispatcher, agentID string) {
	t.Helper()
	require.NoError(t, d.Store().UpsertAgent(&reg.AgentSession{
		AgentID:  agentID,
		ExpireAt: time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	}))
}

func TestDispatcher_InvokeRequest_SuccessViaSession(t *testing.T) {
	d := NewDispatcher(nil)
	registerTestAgent(t, d, "agent-1")

	caller := &fakeSessionCaller{respBody: mustMarshal(t, &sdkv1.InvokeResponse{Payload: []byte("ok")})}
	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-1": caller,
	}})

	resp, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{
		FunctionId: "test-func",
		Payload:    []byte("ping"),
	})
	require.NoError(t, err)
	assert.Equal(t, []byte("ok"), resp.GetPayload())
	assert.Equal(t, 1, caller.callCount())
}

func TestDispatcher_InvokeRequest_UnmarshalErrorViaSession(t *testing.T) {
	d := NewDispatcher(nil)
	registerTestAgent(t, d, "agent-1")

	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-1": &fakeSessionCaller{respBody: []byte("not-a-proto-message")},
	}})

	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{
		FunctionId: "test-func",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal response")
}

func TestDispatcher_InvokeRequest_SessionNotFound(t *testing.T) {
	d := NewDispatcher(nil)
	registerTestAgent(t, d, "agent-1")
	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{}})

	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestDispatcher_InvokeRequest_NoSessionResolver(t *testing.T) {
	d := NewDispatcher(nil)
	registerTestAgent(t, d, "agent-1")

	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session resolver not configured")
}

func TestDispatcher_InvokeRequest_CallErrorViaSession(t *testing.T) {
	d := NewDispatcher(nil)
	registerTestAgent(t, d, "agent-1")

	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-1": &fakeSessionCaller{err: errors.New("connection reset")},
	}})

	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection reset")
}

// recordingRunWriter captures CreateRunWithMeta invocations.
type recordingRunWriter struct {
	mu        sync.Mutex
	created   []string
	errToFail error
}

func (w *recordingRunWriter) CreateRun(ctx context.Context, taskID, functionID, agentID, gameID, env, status string, inputPayload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.created = append(w.created, taskID)
	return nil
}

func (w *recordingRunWriter) CreateRunWithMeta(ctx context.Context, taskID, functionID, agentID, gameID, env, status, actor, addr, traceID string, inputPayload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.created = append(w.created, taskID)
	return w.errToFail
}

func (w *recordingRunWriter) createdIDs() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.created...)
}

func TestDispatcher_StartTaskRequest_SuccessViaSession(t *testing.T) {
	d := NewDispatcher(nil)
	require.NoError(t, d.Store().UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{
			"test-func": {Enabled: true},
		},
	}))

	writer := &recordingRunWriter{}
	d.SetTaskRunWriter(writer)

	caller := &fakeSessionCaller{respBody: mustMarshal(t, &sdkv1.StartTaskResponse{TaskId: "agent-task-1"})}
	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-1": caller,
	}})

	resp, err := d.StartTaskRequest(context.Background(), &sdkv1.InvokeRequest{
		FunctionId: "test-func",
		Payload:    []byte("payload"),
		Metadata: map[string]string{
			"game_id": "demo-game",
			"env":     "development",
			"actor":   "admin",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "agent-task-1", resp.GetTaskId())

	ids := writer.createdIDs()
	require.Len(t, ids, 1)
	assert.NotEmpty(t, ids[0])

	agentID, ok := d.TaskAgentID("agent-task-1")
	require.True(t, ok)
	assert.Equal(t, "agent-1", agentID)
}

func TestDispatcher_StartTaskRequest_UnmarshalErrorViaSession(t *testing.T) {
	d := NewDispatcher(nil)
	registerTestAgent(t, d, "agent-1")

	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-1": &fakeSessionCaller{respBody: []byte("garbage")},
	}})

	_, err := d.StartTaskRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal response")
}

func TestDispatcher_InvokeBroadcast_MixedResults(t *testing.T) {
	d := NewDispatcher(nil)
	registerTestAgent(t, d, "agent-good")
	registerTestAgent(t, d, "agent-bad")

	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-good": &fakeSessionCaller{respBody: mustMarshal(t, &sdkv1.InvokeResponse{Payload: []byte("fine")})},
		"agent-bad":  &fakeSessionCaller{respBody: []byte("garbage")},
	}})

	result, err := d.InvokeBroadcast(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.Total)
	require.Len(t, result.Successes, 1)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "agent-good", result.Successes[0].AgentID)
	assert.Equal(t, "agent-bad", result.Failures[0].AgentID)
}

func TestDispatcher_InvokeBroadcast_AllSuccess(t *testing.T) {
	d := NewDispatcher(nil)
	registerTestAgent(t, d, "agent-a")
	registerTestAgent(t, d, "agent-b")

	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-a": &fakeSessionCaller{respBody: mustMarshal(t, &sdkv1.InvokeResponse{Payload: []byte("a")})},
		"agent-b": &fakeSessionCaller{respBody: mustMarshal(t, &sdkv1.InvokeResponse{Payload: []byte("b")})},
	}})

	result, err := d.InvokeBroadcast(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)
	assert.Len(t, result.Successes, 2)
	assert.Empty(t, result.Failures)
}

func TestDispatcher_CancelTask_SuccessUnregisters(t *testing.T) {
	d := NewDispatcher(nil)
	d.RegisterTask("task-cancel", "agent-1")

	caller := &fakeSessionCaller{respBody: []byte{}}
	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-1": caller,
	}})

	require.NoError(t, d.CancelTask(context.Background(), "task-cancel"))
	assert.Equal(t, 1, caller.callCount())

	_, ok := d.TaskAgentID("task-cancel")
	assert.False(t, ok)
}

func TestDispatcher_CancelTask_CallErrorKeepsRegistration(t *testing.T) {
	d := NewDispatcher(nil)
	d.RegisterTask("task-keep", "agent-1")

	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-1": &fakeSessionCaller{err: errors.New("boom")},
	}})

	err := d.CancelTask(context.Background(), "task-keep")
	require.Error(t, err)

	_, ok := d.TaskAgentID("task-keep")
	assert.True(t, ok)
}

func TestDispatcher_InvokeRequest_HASelectsHealthyAgent(t *testing.T) {
	d := NewDispatcherWithHA(nil, nil, nil, true, StrategyMinID, nil)
	defer d.Close()
	registerTestAgent(t, d, "agent-ha")

	caller := &fakeSessionCaller{respBody: mustMarshal(t, &sdkv1.InvokeResponse{Payload: []byte("ha")})}
	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-ha": caller,
	}})

	resp, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	require.NoError(t, err)
	assert.Equal(t, []byte("ha"), resp.GetPayload())
}

func TestDispatcher_InvokeRequest_HARejectsUnavailableAgents(t *testing.T) {
	d := NewDispatcherWithHA(nil, nil, nil, true, StrategyMinID, nil)
	defer d.Close()
	registerTestAgent(t, d, "agent-sick")

	// Drive the circuit breaker open so every candidate becomes unavailable.
	state := d.GetHealthTracker().RegisterAgent("agent-sick", "")
	for i := 0; i < int(DefaultHealthCheckConfig().FailureThreshold); i++ {
		state.RecordFailure()
	}
	assert.True(t, state.IsCircuitOpen())

	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no healthy agents available")
}

func TestDispatcher_StreamTaskAfterSeq_EdgeCases(t *testing.T) {
	t.Run("empty task id", func(t *testing.T) {
		d := NewDispatcher(nil)
		_, _, err := d.StreamTaskAfterSeq(context.Background(), "  ", 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task id is required")
	})

	t.Run("query not configured", func(t *testing.T) {
		d := NewDispatcher(nil)
		_, _, err := d.StreamTaskAfterSeq(context.Background(), "task-1", 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task event query not configured")
	})

	t.Run("list events error", func(t *testing.T) {
		d := NewDispatcherWithTaskStore(nil, nil, listErrorTaskQuery{})
		_, _, err := d.StreamTaskAfterSeq(context.Background(), "task-1", 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "query events")
	})
}

type listErrorTaskQuery struct{}

func (listErrorTaskQuery) ListEvents(context.Context, string, int64) ([]TaskEventRecord, error) {
	return nil, errors.New("storage offline")
}

func (listErrorTaskQuery) GetRun(context.Context, string) (*TaskRunState, error) {
	return nil, nil
}

func TestDispatcher_StreamTaskRealtimeAfterSeq_EdgeCases(t *testing.T) {
	t.Run("empty task id", func(t *testing.T) {
		d := NewDispatcher(nil)
		_, err := d.StreamTaskRealtimeAfterSeq(context.Background(), "", 0, func(*sdkv1.TaskEvent) bool { return true })
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task id is required")
	})

	t.Run("query not configured", func(t *testing.T) {
		d := NewDispatcher(nil)
		_, err := d.StreamTaskRealtimeAfterSeq(context.Background(), "task-1", 0, func(*sdkv1.TaskEvent) bool { return true })
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task event query not configured")
	})

	t.Run("cancelled context returns ctx error", func(t *testing.T) {
		d := NewDispatcherWithTaskStore(nil, nil, staticTaskQuery{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := d.StreamTaskRealtimeAfterSeq(ctx, "task-1", 0, func(*sdkv1.TaskEvent) bool { return true })
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("done run ends stream", func(t *testing.T) {
		d := NewDispatcherWithTaskStore(nil, nil, staticTaskQuery{})
		var seen int
		done, err := d.StreamTaskRealtimeAfterSeq(context.Background(), "task-1", 0, func(*sdkv1.TaskEvent) bool {
			seen++
			return true
		})
		require.NoError(t, err)
		assert.True(t, done)
		assert.Equal(t, 2, seen)
	})

	t.Run("callback stop ends stream", func(t *testing.T) {
		d := NewDispatcherWithTaskStore(nil, nil, staticTaskQuery{})
		done, err := d.StreamTaskRealtimeAfterSeq(context.Background(), "task-1", 0, func(*sdkv1.TaskEvent) bool {
			return false
		})
		require.NoError(t, err)
		assert.True(t, done)
	})

	t.Run("list events error propagates", func(t *testing.T) {
		d := NewDispatcherWithTaskStore(nil, nil, listErrorTaskQuery{})
		_, err := d.StreamTaskRealtimeAfterSeq(context.Background(), "task-1", 0, func(*sdkv1.TaskEvent) bool { return true })
		require.Error(t, err)
		assert.Contains(t, err.Error(), "query events")
	})
}

// staticTaskQuery reports a finished task with two events.
type staticTaskQuery struct{}

func (staticTaskQuery) ListEvents(context.Context, string, int64) ([]TaskEventRecord, error) {
	return []TaskEventRecord{
		{Seq: 1, Event: &sdkv1.TaskEvent{TaskId: "task-1", Type: "progress"}},
		{Seq: 2, Event: &sdkv1.TaskEvent{TaskId: "task-1", Type: "completed"}},
	}, nil
}

func (staticTaskQuery) GetRun(context.Context, string) (*TaskRunState, error) {
	return &TaskRunState{TaskID: "task-1", Status: "succeeded"}, nil
}

func TestDispatcher_ListTaskRoutings_Variants(t *testing.T) {
	t.Run("nil dispatcher returns empty list", func(t *testing.T) {
		var d *Dispatcher
		routings, err := d.ListTaskRoutings()
		require.NoError(t, err)
		assert.Empty(t, routings)
	})

	t.Run("nil task store returns empty list", func(t *testing.T) {
		d := NewDispatcher(nil)
		d.taskStore = nil
		routings, err := d.ListTaskRoutings()
		require.NoError(t, err)
		assert.Empty(t, routings)
	})

	t.Run("returns registered routings", func(t *testing.T) {
		d := NewDispatcher(nil)
		d.RegisterTask("task-list-1", "agent-1")
		routings, err := d.ListTaskRoutings()
		require.NoError(t, err)
		require.Len(t, routings, 1)
		assert.Equal(t, "task-list-1", routings[0].TaskID)
		assert.Equal(t, "agent-1", routings[0].AgentID)
	})
}

func TestDispatcher_CleanupOldTasks_StoreError(t *testing.T) {
	d := NewDispatcher(nil)
	d.taskStore = &errorTaskRoutingStore{}
	err := d.CleanupOldTasks(time.Minute)
	require.Error(t, err)
}

func TestIsTerminalEventMixedCase(t *testing.T) {
	assert.True(t, isTerminalEvent(&sdkv1.TaskEvent{Type: "Done"}))
	assert.False(t, isTerminalEvent(&sdkv1.TaskEvent{Type: ""}))
}

func TestTaskEventsFromRecords_SkipsNilEvents(t *testing.T) {
	records := []TaskEventRecord{
		{Seq: 1, Event: &sdkv1.TaskEvent{Type: "progress"}},
		{Seq: 2},
		{Seq: 3, Event: &sdkv1.TaskEvent{Type: "log"}},
	}
	events := taskEventsFromRecords(records)
	require.Len(t, events, 2)
}
