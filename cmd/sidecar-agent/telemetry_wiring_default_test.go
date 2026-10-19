//go:build !no_otel

package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// 总开关开启 → sidecar 命令入口接通遥测面（todo.md:31 接通 sidecar 命令
// 入口配置与生命周期）：App 持有服务，App.Stop 统一有界 Shutdown。
// 未配置采集端 → 不建 exporter/周期导出任务，服务仍可创建（轻量面）。
func TestStartAgentCore_TelemetryMasterOnWiresService(t *testing.T) {
	saveRootGlobals(t)
	isolateTelemetryEnv(t)
	dir := t.TempDir()
	cfgFile = filepath.Join(dir, "agent.yaml")

	cfg := sidecarTelemetryBaseline()
	cfg.Telemetry.Enabled = true
	cfg.Telemetry.ServiceName = "sidecar-tel"
	cfg.Telemetry.EnableTracing = true
	cfg.Telemetry.EnableMetrics = true

	core, _, _, err := startAgentCore(context.Background(), cfg, dir)
	require.NoError(t, err)
	t.Cleanup(core.Stop)

	tel := core.TelemetryService()
	require.NotNil(t, tel, "master on must wire the telemetry service")
}
