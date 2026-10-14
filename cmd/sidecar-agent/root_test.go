package main

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cuihairu/croupier/core/healthprobe"
)

func TestAgentConfigUnmarshalUppercaseSections(t *testing.T) {
	data := []byte(`
Name: croupier-agent
Host: 0.0.0.0
Port: 19091

Server:
  Addr: "server:19090"
  Insecure: true

Agent:
  LocalAddr: "agent:19090"
  HTTPAddr: "agent:19091"

Upstream:
  HeartbeatInterval: 30
  RetryInterval: 5
  MaxRetries: 3
  Timeout: 10000
`)

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if cfg.Server.Addr != "server:19090" {
		t.Fatalf("server addr = %q, want %q", cfg.Server.Addr, "server:19090")
	}
	if cfg.Agent.LocalAddr != "agent:19090" {
		t.Fatalf("local addr = %q, want %q", cfg.Agent.LocalAddr, "agent:19090")
	}
	if cfg.Agent.HTTPAddr != "agent:19091" {
		t.Fatalf("http addr = %q, want %q", cfg.Agent.HTTPAddr, "agent:19091")
	}
	if cfg.Upstream.HeartbeatInterval != 30 {
		t.Fatalf("heartbeat interval = %d, want %d", cfg.Upstream.HeartbeatInterval, 30)
	}
}

func TestResolveAgentID(t *testing.T) {
	if got := resolveAgentID("fixed-agent"); got != "fixed-agent" {
		t.Fatalf("resolveAgentID() = %q, want %q", got, "fixed-agent")
	}

	if got := resolveAgentID(""); got == "" {
		t.Fatal("resolveAgentID() returned empty id for blank config")
	}
}

func TestAgentConfigDefaultsToTLSServerAndPlainGateway(t *testing.T) {
	data := []byte(`
name: croupier-agent
server:
  addr: "server:19090"
agent:
  localAddr: "127.0.0.1:19091"
`)

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if cfg.Server.Insecure {
		t.Fatal("server insecure = true, want false by default")
	}
	if cfg.TLS.Enabled {
		t.Fatal("local gateway tls = true, want false by default")
	}
}

func TestBundledDevConfigsUsePlainControlWhenServerHasNoTLS(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"../../configs/agent.yaml",
		"../../configs/agent.local.yaml",
		"../../docker/configs/agent.yaml",
	} {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read config: %v", err)
			}

			var cfg AgentConfig
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("unmarshal config: %v", err)
			}
			if !cfg.Server.Insecure {
				t.Fatal("expected bundled dev agent config to use plain control connection")
			}
		})
	}
}

// 示例配置里的 probe 段是文档的一部分：整段默认注释掉（不启用探测），但
// 注释里的内容必须真能解析、真能装配——示例写错等于教用户写错。
func TestBundledAgentConfigProbeExampleIsValidButDisabled(t *testing.T) {
	data, err := os.ReadFile("../../configs/agent.yaml")
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	// 1) 默认不带探测：注释块不得被解析成启用状态。
	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg.Probe != nil {
		t.Fatalf("bundled config must ship with probes disabled, got %+v", cfg.Probe)
	}

	// 2) 取消注释后必须可解析且装配通过。
	uncommented := uncommentProbeBlock(string(data))
	if uncommented == "" {
		t.Fatal("agent.yaml should document the probe section as a commented example")
	}
	var probeCfg struct {
		Probe *ProbeConfig `yaml:"probe"`
	}
	if err := yaml.Unmarshal([]byte(uncommented), &probeCfg); err != nil {
		t.Fatalf("unmarshal uncommented probe example: %v", err)
	}
	if probeCfg.Probe == nil || len(probeCfg.Probe.Targets) == 0 {
		t.Fatal("uncommented probe example yielded no targets")
	}
	if _, err := healthprobe.NewManager(probeCfg.Probe.Targets, "demo", "prod", nil, nil); err != nil {
		t.Fatalf("documented probe example must assemble: %v", err)
	}
}

// uncommentProbeBlock 取出 agent.yaml 中以 "# probe:" 起始的注释块并去注释。
func uncommentProbeBlock(doc string) string {
	var out []string
	started := false
	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		if !started {
			if trimmed == "# probe:" {
				started = true
				out = append(out, "probe:")
			}
			continue
		}
		// 注释块到下一个非注释非空行或下一个顶级注释标题为止。
		if trimmed == "" || !strings.HasPrefix(trimmed, "#") {
			continue
		}
		out = append(out, strings.TrimPrefix(strings.TrimPrefix(trimmed, "#"), " "))
	}
	if !started {
		return ""
	}
	// db 目标示例含明文口令占位，不参与装配校验：整条目标截掉。
	for i, line := range out {
		if strings.HasPrefix(strings.TrimSpace(line), "- name: game-db") {
			return strings.Join(out[:i], "\n") + "\n"
		}
	}
	return strings.Join(out, "\n") + "\n"
}

