package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestUnmarshalClusterConfig_CanonicalKeys(t *testing.T) {
	input := `
cluster:
  enabled: true
  instanceId: server-a
  advertiseAddr: "10.0.1.11:8444"
  interconnectAddr: "10.0.1.11:8445"
  heartbeatInterval: 5s
  leaseTtl: 15s
  ownerTtl: 3m
  peerPollInterval: 10s
  insecureSkipTls: false
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	c := cfg.Cluster
	if !c.Enabled || c.InstanceID != "server-a" || c.AdvertiseAddr != "10.0.1.11:8444" {
		t.Fatalf("base fields mismatch: %+v", c)
	}
	if c.InterconnectAddr != "10.0.1.11:8445" || c.HeartbeatInterval != "5s" || c.LeaseTTL != "15s" {
		t.Fatalf("detail fields mismatch: %+v", c)
	}
	if c.OwnerTTL != "3m" || c.PeerPollInterval != "10s" || c.InsecureSkipTLS {
		t.Fatalf("extra fields mismatch: %+v", c)
	}
}

func TestUnmarshalClusterConfig_DisabledByDefault(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("server:\n  port: 18780\n"), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if cfg.Cluster.Enabled {
		t.Fatal("cluster must be disabled by default")
	}
}
