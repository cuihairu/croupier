package analytics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// scopeRowsV9 replays fixed (game, env, recent, baseline) rows for the
// pipeline monitor events query; scanErr forces the scan-failure branch.
type scopeRowsV9 struct {
	rows    [][]any
	index   int
	scanErr error
}

func (r *scopeRowsV9) Next() bool { r.index++; return r.index <= len(r.rows) }

func (r *scopeRowsV9) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.rows[r.index-1]
	for i, d := range dest {
		switch target := d.(type) {
		case *string:
			*target = row[i].(string)
		case *int64:
			*target = row[i].(int64)
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}

func (r *scopeRowsV9) Close() error { return nil }

// errDeadCounterV9 simulates an unreachable Redis for XLen-based checks.
type errDeadCounterV9 struct{}

func (errDeadCounterV9) XLen(_ context.Context, _ string) *redis.IntCmd {
	return redis.NewIntResult(0, errors.New("redis unavailable"))
}

// updatedStatusV9 records one UpdateStatus call on the fake sink.
type updatedStatusV9 struct {
	id     uint
	status string
}

// fakeAlertSinkV9 is a scriptable PipelineAlertSink for error branches.
type fakeAlertSinkV9 struct {
	findErr   error
	createErr error
	updateErr error
	existing  *model.Alert
	created   []*model.Alert
	updated   []updatedStatusV9
}

func (f *fakeAlertSinkV9) Create(_ context.Context, alert *model.Alert) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, alert)
	return nil
}

func (f *fakeAlertSinkV9) FindByAlertID(_ context.Context, _ string) (*model.Alert, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.existing, nil
}

func (f *fakeAlertSinkV9) UpdateStatus(_ context.Context, id uint, status string) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = append(f.updated, updatedStatusV9{id: id, status: status})
	return nil
}

// newScopeMonitorV9 wires a monitor to a conn that returns the scripted rows
// (or queryErr) for the events-stream query.
func newScopeMonitorV9(sink PipelineAlertSink, rows warehouseRows, queryErr error) *PipelineMonitor {
	m := NewPipelineMonitor(sink, nil)
	m.conn = func() (warehouseConn, error) { return &stubConn{rows: rows, queryErr: queryErr}, nil }
	return m
}

func TestPipelineConfigFromEnvOverridesV9(t *testing.T) {
	t.Setenv("PIPELINE_MONITOR_INTERVAL", "90s")
	t.Setenv("PIPELINE_MONITOR_DEAD_THRESHOLD", "5")
	t.Setenv("PIPELINE_MONITOR_MQ_BACKLOG", "55")
	cfg := pipelineConfigFromEnv()
	assert.Equal(t, 90*time.Second, cfg.Interval)
	assert.Equal(t, int64(5), cfg.DeadThreshold)
	assert.Equal(t, int64(55), cfg.MQBacklogLimit)

	t.Setenv("PIPELINE_MONITOR_INTERVAL", "not-a-duration")
	t.Setenv("PIPELINE_MONITOR_DEAD_THRESHOLD", "-3")
	t.Setenv("PIPELINE_MONITOR_MQ_BACKLOG", "oops")
	cfg = pipelineConfigFromEnv()
	assert.Equal(t, 60*time.Second, cfg.Interval)
	assert.Equal(t, int64(100), cfg.DeadThreshold)
	assert.Equal(t, int64(10000), cfg.MQBacklogLimit)

	t.Setenv("PIPELINE_MONITOR_INTERVAL", "0s")
	cfg = pipelineConfigFromEnv()
	assert.Equal(t, 60*time.Second, cfg.Interval)
}

func TestEnvOrV9(t *testing.T) {
	assert.Equal(t, "fallback", envOr("ENV_OR_V9_MISSING", "fallback"))
	t.Setenv("ENV_OR_V9_MISSING", "  value  ")
	assert.Equal(t, "value", envOr("ENV_OR_V9_MISSING", "fallback"))
}

func TestScopeKeyV9(t *testing.T) {
	assert.Equal(t, "game/env", scopeKey("game", "env"))
}

