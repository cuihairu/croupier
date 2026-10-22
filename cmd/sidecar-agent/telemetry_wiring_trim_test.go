//go:build no_otel

package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// no_otel 裁剪构建 + 总开配置 → sidecar 启动 fail-closed 报错
// （与 no_go_plugin 插件门禁同策略，todo.md:31），报错指明 build tag。
func TestStartAgentCore_TelemetryTrimBuildFailsClosed(t *testing.T) {
	saveRootGlobals(t)
	isolateTelemetryEnv(t)
	dir := t.TempDir()
	cfgFile = filepath.Join(dir, "agent.yaml")

	cfg := sidecarTelemetryBaseline()
	cfg.Telemetry.Enabled = true

	_, _, _, err := startAgentCore(context.Background(), cfg, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no_otel")
	assert.Contains(t, err.Error(), "built without OTel support")
}

// no_otel 裁剪构建 + 关闭配置 → 正常启动且不接线遥测服务。
func TestStartAgentCore_TelemetryTrimBuildMasterOffRunsLightweight(t *testing.T) {
	saveRootGlobals(t)
	isolateTelemetryEnv(t)
	dir := t.TempDir()
	cfgFile = filepath.Join(dir, "agent.yaml")

	core, _, _, err := startAgentCore(context.Background(), sidecarTelemetryBaseline(), dir)
	require.NoError(t, err)
	t.Cleanup(core.Stop)
	assert.Nil(t, core.TelemetryService())
}
