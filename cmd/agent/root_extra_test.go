package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// saveRootGlobals 还原 runAgent/startAgentCore 测试触碰的旗标全局变量。
func saveRootGlobals(t *testing.T) {
	t.Helper()
	oldCfgFile, oldMode, oldDebug := cfgFile, mode, debug
	t.Cleanup(func() { cfgFile, mode, debug = oldCfgFile, oldMode, oldDebug })
}

func TestCollectSystemLabels(t *testing.T) {
	labels := collectSystemLabels()
	assert.Equal(t, "linux", labels["os"])
	assert.NotEmpty(t, labels["arch"])
	assert.NotEmpty(t, labels["hostname"])
	assert.NotEmpty(t, labels["cpu_count"])
	assert.Contains(t, labels["go_version"], "go")
	// 本机必有非回环 IPv4 或键缺失均可接受，但值不应是回环地址
	if ip, ok := labels["ip"]; ok {
		assert.NotEqual(t, "127.0.0.1", ip)
	}
}

func TestRunAgent_ErrorPaths(t *testing.T) {
	saveRootGlobals(t)
	dir := t.TempDir()

	// 缺配置路径
	cfgFile = ""
	assert.EqualError(t, runAgent(), "配置文件是必需的")

	// 读失败
	cfgFile = filepath.Join(dir, "missing.yaml")
	assert.ErrorContains(t, runAgent(), "failed to read config file")

	// 解析失败（yaml 非法）
	bad := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(bad, []byte("server: {unclosed"), 0o644))
	cfgFile = bad
	assert.ErrorContains(t, runAgent(), "failed to parse config")

	// 缺 agent.gameId/env（作用域必填）
	noScope := filepath.Join(dir, "noscope.yaml")
	require.NoError(t, os.WriteFile(noScope, []byte("server:\n  addr: \"\"\n"), 0o644))
	cfgFile = noScope
	assert.ErrorContains(t, runAgent(), "agent.gameId 和 agent.env 为必填")

	// startAgentCore 失败：非 Insecure + CA 缺失 + 证书目录只读（root 下跳过）
	if os.Getuid() != 0 {
		dir2 := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir2, "certs"), 0o555))
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir2, "certs"), 0o755) })
		bad := filepath.Join(dir2, "agent.yaml")
		require.NoError(t, os.WriteFile(bad, []byte(`
server:
  caFile: missing-ca.crt
agent:
  gameId: g
  env: e
`), 0o644))
		cfgFile = bad
		assert.ErrorContains(t, runAgent(), "failed to generate CA cert")
	}
}