func TestMonitorRunV9(t *testing.T) {
	sink := newMonitorTestSink(t)
	m := NewPipelineMonitor(sink, nil)
	m.cfg.Interval = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.Run(ctx)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}

	// Non-positive interval falls back to one minute; cancellation still returns.
	m2 := NewPipelineMonitor(sink, nil)
	m2.cfg.Interval = -1
	ctx2, cancel2 := context.WithCancel(context.Background())
	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		m2.Run(ctx2)
	}()
	cancel2()
	select {
	case <-done2:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return with non-positive interval")
	}
}

func TestMonitorCheckNilGuardsV9(t *testing.T) {
	var nilMonitor *PipelineMonitor
	require.NoError(t, nilMonitor.Check(context.Background()))

	m := NewPipelineMonitor(nil, nil)
	require.NoError(t, m.Check(context.Background()))
}

func TestMonitorEventStreamsFireAndRecoverV9(t *testing.T) {
	sink := newMonitorTestSink(t)
	firing := &scopeRowsV9{rows: [][]any{
		{"stall-game", "prod", int64(0), int64(1000)}, // recent==0 → stalled
		{"drop-game", "prod", int64(10), int64(6000)}, // rate 2 < 20 → drop
		{"healthy-game", "prod", int64(1000), int64(6000)},
		{"low-game", "prod", int64(0), int64(10)}, // baseline below min → ignored
		{"", "", int64(0), int64(5000)},           // blank scope skipped
	}}
	m := newScopeMonitorV9(sink, firing, nil)

	require.NoError(t, m.Check(context.Background()))
	stalled, err := sink.FindByAlertID(context.Background(), "pipe:event_stream_stalled:stall-game/prod")
	require.NoError(t, err)
	assert.Equal(t, PipelineAlertStatusFiring, stalled.Status)
	assert.Equal(t, AlertTypeDataPipeline, stalled.Type)
	dropped, err := sink.FindByAlertID(context.Background(), "pipe:event_volume_drop:drop-game/prod")
	require.NoError(t, err)
	assert.Equal(t, PipelineAlertStatusFiring, dropped.Status)
	assert.Equal(t, AlertTypeDataQuality, dropped.Type)

	// A second firing round keeps dedup and hits the "still firing" branch.
	require.NoError(t, m.Check(context.Background()))

	// Healthy rows resolve both alerts via resolveAsync.
	healthy := &scopeRowsV9{rows: [][]any{
		{"stall-game", "prod", int64(1000), int64(1000)},
		{"drop-game", "prod", int64(1000), int64(1000)},
	}}
	m.conn = func() (warehouseConn, error) { return &stubConn{rows: healthy}, nil }
	require.NoError(t, m.Check(context.Background()))
	require.Eventually(t, func() bool {
		got, err := sink.FindByAlertID(context.Background(), "pipe:event_stream_stalled:stall-game/prod")
		return err == nil && got.Status == PipelineAlertStatusResolved
	}, 3*time.Second, 20*time.Millisecond)
	require.Eventually(t, func() bool {
		got, err := sink.FindByAlertID(context.Background(), "pipe:event_volume_drop:drop-game/prod")
		return err == nil && got.Status == PipelineAlertStatusResolved
	}, 3*time.Second, 20*time.Millisecond)
}

func TestMonitorEventStreamQueryFailureFiresAlertV9(t *testing.T) {
	sink := newMonitorTestSink(t)
	m := newScopeMonitorV9(sink, nil, errors.New("query boom"))
	require.NoError(t, m.Check(context.Background()))
	alert, err := sink.FindByAlertID(context.Background(), "pipe:events_query_failed")
	require.NoError(t, err)
	assert.Equal(t, PipelineAlertStatusFiring, alert.Status)
	assert.Equal(t, AlertTypeDataPipeline, alert.Type)
}

func TestMonitorEventStreamScanErrorSkipsRowV9(t *testing.T) {
	sink := newMonitorTestSink(t)
	rows := &scopeRowsV9{
		rows:    [][]any{{"g", "prod", int64(0), int64(1000)}},
		scanErr: errors.New("scan boom"),
	}
	m := newScopeMonitorV9(sink, rows, nil)
	require.NoError(t, m.Check(context.Background()))
	alerts, _, err := sink.List(context.Background(), model.ListAlertsOptions{})
	require.NoError(t, err)
	assert.Empty(t, alerts)
}

