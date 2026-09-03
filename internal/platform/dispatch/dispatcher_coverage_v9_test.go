package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDispatcherInvokeAndStartTaskSuccessV9(t *testing.T) {
	t.Run("invoke returns payload", func(t *testing.T) {
		d := NewDispatcher(nil)
		registerTestAgent(t, d, "agent-1")
		caller := &fakeSessionCaller{respBody: mustMarshal(t, &sdkv1.InvokeResponse{Payload: []byte("pong")})}
		d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{"agent-1": caller}})

		payload, err := d.Invoke(context.Background(), "test-func", []byte("ping"))
		require.NoError(t, err)
		assert.Equal(t, []byte("pong"), payload)
	})

	t.Run("invoke honors declared timeout budget", func(t *testing.T) {
		d := NewDispatcher(nil)
		registerTestAgent(t, d, "agent-1")
		caller := &fakeSessionCaller{respBody: mustMarshal(t, &sdkv1.InvokeResponse{Payload: []byte("ok")})}
		d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{"agent-1": caller}})

		_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{
			FunctionId: "test-func",
			Metadata:   map[string]string{"timeoutMs": "2000"},
		})
		require.NoError(t, err)
		assert.Equal(t, 1, caller.callCount())
	})

	t.Run("start task returns task id", func(t *testing.T) {
		d := NewDispatcher(nil)
		registerTestAgent(t, d, "agent-1")
		caller := &fakeSessionCaller{respBody: mustMarshal(t, &sdkv1.StartTaskResponse{TaskId: "task-42"})}
		d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{"agent-1": caller}})

		taskID, err := d.StartTask(context.Background(), "test-func", []byte("payload"))
		require.NoError(t, err)
		assert.Equal(t, "task-42", taskID)
	})
}

func TestDispatcherInvokeRequestFailoverLoopExhaustedV9(t *testing.T) {
	d := NewDispatcher(nil)
	registerTestAgent(t, d, "agent-1")
	registerTestAgent(t, d, "agent-2")
	registerTestAgent(t, d, "agent-3")

	fwd := &stubRemoteForwarder{err: errors.New("no route")}
	d.SetRemoteForwarder(fwd)

	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	require.Error(t, err)
	assert.ErrorIs(t, err, errAgentUnreachable)
	assert.Len(t, fwd.calls, 3)
}

func TestDispatcherInvokeRequestMarshalErrorsV9(t *testing.T) {
	t.Run("plain dispatcher returns marshal error", func(t *testing.T) {
		d := NewDispatcher(nil)
		registerTestAgent(t, d, "agent-1")

		_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{
			FunctionId: "test-func",
			Metadata:   map[string]string{"bad": "\xff\xfe"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshal request")
	})

	t.Run("ha dispatcher records failure", func(t *testing.T) {
		d := NewDispatcherWithHA(nil, nil, nil, true, StrategyMinID, nil)
		defer d.Close()
		registerTestAgent(t, d, "agent-1")
		d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
			"agent-1": &fakeSessionCaller{respBody: mustMarshal(t, &sdkv1.InvokeResponse{})},
		}})

		_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{
			FunctionId: "test-func",
			Metadata:   map[string]string{"bad": "\xff\xfe"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshal request")

		stats, statErr := d.GetHealthTracker().GetStatistics("agent-1")
		require.NoError(t, statErr)
		assert.Greater(t, stats.FailedRequests, int64(0))
	})
}

func TestDispatcherInvokeRequestHARecordFailureOnCallErrorV9(t *testing.T) {
	d := NewDispatcherWithHA(nil, nil, nil, true, StrategyMinID, nil)
	defer d.Close()
	registerTestAgent(t, d, "agent-1")
	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-1": &fakeSessionCaller{err: errors.New("connection reset")},
	}})

	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection reset")

	stats, statErr := d.GetHealthTracker().GetStatistics("agent-1")
	require.NoError(t, statErr)
	assert.Greater(t, stats.FailedRequests, int64(0))
}