func TestBundledDockerConfigUsesDedicatedLocalGatewayPort(t *testing.T) {
	data, err := os.ReadFile("../../docker/configs/agent.yaml")
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	// localAddr 是监听地址：绑定 0.0.0.0 保证容器 healthcheck（nc 127.0.0.1）可达；
	// 对外通告地址由 httpAddr（agent:19091）承担。绑定容器名会导致 listener 只
	// 监听容器 IP，localhost 探测失败（部署验收中实际踩过）。
	if cfg.Agent.LocalAddr != "0.0.0.0:19091" {
		t.Fatalf("localAddr = %q, want %q", cfg.Agent.LocalAddr, "0.0.0.0:19091")
	}
	if cfg.Agent.HTTPAddr != "agent:19091" {
		t.Fatalf("httpAddr = %q, want %q", cfg.Agent.HTTPAddr, "agent:19091")
	}
}

// Ops 键命名契约：canonical 小驼峰 + 旧 snake_case 仅解析兼容。
func TestAgentConfigOpsMetricsKeys(t *testing.T) {
	canon := []byte(`
ops:
  enabled: true
  metricsInterval: 30s
  metricsEnabled: true
`)
	var cfg AgentConfig
	if err := yaml.Unmarshal(canon, &cfg); err != nil {
		t.Fatalf("unmarshal canonical: %v", err)
	}
	if cfg.Ops == nil || !cfg.Ops.MetricsEnabled || cfg.Ops.MetricsInterval != "30s" {
		t.Fatalf("canonical ops keys not parsed: %+v", cfg.Ops)
	}

	legacy := []byte(`
ops:
  enabled: true
  metrics_interval: 45s
  metrics_enabled: false
`)
	var cfg2 AgentConfig
	if err := yaml.Unmarshal(legacy, &cfg2); err != nil {
		t.Fatalf("unmarshal legacy: %v", err)
	}
	if cfg2.Ops == nil || cfg2.Ops.MetricsInterval != "45s" || cfg2.Ops.MetricsEnabled {
		t.Fatalf("legacy ops keys not compat-parsed: %+v", cfg2.Ops)
	}
}

// 声明式探针配置面（§12 R2）：`probe.targets[]` 小驼峰键直接落到
// core/healthprobe.TargetConfig（同一套契约，不在 CLI 侧另造结构体）。
func TestAgentConfigProbeTargetsCanonicalKeys(t *testing.T) {
	data := []byte(`
probe:
  maxDetailsPerWindow: 5
  targets:
    - name: redis
      type: tcp
      address: 127.0.0.1:6379
      semantics: readiness
      interval: 15s
      failThreshold: 2
      recoverThreshold: 1
    - name: gateway
      type: http
      url: http://127.0.0.1:8080/healthz
      timeout: 3s
`)
	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal probe section: %v", err)
	}
	if cfg.Probe == nil {
		t.Fatal("probe section not parsed")
	}
	if cfg.Probe.MaxDetailsPerWindow != 5 {
		t.Fatalf("maxDetailsPerWindow = %d, want 5", cfg.Probe.MaxDetailsPerWindow)
	}
	if len(cfg.Probe.Targets) != 2 {
		t.Fatalf("targets = %d, want 2", len(cfg.Probe.Targets))
	}
	first := cfg.Probe.Targets[0]
	if first.Name != "redis" || first.Type != "tcp" || first.Address != "127.0.0.1:6379" {
		t.Fatalf("tcp target not parsed: %+v", first)
	}
	if first.Interval.String() != "15s" || first.FailThreshold != 2 || first.RecoverThreshold != 1 {
		t.Fatalf("thresholds/interval not parsed: %+v", first)
	}
	if first.Semantics != healthprobe.SemanticsReadiness {
		t.Fatalf("semantics = %q", first.Semantics)
	}
	second := cfg.Probe.Targets[1]
	if second.URL != "http://127.0.0.1:8080/healthz" || second.Timeout.String() != "3s" {
		t.Fatalf("http target not parsed: %+v", second)
	}
}

// canonical 顶层 `tls:` / `outboundTLS:` 键直接由 AgentConfig 内联字段
// （yaml:"tls"/"outboundTLS"）捕获；TLS 不设 canonical 二次视图（键集重叠、
// 冗余分支已删），这里锁定 canonical 键到 TLS/OutboundTLS 的解析路径。
func TestAgentConfigUnmarshalCanonicalTLS(t *testing.T) {
	data := []byte(`
Name: croupier-agent
tls:
  enabled: true
  certFile: /etc/croupier/tls/agent.crt
  keyFile: /etc/croupier/tls/agent.key
  serverName: croupier.local
outboundTLS:
  enabled: true
  caFile: /etc/croupier/tls/ca.crt
  insecureSkipVerify: true
`)
	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal canonical tls: %v", err)
	}
	if !cfg.TLS.Enabled || cfg.TLS.CertFile != "/etc/croupier/tls/agent.crt" ||
		cfg.TLS.ServerName != "croupier.local" {
		t.Fatalf("canonical tls not parsed: %+v", cfg.TLS)
	}
	if !cfg.OutboundTLS.Enabled || cfg.OutboundTLS.CAFile != "/etc/croupier/tls/ca.crt" ||
		!cfg.OutboundTLS.InsecureSkipVerify {
		t.Fatalf("canonical outboundTLS not parsed: %+v", cfg.OutboundTLS)
	}
}
