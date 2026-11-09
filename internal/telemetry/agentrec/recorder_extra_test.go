//go:build !no_otel

package agentrec

// recorder 补测：Build 的 nil logger 缺省、OTLP 端点装配（URL 形态 / host:port
// 形态 / insecure）、trimEndpoint 归一、默认 serviceName 资源标注、Stop 携带
// 错误属性。
//
// 诚实跳过：
//   - newOTLP 的 exporter 构造错误臂（91/95）：otlpmetrichttp.New 对无效 URL
//     静默容错、其余选项延迟校验，构造期错误不可复现。
//   - newWithReader 的 resource.New 错误臂（110）与四个 instrument 创建错误臂
//     （121/125/129/133）：合法资源属性与合法指标名下构造恒成功。

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"github.com/cuihairu/croupier/core/otelrec"
)

// Build：logger 为 nil 时回退 slog.Default()（69）。
func TestBuild_NilLoggerDefaults(t *testing.T) {
	rec, err := Build(context.Background(), otelrec.TelemetryConfig{Enabled: false}, nil)
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.NoError(t, rec.Shutdown(context.Background()))
}

// Build：Enabled + URL 形态端点 → 真实 OTLP 装配（79/84-87/88 insecure/90/93）。
func TestBuild_OTLPURLEndpoint(t *testing.T) {
	rec, err := Build(context.Background(), otelrec.TelemetryConfig{
		Enabled: true, OTLPEndpoint: "http://127.0.0.1:4318", Insecure: true,
	}, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	require.NotNil(t, rec.mp, "enabled with endpoint must construct SDK")
	// 无采集端，flush 失败不影响装配语义；尽快释放导出器。
	_ = rec.Shutdown(context.Background())
}

// Build：Enabled + host:port 形态端点 → else 分支装配（88-89）。
func TestBuild_OTLPHostPortEndpoint(t *testing.T) {
	rec, err := Build(context.Background(), otelrec.TelemetryConfig{
		Enabled: true, OTLPEndpoint: "127.0.0.1:4318", Insecure: true,
	}, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	require.NotNil(t, rec.mp)
	_ = rec.Shutdown(context.Background())
}

// trimEndpoint：缺 path 拼 defaultPath（230）；已带 /v1/ 保持原样（232）。
func TestTrimEndpoint(t *testing.T) {
	assert.Equal(t, "127.0.0.1:4318/v1/metrics", trimEndpoint("127.0.0.1:4318", "/v1/metrics"))
	assert.Equal(t, "http://h:4318/v1/metrics", trimEndpoint("http://h:4318/", "/v1/metrics"))
	assert.Equal(t, "http://h:4318/custom/v1/metrics",
		trimEndpoint("http://h:4318/custom/v1/metrics/", "/v1/metrics"))
}

// newWithReader：serviceName 缺省回退 croupier-agent 并落到资源标注（106）。
func TestNewWithReader_DefaultsServiceName(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	rec, err := newWithReader(ctx, otelrec.TelemetryConfig{}, reader, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	t.Cleanup(func() { _ = rec.Shutdown(ctx) })

	rec.Count(ctx, "evt", 1, nil)
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &rm))
	v, ok := rm.Resource.Set().Value(semconv.ServiceNameKey)
	require.True(t, ok, "service.name resource attribute missing")
	assert.Equal(t, "croupier-agent", v.AsString())
}

// Stop：携带错误时 lifecycle 追加 error 属性（163）。
func TestRecorder_StopWithErrorAttr(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	rec, err := newWithReader(ctx, otelrec.TelemetryConfig{ServiceName: "s"}, reader, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	t.Cleanup(func() { _ = rec.Shutdown(ctx) })

	rec.Start(ctx, map[string]string{"agent": "a"})
	rec.Stop(ctx, errors.New("boom"))

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &rm))
	lc, ok := findByName(rm, instrumentLifecycle)
	require.True(t, ok)
	lcSum := lc.Data.(metricdata.Sum[int64])
	found := false
	for _, dp := range lcSum.DataPoints {
		if attrKey(dp.Attributes) == "agent=a,error=boom,phase=stop" {
			found = true
		}
	}
	assert.True(t, found, "stop 数据点必须携带 error 属性")
}
