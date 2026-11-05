//go:build !no_otel

package agentrec

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/cuihairu/croupier/core/otelrec"
)

// --- 开关语义（默认构建） ---

func TestBuild_DisabledIsNoop(t *testing.T) {
	rec, err := Build(context.Background(), otelrec.TelemetryConfig{Enabled: false}, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	assert.Nil(t, rec.mp, "disabled must not construct SDK")
	// 全部方法不 panic（零开销直通契约）。
	rec.Start(context.Background(), map[string]string{"agent": "a"})
	rec.Stop(context.Background(), errors.New("x"))
	rec.Connected(context.Background())
	rec.Disconnected(context.Background(), errors.New("y"))
	rec.Count(context.Background(), "evt", 1, nil)
	rec.Observe(context.Background(), "evt", 1.5, nil)
}

func TestBuild_EnabledNoEndpointWarnsAndNoop(t *testing.T) {
	rec, err := Build(context.Background(), otelrec.TelemetryConfig{Enabled: true}, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	assert.Nil(t, rec.mp, "enabled without endpoint must not export to default remote")
}

// --- 内存 exporter 断言（ManualReader） ---

// emitAll 打满四类观测点。
func emitAll(rec *Recorder) {
	ctx := context.Background()
	rec.Start(ctx, map[string]string{"agent": "cap-a", "role": "capture"})
	rec.Connected(ctx)
	rec.Count(ctx, "events", 5, map[string]string{"op": "insert"})
	rec.Count(ctx, "errors", 1, map[string]string{"kind": "reconnect"})
	rec.Observe(ctx, "probe_latency", 12.5, map[string]string{"target": "t1"})
	rec.Disconnected(ctx, errors.New("conn reset"))
	rec.Stop(ctx, nil)
}

// collectMetrics 用 ManualReader 拉取并返回 ResourceMetrics。
func collectMetrics(t *testing.T, reader *sdkmetric.ManualReader) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	return rm
}

// findByName 按指标名在 ScopeMetrics 中查找。
func findByName(rm metricdata.ResourceMetrics, name string) (metricdata.Metrics, bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m, true
			}
		}
	}
	return metricdata.Metrics{}, false
}

// attrKey 把 DataPoint 的 Attributes 编码为可断言的规范串（key 排序去维度序）。
func attrKey(set attribute.Set) string {
	kvs := set.ToSlice()
	sort.Slice(kvs, func(i, j int) bool { return kvs[i].Key < kvs[j].Key })
	var b strings.Builder
	for i, kv := range kvs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(string(kv.Key))
		b.WriteByte('=')
		b.WriteString(kv.Value.Emit())
	}
	return b.String()
}

func TestRecorder_EmitAndCollect(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	rec, err := newWithReader(context.Background(),
		otelrec.TelemetryConfig{ServiceName: "croupier-capture"}, reader, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	t.Cleanup(func() { _ = rec.Shutdown(context.Background()) })

	emitAll(rec)
	rm := collectMetrics(t, reader)

	// agent.lifecycle：phase=start 与 phase=stop 各一次。
	lc, ok := findByName(rm, instrumentLifecycle)
	require.True(t, ok, "lifecycle metric missing")
	lcSum, isSum := lc.Data.(metricdata.Sum[int64])
	require.True(t, isSum)
	got := map[string]int64{}
	for _, dp := range lcSum.DataPoints {
		got[attrKey(dp.Attributes)] = dp.Value
	}
	require.Contains(t, got, `agent=cap-a,phase=start,role=capture`, "start attrs merged from base: %v", got)
	require.Contains(t, got, `agent=cap-a,phase=stop,role=capture`, "stop attrs merged from base: %v", got)
	assert.EqualValues(t, 1, got[`agent=cap-a,phase=start,role=capture`])
	assert.EqualValues(t, 1, got[`agent=cap-a,phase=stop,role=capture`])

	// agent.connected：state=1 与 state=0（断连）各一次。
	cn, ok := findByName(rm, instrumentConnected)
	require.True(t, ok, "connected metric missing")
	cnSum := cn.Data.(metricdata.Sum[int64])
	connGot := map[string]int64{}
	for _, dp := range cnSum.DataPoints {
		connGot[attrKey(dp.Attributes)] = dp.Value
	}
	assert.EqualValues(t, 1, connGot[`agent=cap-a,role=capture,state=1`])
	assert.EqualValues(t, 1, connGot[`agent=cap-a,error=conn reset,role=capture,state=0`])

	// agent.events_total：events{op=insert}=5、errors=1（Disconnected 亦计 errors）。
	ev, ok := findByName(rm, instrumentEvents)
	require.True(t, ok, "events metric missing")
	evSum := ev.Data.(metricdata.Sum[int64])
	evGot := map[string]int64{}
	for _, dp := range evSum.DataPoints {
		evGot[attrKey(dp.Attributes)] = dp.Value
	}
	assert.EqualValues(t, 5, evGot[`agent=cap-a,event=events,op=insert,role=capture`])
	assert.EqualValues(t, 1, evGot[`agent=cap-a,event=errors,kind=reconnect,role=capture`])

	// agent.observe_value：histogram 记录 12.5。
	ob, ok := findByName(rm, instrumentObserve)
	require.True(t, ok, "observe metric missing")
	obHist := ob.Data.(metricdata.Histogram[float64])
	require.Len(t, obHist.DataPoints, 1)
	require.EqualValues(t, 1, obHist.DataPoints[0].Count)
	assert.Equal(t, 12.5, obHist.DataPoints[0].Sum)
}

func TestRecorder_ShutdownIdempotent(t *testing.T) {
	rec, err := Build(context.Background(), otelrec.TelemetryConfig{Enabled: false}, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	require.NoError(t, rec.Shutdown(context.Background()), "noop shutdown must be no-op")
	require.NoError(t, rec.Shutdown(context.Background()), "idempotent")
}