func TestDispatcherInvokeBroadcastValidationAndMarshalV9(t *testing.T) {
	t.Run("partial game scope rejected", func(t *testing.T) {
		d := NewDispatcher(nil)
		_, err := d.InvokeBroadcast(context.Background(), &sdkv1.InvokeRequest{
			FunctionId: "test-func",
			Metadata:   map[string]string{"gameId": "game-a"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be supplied together")
	})

	t.Run("marshal failure captured per agent", func(t *testing.T) {
		d := NewDispatcher(nil)
		registerTestAgent(t, d, "agent-1")

		result, err := d.InvokeBroadcast(context.Background(), &sdkv1.InvokeRequest{
			FunctionId: "test-func",
			Metadata:   map[string]string{"bad": "\xff\xfe"},
		})
		require.NoError(t, err)
		assert.Equal(t, 1, result.Total)
		require.Len(t, result.Failures, 1)
		assert.Contains(t, result.Failures[0].Err.Error(), "marshal request")
	})
}

func TestDispatcherInvokeBroadcastHAHealthTrackingV9(t *testing.T) {
	d := NewDispatcherWithHA(nil, nil, nil, true, StrategyMinID, nil)
	defer d.Close()
	registerTestAgent(t, d, "agent-good")
	registerTestAgent(t, d, "agent-bad")
	d.GetHealthTracker().RegisterAgent("agent-good", "")
	d.GetHealthTracker().RegisterAgent("agent-bad", "")
	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
		"agent-good": &fakeSessionCaller{respBody: mustMarshal(t, &sdkv1.InvokeResponse{})},
		"agent-bad":  &fakeSessionCaller{respBody: []byte("garbage")},
	}})

	result, err := d.InvokeBroadcast(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)
	require.Len(t, result.Successes, 1)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "agent-good", result.Successes[0].AgentID)
	assert.Equal(t, "agent-bad", result.Failures[0].AgentID)

	goodStats, statErr := d.GetHealthTracker().GetStatistics("agent-good")
	require.NoError(t, statErr)
	assert.Greater(t, goodStats.SuccessfulRequests, int64(0))

	badStats, statErr := d.GetHealthTracker().GetStatistics("agent-bad")
	require.NoError(t, statErr)
	assert.Greater(t, badStats.FailedRequests, int64(0))
}

func TestDispatcherStartTaskRequestHAPathsV9(t *testing.T) {
	newHADispatcherV9 := func(t *testing.T, caller *fakeSessionCaller) *Dispatcher {
		t.Helper()
		d := NewDispatcherWithHA(nil, nil, nil, true, StrategyMinID, nil)
		t.Cleanup(func() { _ = d.Close() })
		registerTestAgent(t, d, "agent-1")
		d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{"agent-1": caller}})
		return d
	}

	t.Run("marshal error records failure", func(t *testing.T) {
		d := newHADispatcherV9(t, &fakeSessionCaller{respBody: []byte{}})
		_, err := d.StartTaskRequest(context.Background(), &sdkv1.InvokeRequest{
			FunctionId: "test-func",
			Metadata:   map[string]string{"bad": "\xff\xfe"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshal request")

		stats, statErr := d.GetHealthTracker().GetStatistics("agent-1")
		require.NoError(t, statErr)
		assert.Greater(t, stats.FailedRequests, int64(0))
	})

	t.Run("call error records failure", func(t *testing.T) {
		d := newHADispatcherV9(t, &fakeSessionCaller{err: errors.New("agent down")})
		_, err := d.StartTaskRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agent down")

		stats, statErr := d.GetHealthTracker().GetStatistics("agent-1")
		require.NoError(t, statErr)
		assert.Greater(t, stats.FailedRequests, int64(0))
	})

	t.Run("unmarshal error records failure", func(t *testing.T) {
		d := newHADispatcherV9(t, &fakeSessionCaller{respBody: []byte("garbage")})
		_, err := d.StartTaskRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal response")

		stats, statErr := d.GetHealthTracker().GetStatistics("agent-1")
		require.NoError(t, statErr)
		assert.Greater(t, stats.FailedRequests, int64(0))
	})

	t.Run("success records success and registers task", func(t *testing.T) {
		d := newHADispatcherV9(t, &fakeSessionCaller{respBody: mustMarshal(t, &sdkv1.StartTaskResponse{TaskId: "task-ha"})})
		resp, err := d.StartTaskRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "test-func"})
		require.NoError(t, err)
		assert.Equal(t, "task-ha", resp.GetTaskId())

		agentID, ok := d.TaskAgentID("task-ha")
		require.True(t, ok)
		assert.Equal(t, "agent-1", agentID)

		stats, statErr := d.GetHealthTracker().GetStatistics("agent-1")
		require.NoError(t, statErr)
		assert.Greater(t, stats.SuccessfulRequests, int64(0))
	})
}

func TestDispatcherCancelTaskMarshalErrorV9(t *testing.T) {
	d := NewDispatcher(nil)
	d.RegisterTask("\xff", "agent-1")

	err := d.CancelTask(context.Background(), "\xff")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal request")

	_, ok := d.TaskAgentID("\xff")
	assert.True(t, ok, "registration must be kept when cancel fails")
}

type runNotFoundTaskQueryV9 struct{}

func (runNotFoundTaskQueryV9) ListEvents(context.Context, string, int64) ([]TaskEventRecord, error) {
	return []TaskEventRecord{{Seq: 1, Event: &sdkv1.TaskEvent{TaskId: "task-x", Type: "progress"}}}, nil
}

func (runNotFoundTaskQueryV9) GetRun(context.Context, string) (*TaskRunState, error) {
	return nil, ErrTaskRunNotFound
}

