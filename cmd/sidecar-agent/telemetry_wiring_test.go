package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/telemetry"
)

// sidecarTelemetryBaseline 返回最小可启动配置（无遥测段）。
func sidecarTelemetryBaseline() *AgentConfig {
	return &AgentConfig{
		Server: AgentServerConfig{Insecure: true},
		Agent:  AgentInfoConfig{GameID: "g", Env: "e", LocalAddr: "127.0.0.1:0", ID: "tel-agent"},
	}
}

// 清掉 OTEL_* 环境变量，保证用例只受 YAML 段影响。
func isolateTelemetryEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"OTEL_ENABLED", "CROUPIER_TELEMETRY_ENABLED", "OTEL_ENABLE_TRACING",
		"OTEL_ENABLE_METRICS", "ANALYTICS_BRIDGE_ENABLED", "OTEL_EXPORTER_OTLP_ENDPOINT",
	} {
		if old, ok := os.LookupEnv(key); ok {
			require.NoError(t, os.Unsetenv(key))
			t.Cleanup(func() { _ = os.Setenv(key, old) })
		}
	}
}

// 缺省（未配置 telemetry 段）=总开关关闭：不建遥测服务，进程内零
// exporter/零周期导出任务（todo.md:31 默认集成=编译含 OTel，运行时开关收口）。
func TestStartAgentCore_TelemetryMasterOffByDefault(t *testing.T) {
	saveRootGlobals(t)
	isolateTelemetryEnv(t)
	dir := t.TempDir()
	cfgFile = filepath.Join(dir, "agent.yaml")

	core, _, _, err := startAgentCore(context.Background(), sidecarTelemetryBaseline(), dir)
	require.NoError(t, err)
	t.Cleanup(core.Stop)
	assert.Nil(t, core.TelemetryService(), "master off must not wire a telemetry service")
}

// buildAgentTelemetryConfig：service name 缺省 croupier-agent，子开关透传，
// 总开关仍只由 enabled 决定（子开关不得点亮总开关）。
func TestBuildAgentTelemetryConfigDefaultsAndMasterGate(t *testing.T) {
	isolateTelemetryEnv(t)

	cfg := buildAgentTelemetryConfig(&AgentConfig{})
	assert.Equal(t, "croupier-agent", cfg.ServiceName)
	assert.False(t, cfg.Enabled)

	in := &AgentConfig{Telemetry: telemetry.TelemetryConfig{
		Enabled:       false,
		EnableTracing: true,
		EnableMetrics: true,
	}}
	resolved := buildAgentTelemetryConfig(in)
	assert.False(t, resolved.Enabled, "sub-switches must not light the master switch")
	assert.True(t, resolved.EnableTracing)
	assert.True(t, resolved.EnableMetrics)
}
