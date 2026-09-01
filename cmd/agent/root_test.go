package main

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
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