// TestRunAgent_SuccessSignalsShutdown 走完整成功路径：本地网关起监听后向自身
// 发 SIGTERM，signal.NotifyContext 收口优雅退出（core.Stop 由 defer 执行）。
func TestRunAgent_SuccessSignalsShutdown(t *testing.T) {
	saveRootGlobals(t)
	dir := t.TempDir()
	const listenPort = "18931"

	cfgPath := filepath.Join(dir, "agent.yaml")
	cfgData := `
server:
  addr: ""
  insecure: true
agent:
  gameId: demo-game
  env: development
  localAddr: "127.0.0.1:` + listenPort + `"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgData), 0o644))
	cfgFile = cfgPath
	mode = "test"
	debug = true

	done := make(chan error, 1)
	go func() { done <- runAgent() }()

	// 监听就绪即证明信号处理器已注册（NotifyContext 先于 startAgentCore）
	deadline := time.Now().Add(10 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+listenPort, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("local gateway never came up: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// 优雅关闭：SIGTERM 被自身处理器接住，runAgent 返回 nil
	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("runAgent did not exit after SIGTERM")
	}
}

func TestStartAgentCore_NilConfig(t *testing.T) {
	_, _, err := startAgentCore(context.Background(), nil, "")
	assert.EqualError(t, err, "missing config")
}

// TestStartAgentCore_Wiring 覆盖地址推导与 TLS/Outbound/Ops 的接线分支；
// server addr 留空让 Run 不发起上游连接，用例结束即 Stop。
func TestStartAgentCore_Wiring(t *testing.T) {
	saveRootGlobals(t)
	dir := t.TempDir()
	cfgFile = filepath.Join(dir, "agent.yaml")

	cases := []struct {
		name         string
		cfg          AgentConfig
		wantLocal    string
		wantOpsOnOff bool
	}{
		{
			name: "localAddr 直配",
			cfg: AgentConfig{
				Server: AgentServerConfig{Insecure: true},
				Agent:  AgentInfoConfig{GameID: "g", Env: "e", LocalAddr: "127.0.0.1:0"},
			},
			wantLocal: "127.0.0.1:0",
		},
		{
			name: "tcp:// 前缀剥离",
			cfg: AgentConfig{
				Server: AgentServerConfig{Insecure: true},
				Agent:  AgentInfoConfig{GameID: "g", Env: "e", LocalAddr: "tcp://127.0.0.1:0"},
			},
			wantLocal: "127.0.0.1:0",
		},
		{
			name: "回落 host:port（0.0.0.0）",
			cfg: AgentConfig{
				Host:   "0.0.0.0",
				Port:   0,
				Server: AgentServerConfig{Insecure: true},
				Agent:  AgentInfoConfig{GameID: "g", Env: "e"},
			},
			wantLocal: "0.0.0.0:0",
		},
		{
			name: "回落具体 host",
			cfg: AgentConfig{
				Host:   "192.168.1.10",
				Port:   19999,
				Server: AgentServerConfig{Insecure: true},
				Agent:  AgentInfoConfig{GameID: "g", Env: "e"},
			},
			wantLocal: "192.168.1.10:19999",
		},
		{
			name: "TLS + OutboundTLS + Ops 全开",
			cfg: AgentConfig{
				Server: AgentServerConfig{
					Addr: "", Insecure: true, // Insecure=true → TLSConfig=nil 分支
					TLSCertFile: "/no/cert.pem", TLSKeyFile: "/no/key.pem",
				},
				Agent:       AgentInfoConfig{GameID: "g", Env: "e", LocalAddr: "127.0.0.1:0", InvokeTimeoutMs: 3000},
				OutboundTLS: AgentTLSConfig{Enabled: true, CertFile: "/no/o.pem"},
				Ops:         &OpsConfig{Enabled: true, MetricsEnabled: true, MetricsInterval: "5s"},
			},
			wantLocal:    "127.0.0.1:0",
			wantOpsOnOff: true,
		},
		{
			name: "Ops interval 非法 → 回落默认",
			cfg: AgentConfig{
				Server:      AgentServerConfig{Insecure: true},
				Agent:       AgentInfoConfig{GameID: "g", Env: "e", LocalAddr: "127.0.0.1:0", Labels: map[string]string{"rack": "a"}},
				OutboundTLS: AgentTLSConfig{}, // Enabled=false → nil 分支
				Ops:         &OpsConfig{Enabled: true, MetricsInterval: "bogus"},
			},
			wantLocal:    "127.0.0.1:0",
			wantOpsOnOff: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			core, localAddr, err := startAgentCore(context.Background(), &tc.cfg, dir)
			require.NoError(t, err)
			require.NotNil(t, core)
			assert.Equal(t, tc.wantLocal, localAddr)
			core.Stop()
		})
	}
}

// TestStartAgentCore_DevCertGeneration：非 Insecure + CA 缺失 → 自动生成 dev 证书；
// 证书目录不可写时生成失败报错（root 下权限模型不生效，跳过失败分支）。
func TestStartAgentCore_DevCertGeneration(t *testing.T) {
	saveRootGlobals(t)
	dir := t.TempDir()
	cfgFile = filepath.Join(dir, "agent.yaml")

	cfg := &AgentConfig{
		Server: AgentServerConfig{Insecure: false, CAFile: filepath.Join(dir, "no-such-ca.crt")},
		Agent:  AgentInfoConfig{GameID: "g", Env: "e", LocalAddr: "127.0.0.1:0"},
	}
	core, _, err := startAgentCore(context.Background(), cfg, dir)
	require.NoError(t, err)
	core.Stop()
	_, statErr := os.Stat(filepath.Join(dir, "certs", "ca.crt"))
	assert.NoError(t, statErr, "dev CA 应已生成")

	if os.Getuid() != 0 {
		// 独立配置目录 + 预置只读空证书目录 → EnsureDevCA 写入失败。
		// （首个用例的 certDir 已有证书，EnsureDevCA 直接复用不会触发写入。）
		dir2 := t.TempDir()
		cfgFile = filepath.Join(dir2, "agent.yaml")
		certDir2 := filepath.Join(dir2, "certs")
		require.NoError(t, os.MkdirAll(certDir2, 0o555))
		t.Cleanup(func() { _ = os.Chmod(certDir2, 0o755) })
		cfg2 := &AgentConfig{
			Server: AgentServerConfig{Insecure: false, CAFile: filepath.Join(dir2, "missing.crt")},
			Agent:  AgentInfoConfig{GameID: "g", Env: "e", LocalAddr: "127.0.0.1:0"},
		}
		_, _, err = startAgentCore(context.Background(), cfg2, dir2)
		assert.ErrorContains(t, err, "failed to generate CA cert")
	}
}

// ---- YAML 兼容解析矩阵：legacy PascalCase ↔ canonical 小驼峰 ----

func TestUnmarshalLegacyLogConfigCompat(t *testing.T) {
	// legacy Logging（PascalCase）→ toCommon（legacyAgentLogConfig.toCommon）
	legacy := []byte(`
agent:
  gameId: g
  env: e
Logging:
  Level: debug
  Format: json
  Output: file
  File: /tmp/a.log
  MaxSize: 10
  MaxBackups: 2
  MaxAge: 7
  Compress: true
`)
	var cfg AgentConfig
	require.NoError(t, yaml.Unmarshal(legacy, &cfg))
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "file", cfg.Logging.Output)
	assert.Equal(t, "/tmp/a.log", cfg.Logging.File)
	assert.Equal(t, 10, cfg.Logging.MaxSize)
	assert.Equal(t, 2, cfg.Logging.MaxBackups)
	assert.Equal(t, 7, cfg.Logging.MaxAge)
	assert.True(t, cfg.Logging.Compress)

	// canonical log（小驼峰）→ canonicalAgentLogConfig.toCommon。
	// 注意：common.LogConfig 无显式 yaml 键名（yaml.v3 全小写化），驼峰键
	// maxSize/maxBackups 只能经 canonical 分支接管——前提是 plain 解码全零。
	canon := []byte(`
agent:
  gameId: g
  env: e
log:
  maxSize: 5
  maxAge: 7
`)
	var cfg2 AgentConfig
	require.NoError(t, yaml.Unmarshal(canon, &cfg2))
	assert.Equal(t, 5, cfg2.Logging.MaxSize)
	assert.Equal(t, 7, cfg2.Logging.MaxAge)

	// 同时给 legacy + canonical → canonical 优先
	both := []byte(`
agent:
  gameId: g
  env: e
Logging:
  Level: debug
log:
  level: info
`)
	var cfg3 AgentConfig
	require.NoError(t, yaml.Unmarshal(both, &cfg3))
	assert.Equal(t, "info", cfg3.Logging.Level)
}

func TestUnmarshalLegacyTLSCompat(t *testing.T) {
	// legacy TLS 段（PascalCase 字段）走 AgentTLSConfig compat 分支
	legacy := []byte(`
agent:
  gameId: g
  env: e
TLS:
  Enabled: true
  CertFile: /c.pem
  KeyFile: /k.pem
  CAFile: /ca.pem
  ServerName: srv.local
  InsecureSkipVerify: true
`)
	var cfg AgentConfig
	require.NoError(t, yaml.Unmarshal(legacy, &cfg))
	assert.True(t, cfg.TLS.Enabled)
	assert.Equal(t, "/c.pem", cfg.TLS.CertFile)
	assert.Equal(t, "/k.pem", cfg.TLS.KeyFile)
	assert.Equal(t, "/ca.pem", cfg.TLS.CAFile)
	assert.Equal(t, "srv.local", cfg.TLS.ServerName)
	assert.True(t, cfg.TLS.InsecureSkipVerify)

	// legacy OutboundTLS 段同理
	outbound := []byte(`
agent:
  gameId: g
  env: e
OutboundTLS:
  Enabled: true
  CertFile: /oc.pem
`)
	var cfg2 AgentConfig
	require.NoError(t, yaml.Unmarshal(outbound, &cfg2))
	assert.True(t, cfg2.OutboundTLS.Enabled)
	assert.Equal(t, "/oc.pem", cfg2.OutboundTLS.CertFile)
}

func TestUnmarshalLegacyServerInfoUpstreamCompat(t *testing.T) {
	// Server/Agent 段 legacy 字段（PascalCase）走 compat 分支。
	// 注意：compat 是整段级别合并（decoded 段全零才整体替换），非字段级混排。
	data := []byte(`
Server:
  Addr: srv:19090
  Transport: tcp
  Insecure: true
  ServerName: sni
  InsecureSkipVerify: true
  TLSCertFile: /tc.pem
  TLSKeyFile: /tk.pem
  CAFile: /ca.pem
Agent:
  ID: fixed
  GameID: g2
  Env: prod
  Transport: tcp
  LocalAddr: 127.0.0.1:1
  HTTPAddr: 127.0.0.1:2
  Labels:
    rack: a
`)
	var cfg AgentConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, "srv:19090", cfg.Server.Addr)
	assert.Equal(t, "tcp", cfg.Server.Transport)
	assert.True(t, cfg.Server.Insecure)
	assert.Equal(t, "sni", cfg.Server.ServerName)
	assert.True(t, cfg.Server.InsecureSkipVerify)
	assert.Equal(t, "/tc.pem", cfg.Server.TLSCertFile)
	assert.Equal(t, "/tk.pem", cfg.Server.TLSKeyFile)
	assert.Equal(t, "/ca.pem", cfg.Server.CAFile)
	assert.Equal(t, "fixed", cfg.Agent.ID)
	assert.Equal(t, "g2", cfg.Agent.GameID)
	assert.Equal(t, "prod", cfg.Agent.Env)
	assert.Equal(t, "tcp", cfg.Agent.Transport)
	assert.Equal(t, "127.0.0.1:1", cfg.Agent.LocalAddr)
	assert.Equal(t, "127.0.0.1:2", cfg.Agent.HTTPAddr)
	assert.Equal(t, map[string]string{"rack": "a"}, cfg.Agent.Labels)

	// Upstream legacy int 指针 compat
	up := []byte(`
agent:
  gameId: g
  env: e
Upstream:
  HeartbeatInterval: 11
  RetryInterval: 12
  MaxRetries: 13
  Timeout: 14
`)
	var cfg2 AgentConfig
	require.NoError(t, yaml.Unmarshal(up, &cfg2))
	assert.Equal(t, 11, cfg2.Upstream.HeartbeatInterval)
	assert.Equal(t, 12, cfg2.Upstream.RetryInterval)
	assert.Equal(t, 13, cfg2.Upstream.MaxRetries)
	assert.Equal(t, 14, cfg2.Upstream.Timeout)
}

func TestUnmarshalTopLevelLegacyCompat(t *testing.T) {
	// 顶层 Name/Host/Port 的 legacy 透传
	data := []byte(`
Name: top-name
Host: 10.0.0.1
Port: 12345
agent:
  gameId: g
  env: e
`)
	var cfg AgentConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, "top-name", cfg.Name)
	assert.Equal(t, "10.0.0.1", cfg.Host)
	assert.Equal(t, 12345, cfg.Port)
}

func TestUnmarshalDecodeErrors(t *testing.T) {
	// plain decode 失败：Port 类型不合法
	var cfg AgentConfig
	assert.Error(t, yaml.Unmarshal([]byte("Port: [not-a-number]"), &cfg))

	// canonical decode 失败：log.maxSize 类型不合法
	var cfg2 AgentConfig
	assert.Error(t, yaml.Unmarshal([]byte("log:\n  maxSize: {bad: mapping}"), &cfg2))

	// compat decode 失败：Logging.MaxSize 类型不合法
	var cfg3 AgentConfig
	assert.Error(t, yaml.Unmarshal([]byte("Logging:\n  MaxSize: [1,2]"), &cfg3))

	// Server 段 plain decode 失败
	var cfg4 AgentConfig
	assert.Error(t, yaml.Unmarshal([]byte("Server: 42"), &cfg4))
}

func TestApplyAgentConfigDefaults_Nil(t *testing.T) {
	// nil 入口直接返回（防御分支）
	applyAgentConfigDefaults(nil)
}

// TestRootCommandRunEAliases 直调根命令与 agent 别名的 RunE 闭包（cfgFile 为空报错）。
func TestRootCommandRunEAliases(t *testing.T) {
	saveRootGlobals(t)
	cfgFile = ""
	assert.EqualError(t, rootCmd.RunE(rootCmd, nil), "配置文件是必需的")

	cmd, _, err := rootCmd.Find([]string{"agent"})
	require.NoError(t, err)
	require.NotNil(t, cmd.RunE)
	assert.EqualError(t, cmd.RunE(cmd, nil), "配置文件是必需的")

	cmd, _, err = rootCmd.Find([]string{"version"})
	require.NoError(t, err)
	require.NotNil(t, cmd.Run)
	cmd.Run(cmd, nil) // 仅验证不 panic（PrintVersionInfo 单测覆盖输出）
}

// TestUnmarshalCompatDecodeErrors 覆盖 compat 解析错误分支：
// yaml.v3 精确键匹配，PascalCase/snake 专属键 plain 视而不见、compat 撞类型错。
func TestUnmarshalCompatDecodeErrors(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"Ops plain 标量", "ops: 42"},
		{"Ops compat 键类型错", "ops:\n  metrics_enabled: [x]"},
		{"TLS plain 标量", "TLS: 42"},
		{"TLS compat 键类型错", "TLS:\n  InsecureSkipVerify: [x]"},
		{"顶层 plain 键类型错", "port: [x]"},
		{"Server compat 键类型错", "Server:\n  InsecureSkipVerify: [x]"},
		{"Agent plain 键类型错", "agent:\n  gameId: [x]"},
		{"Agent compat 键类型错", "Agent:\n  GameID: [x]"},
		{"Upstream plain 键类型错", "Upstream:\n  heartbeatInterval: [x]"},
		{"Upstream compat 键类型错", "Upstream:\n  HeartbeatInterval: [x]"},
	}
	for _, tc := range cases {
		var cfg AgentConfig
		assert.Error(t, yaml.Unmarshal([]byte(tc.data), &cfg), tc.name)
	}
}

// TestUnmarshalCanonicalTLSOverride canonical tls/outboundTLS 段接管（plain 内联全零时）。
func TestUnmarshalCanonicalTLSOverride(t *testing.T) {
	data := []byte(`
agent:
  gameId: g
  env: e
tls:
  enabled: true
  certFile: /c.pem
outboundTLS:
  enabled: true
  caFile: /ca.pem
`)
	var cfg AgentConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.True(t, cfg.TLS.Enabled)
	assert.Equal(t, "/c.pem", cfg.TLS.CertFile)
	assert.True(t, cfg.OutboundTLS.Enabled)
	assert.Equal(t, "/ca.pem", cfg.OutboundTLS.CAFile)
}