func TestDispatcherStreamTaskAfterSeqRunNotFoundV9(t *testing.T) {
	d := NewDispatcherWithTaskStore(nil, nil, runNotFoundTaskQueryV9{})
	events, done, err := d.StreamTaskAfterSeq(context.Background(), "task-x", 0)
	require.NoError(t, err)
	assert.False(t, done)
	require.Len(t, events, 1)
	assert.Equal(t, "progress", events[0].GetType())
}

func TestDispatcherStreamTaskRealtimeRunQueryErrorV9(t *testing.T) {
	d := NewDispatcherWithTaskStore(nil, nil, runErrorTaskQuery{})
	_, err := d.StreamTaskRealtimeAfterSeq(context.Background(), "task-error", 0, func(*sdkv1.TaskEvent) bool {
		return true
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query task run")
}

type nilEventDoneTaskQueryV9 struct{}

func (nilEventDoneTaskQueryV9) ListEvents(context.Context, string, int64) ([]TaskEventRecord, error) {
	return []TaskEventRecord{
		{Seq: 1},
		{Seq: 2, Event: &sdkv1.TaskEvent{TaskId: "task-n", Type: "completed"}},
	}, nil
}

func (nilEventDoneTaskQueryV9) GetRun(context.Context, string) (*TaskRunState, error) {
	return &TaskRunState{TaskID: "task-n", Status: "succeeded"}, nil
}

func TestDispatcherStreamTaskRealtimeSkipsNilEventsV9(t *testing.T) {
	d := NewDispatcherWithTaskStore(nil, nil, nilEventDoneTaskQueryV9{})
	var seen int
	done, err := d.StreamTaskRealtimeAfterSeq(context.Background(), "task-n", 0, func(*sdkv1.TaskEvent) bool {
		seen++
		return true
	})
	require.NoError(t, err)
	assert.True(t, done)
	assert.Equal(t, 1, seen, "nil event records must be skipped")
}

func TestDispatcherSelectAgentNoCandidatesAfterBuildV9(t *testing.T) {
	d := NewDispatcherWithHA(nil, nil, nil, true, StrategyMinID, nil)
	defer d.Close()

	session := &reg.AgentSession{
		AgentID:   "", // skipped by BuildCandidates
		ExpireAt:  time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{"test-func": {Enabled: true}},
	}
	_, err := d.selectAgent("test-func", []*reg.AgentSession{session}, "", "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no healthy agents available")
}

func TestDispatcherInvokeRequestOnAgentV9(t *testing.T) {
	ctx := context.Background()

	t.Run("nil request rejected", func(t *testing.T) {
		d := NewDispatcher(nil)
		_, err := d.InvokeRequestOnAgent(ctx, "agent-1", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agent id is required")
	})

	t.Run("blank agent id rejected", func(t *testing.T) {
		d := NewDispatcher(nil)
		_, err := d.InvokeRequestOnAgent(ctx, "   ", &sdkv1.InvokeRequest{FunctionId: "f"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agent id is required")
	})

	t.Run("marshal error", func(t *testing.T) {
		d := NewDispatcher(nil)
		_, err := d.InvokeRequestOnAgent(ctx, "agent-1", &sdkv1.InvokeRequest{FunctionId: "\xff"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshal request")
	})

	t.Run("call failure records health failure", func(t *testing.T) {
		d := NewDispatcherWithHA(nil, nil, nil, true, StrategyMinID, nil)
		defer d.Close()
		d.GetHealthTracker().RegisterAgent("agent-1", "")
		d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{
			"agent-1": &fakeSessionCaller{err: errors.New("boom")},
		}})

		_, err := d.InvokeRequestOnAgent(ctx, "agent-1", &sdkv1.InvokeRequest{FunctionId: "f"})
		require.Error(t, err)

		stats, statErr := d.GetHealthTracker().GetStatistics("agent-1")
		require.NoError(t, statErr)
		assert.Greater(t, stats.FailedRequests, int64(0))
	})

	t.Run("success returns raw bytes and records success", func(t *testing.T) {
		d := NewDispatcherWithHA(nil, nil, nil, true, StrategyMinID, nil)
		defer d.Close()
		d.GetHealthTracker().RegisterAgent("agent-1", "")
		caller := &fakeSessionCaller{respBody: []byte("raw-response")}
		d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{"agent-1": caller}})

		out, err := d.InvokeRequestOnAgent(ctx, "agent-1", &sdkv1.InvokeRequest{FunctionId: "f"})
		require.NoError(t, err)
		assert.Equal(t, []byte("raw-response"), out)

		stats, statErr := d.GetHealthTracker().GetStatistics("agent-1")
		require.NoError(t, statErr)
		assert.Greater(t, stats.SuccessfulRequests, int64(0))
	})
}

func TestDispatcherUnregisterTaskStoreDeleteErrorV9(t *testing.T) {
	d := NewDispatcherWithTaskStore(nil, &errorTaskRoutingStore{}, nil)
	d.UnregisterTask("task-gone")

	_, ok := d.TaskAgentID("task-gone")
	assert.False(t, ok, "memory cache entry must be removed even if store delete fails")
}
