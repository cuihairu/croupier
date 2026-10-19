package telemetry

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireFullAssembly 在 no_otel 裁剪构建中跳过依赖 OTel SDK/exporter
// 装配的用例（这些用例在默认构建照常运行；skip 数如实计入结果）。
func requireFullAssembly(t *testing.T) {
	t.Helper()
	if otelAssemblyTrimmed {
		t.Skip("OTel SDK/exporter assembly trimmed (no_otel build)")
	}
}

// --- 总开关门控（todo.md:31，与 go-plugin 同策略） ---

// 总关闭必须压过子开关：Enabled=false 时 enable_tracing/enable_metrics
// 即使全开，也不得创建 TracerProvider/MeterProvider（无 exporter、无周期
// 导出任务），业务面（GameMetrics noop / Bridge 自守门 / GameTracer）保持
// 可用。此用例在默认与 no_otel 裁剪构建下都必须成立。
func TestNewProvider_MasterOffBeatsSubSwitches(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:       false,
		EnableTracing: true,
		EnableMetrics: true,
		CollectorURL:  "localhost:4318",
	}
	p, err := NewProvider(context.Background(), cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Nil(t, p.TracerProvider, "master off must not assemble tracer provider")
	assert.Nil(t, p.MeterProvider, "master off must not assemble meter provider")
	assert.NotNil(t, p.GameMetrics, "lightweight business metrics stay available on noop meter")
	assert.NotNil(t, p.GameTracer)
	assert.NotNil(t, p.Bridge)
	require.NoError(t, p.Shutdown(context.Background()))
}

// MergeEnv 层面：子开关环境变量不得重新点亮总开关（总关闭不得被子开关
// 重新开启，todo.md:31）。
func TestMergeEnv_SubSwitchesCannotLightMaster(t *testing.T) {
	_ = os.Setenv("OTEL_ENABLE_TRACING", "true")
	_ = os.Setenv("OTEL_ENABLE_METRICS", "1")
	_ = os.Setenv("ANALYTICS_BRIDGE_ENABLED", "true")
	defer func() {
		_ = os.Unsetenv("OTEL_ENABLE_TRACING")
		_ = os.Unsetenv("OTEL_ENABLE_METRICS")
		_ = os.Unsetenv("ANALYTICS_BRIDGE_ENABLED")
	}()

	result := MergeEnv(TelemetryConfig{Enabled: false})

	assert.False(t, result.Enabled, "sub-switches must not re-enable the master switch")
	assert.True(t, result.EnableTracing)
	assert.True(t, result.EnableMetrics)
	assert.True(t, result.Analytics.Enabled)
}

// LoadConfigFromEnv 层面：只有 OTEL_ENABLED/CROUPIER_TELEMETRY_ENABLED 能
// 点亮总开关；且未配置采集端时 CollectorURL 保持为空（不填默认远端）。
func TestLoadConfigFromEnv_SubSwitchesCannotLightMaster(t *testing.T) {
	for _, key := range []string{
		"OTEL_ENABLED", "CROUPIER_TELEMETRY_ENABLED",
		"OTEL_ENABLE_TRACING", "OTEL_ENABLE_METRICS",
		"ANALYTICS_BRIDGE_ENABLED", "OTEL_EXPORTER_OTLP_ENDPOINT",
	} {
		_ = os.Unsetenv(key)
	}
	_ = os.Setenv("OTEL_ENABLE_TRACING", "true")
	_ = os.Setenv("OTEL_ENABLE_METRICS", "true")
	_ = os.Setenv("ANALYTICS_BRIDGE_ENABLED", "true")
	defer func() {
		_ = os.Unsetenv("OTEL_ENABLE_TRACING")
		_ = os.Unsetenv("OTEL_ENABLE_METRICS")
		_ = os.Unsetenv("ANALYTICS_BRIDGE_ENABLED")
	}()

	cfg := LoadConfigFromEnv()

	assert.False(t, cfg.Enabled, "sub-switch envs must not light the master switch")
	assert.True(t, cfg.EnableTracing)
	assert.True(t, cfg.EnableMetrics)
	assert.Empty(t, cfg.CollectorURL, "unconfigured collector must stay empty (no default remote)")
}
