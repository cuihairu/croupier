package agent

// 受管对象扩 kind（agent-supervisor-design §10.1/§10.8）用例：一个受管对象
// 同时表示游戏辅助程序与外部探测适配器（kind: program | probe-adapter），
// 两类共用同一套生命周期；kind 缺省=program（存量配置零漂移），未知 kind
// 配置加载即失败（闭集，对齐 snapshotProfile 先例），kind=probe-adapter
// 必须有适配器身份（探测面引用与授权校验的最小锚点）。

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestManagedProcessKindDefaultsToProgram(t *testing.T) {
	// 缺省 kind=空 → program（存量配置行为零漂移）。
	var empty ManagedProcessConfig
	assert.Equal(t, "program", empty.EffectiveKind())

	explicit := ManagedProcessConfig{Kind: "program"}
	assert.Equal(t, "program", explicit.EffectiveKind())

	adapter := ManagedProcessConfig{Kind: "probe-adapter", AdapterID: "com.example.game-a"}
	assert.Equal(t, "probe-adapter", adapter.EffectiveKind())
	assert.Equal(t, "croupier-probe-jsonl-v1", adapter.EffectiveAdapterProtocol(),
		"协议缺省走 croupier-probe-jsonl-v1 契约名")
}

func TestValidateRejectsUnknownKind(t *testing.T) {
	cases := map[string]ManagedProcessConfig{
		"empty":     {Kind: ""},
		"program":   {Kind: "program"},
		"adapter":   {Kind: "probe-adapter", AdapterID: "com.example.game-a"},
		"bogus":     {Kind: "probe_plugin"},
		"wrongcase": {Kind: "Probe-Adapter", AdapterID: "x"},
	}
	for name, cfg := range cases {
		err := cfg.Validate()
		switch name {
		case "bogus", "wrongcase":
			require.Error(t, err, "%s", name)
			assert.Contains(t, err.Error(), "kind")
		default:
			assert.NoError(t, err, "%s", name)
		}
	}
}

func TestValidateProbeAdapterRequiresIdentity(t *testing.T) {
	// 适配器身份是探测面引用与授权校验的最小锚点——缺身份不许声明。
	missing := ManagedProcessConfig{Kind: "probe-adapter"}
	err := missing.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "adapterId")

	blank := ManagedProcessConfig{Kind: "probe-adapter", AdapterID: "  "}
	require.Error(t, blank.Validate())

	ok := ManagedProcessConfig{Kind: "probe-adapter", AdapterID: "com.example.game-a", AdapterVersion: "1.0.0"}
	assert.NoError(t, ok.Validate())

	// program kind 不要求适配器身份。
	program := ManagedProcessConfig{Kind: "program"}
	assert.NoError(t, program.Validate())
}

func TestManagedProcessKindYAMLRoundTrip(t *testing.T) {
	// 配置契约：键一律 lowerCamelCase。
	raw := []byte(`
kind: probe-adapter
command: ./bin/game-a-probe
autoRestart: true
adapterId: com.example.game-a
adapterVersion: 1.0.0
adapterProtocol: croupier-probe-jsonl-v1
adapterCapabilities: [login, handshake]
`)
	var cfg ManagedProcessConfig
	require.NoError(t, yaml.Unmarshal(raw, &cfg))
	require.Equal(t, "probe-adapter", cfg.EffectiveKind())
	assert.Equal(t, "com.example.game-a", cfg.AdapterID)
	assert.Equal(t, "1.0.0", cfg.AdapterVersion)
	assert.Equal(t, "croupier-probe-jsonl-v1", cfg.AdapterProtocol)
	assert.ElementsMatch(t, []string{"login", "handshake"}, cfg.AdapterCapabilities)

	// 反序列化回来的键名须是 canonical（lowerCamelCase），不许漂移。
	out, err := yaml.Marshal(map[string]ManagedProcessConfig{"game-a": cfg})
	require.NoError(t, err)
	for _, key := range []string{"kind:", "adapterId:", "adapterVersion:", "adapterProtocol:", "adapterCapabilities:"} {
		assert.Contains(t, string(out), key)
	}
}

func TestManagedProcessKindSurvivesBootstrap(t *testing.T) {
	// kind=probe-adapter 走同一套生命周期：开机自启、快照条目可见 kind。
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "sleeper")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	bin := filepath.Join(dir, "sleeper")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\nsleep 30\n"), 0o755))

	s := NewOpsServer(&OpsConfig{
		Enabled: true,
		ManagedProcesses: map[string]ManagedProcessConfig{
			"sidecar-app":    {Command: bin, AutoRestart: true, SnapshotProfile: "go"},
			"game-a-adapter": {Command: bin, AutoRestart: true, Kind: "probe-adapter", AdapterID: "com.example.game-a", AdapterVersion: "1.0.0"},
		},
	}, "agent-a", "test", nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	snaps := s.SampleSupervisedProcesses()
	require.Len(t, snaps, 2)
	byName := map[string]string{}
	for _, sp := range snaps {
		byName[sp.GetName()] = sp.GetKind()
	}
	assert.Equal(t, "program", byName["sidecar-app"], "缺省 kind 面板呈现为 program")
	assert.Equal(t, "probe-adapter", byName["game-a-adapter"])
}