func TestMonitorDeadLetterAndMQRedisErrorsAreSkippedV9(t *testing.T) {
	sink := newMonitorTestSink(t)
	m := NewPipelineMonitor(sink, errDeadCounterV9{})
	require.NoError(t, m.Check(context.Background()))
	alerts, _, err := sink.List(context.Background(), model.ListAlertsOptions{})
	require.NoError(t, err)
	assert.Empty(t, alerts)
}

func TestMonitorFireAlertSinkErrorBranchesV9(t *testing.T) {
	ctx := context.Background()

	m := NewPipelineMonitor(&fakeAlertSinkV9{findErr: errors.New("db exploded")}, nil)
	m.fireAlert(ctx, "k", AlertTypeDataPipeline, "critical", "msg", nil)

	sink := &fakeAlertSinkV9{createErr: errors.New("insert failed")}
	m2 := NewPipelineMonitor(sink, nil)
	m2.fireAlert(ctx, "k2", AlertTypeDataPipeline, "critical", "msg", nil)
	assert.Empty(t, sink.created)

	// Re-fire path: an existing resolved alert flips back to firing.
	resolved := &model.Alert{AlertID: "pipe:k3", Status: PipelineAlertStatusResolved}
	sink3 := &fakeAlertSinkV9{existing: resolved}
	m3 := NewPipelineMonitor(sink3, nil)
	m3.fireAlert(ctx, "k3", AlertTypeDataPipeline, "critical", "msg", nil)
	require.Len(t, sink3.updated, 1)
	assert.Equal(t, PipelineAlertStatusFiring, sink3.updated[0].status)
}

func TestMonitorResolveIfFiringBranchesV9(t *testing.T) {
	ctx := context.Background()

	m := NewPipelineMonitor(nil, nil)
	m.resolveIfFiring(ctx, "k", "recovered") // nil sink guard

	m2 := NewPipelineMonitor(&fakeAlertSinkV9{findErr: errors.New("boom")}, nil)
	m2.resolveIfFiring(ctx, "k", "recovered")

	m3 := NewPipelineMonitor(&fakeAlertSinkV9{existing: &model.Alert{Status: PipelineAlertStatusResolved}}, nil)
	m3.resolveIfFiring(ctx, "k", "recovered")

	m4 := NewPipelineMonitor(&fakeAlertSinkV9{
		existing:  &model.Alert{Status: PipelineAlertStatusFiring},
		updateErr: errors.New("update failed"),
	}, nil)
	m4.resolveIfFiring(ctx, "k", "recovered")

	sink := &fakeAlertSinkV9{existing: &model.Alert{Model: gorm.Model{ID: 1}, Status: PipelineAlertStatusFiring}}
	m5 := NewPipelineMonitor(sink, nil)
	m5.resolveIfFiring(ctx, "k", "recovered")
	require.Len(t, sink.updated, 1)
	assert.Equal(t, PipelineAlertStatusResolved, sink.updated[0].status)
}

func TestMonitorResolveAsyncV9(t *testing.T) {
	sink := newMonitorTestSink(t)
	m := NewPipelineMonitor(sink, nil)
	alert := &model.Alert{
		AlertID: "pipe:event_stream_stalled:g/prod",
		Type:    AlertTypeDataPipeline,
		Level:   "critical",
		Message: "m",
		Source:  PipelineAlertSource,
		Status:  PipelineAlertStatusFiring,
	}
	require.NoError(t, sink.Create(context.Background(), alert))
	m.states["event_stream_stalled:g/prod"] = true

	m.resolveAsync(context.Background(), "event_stream_stalled:g/prod")
	require.Eventually(t, func() bool {
		got, err := sink.FindByAlertID(context.Background(), "pipe:event_stream_stalled:g/prod")
		return err == nil && got.Status == PipelineAlertStatusResolved
	}, 3*time.Second, 20*time.Millisecond)
	assert.NotContains(t, m.states, "event_stream_stalled:g/prod")
}
