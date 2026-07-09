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
	if cfg.Agent.LocalAddr != "agent:19091" {
		t.Fatalf("localAddr = %q, want %q", cfg.Agent.LocalAddr, "agent:19091")
	}
}
