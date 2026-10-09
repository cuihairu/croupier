package report

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

type fakeConn struct {
	connected    bool
	taskErr      error
	metricErr    error
	taskBodies   [][]byte
	metricBodies [][]byte
}

func (f *fakeConn) Connected() bool { return f.connected }

func (f *fakeConn) SendTaskEvent(_ context.Context, body []byte) error {
	if f.taskErr != nil {
		return f.taskErr
	}
	f.taskBodies = append(f.taskBodies, body)
	return nil
}

func (f *fakeConn) SendMetricEvent(_ context.Context, body []byte) error {
	if f.metricErr != nil {
		return f.metricErr
	}
	f.metricBodies = append(f.metricBodies, body)
	return nil
}

func TestSendMetricsRoundTrip(t *testing.T) {
	conn := &fakeConn{connected: true}
	var current Conn = conn
	r := New(func() Conn { return current })

	report := &opsv1.MetricsReport{AgentId: "agent-1"}
	require.NoError(t, r.SendMetrics(context.Background(), report))
	require.Len(t, conn.metricBodies, 1)
	assert.NotEmpty(t, conn.metricBodies[0])

	current = nil
	assert.Error(t, r.SendMetrics(context.Background(), report))
}

func TestSendTaskEventRoundTrip(t *testing.T) {
	conn := &fakeConn{connected: true}
	r := New(func() Conn { return conn })
	require.NoError(t, r.SendTaskEvent(context.Background(), &sdkv1.TaskEvent{TaskId: "t-1"}))
	require.Len(t, conn.taskBodies, 1)
}

func TestSendNilGuards(t *testing.T) {
	conn := &fakeConn{connected: true}
	r := New(func() Conn { return conn })
	assert.Error(t, r.SendMetrics(context.Background(), nil))
	assert.Error(t, r.SendTaskEvent(context.Background(), nil))

	conn.metricErr = errors.New("send fail")
	assert.ErrorIs(t, r.SendMetrics(context.Background(), &opsv1.MetricsReport{}), conn.metricErr)

	var nilReporter *Reporter
	assert.Error(t, nilReporter.SendMetrics(context.Background(), &opsv1.MetricsReport{}))
	assert.Error(t, nilReporter.SendTaskEvent(context.Background(), &sdkv1.TaskEvent{}))
}

func TestRunLoopPeriodAndCancel(t *testing.T) {
	var calls atomic.Int32
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		RunLoop(ctx, 5*time.Millisecond, func(context.Context) error {
			calls.Add(1)
			return errors.New("swallowed")
		})
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunLoop did not stop on cancel")
	}
	assert.GreaterOrEqual(t, calls.Load(), int32(2))
}
