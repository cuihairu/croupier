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
  interconnectTls:
    enabled: true
    certFile: /etc/croupier/ic.crt
    keyFile: /etc/croupier/ic.key
    caFile: /etc/croupier/ic-ca.crt
    serverName: cluster.internal
    insecureSkipVerify: false
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
	if !c.InterconnectTLS.EffectiveEnabled() || c.InterconnectTLS.CertFile != "/etc/croupier/ic.crt" ||
		c.InterconnectTLS.KeyFile != "/etc/croupier/ic.key" || c.InterconnectTLS.CAFile != "/etc/croupier/ic-ca.crt" ||
		c.InterconnectTLS.ServerName != "cluster.internal" {
		t.Fatalf("interconnectTLS mismatch: %+v", c.InterconnectTLS)
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

// 审核发现 709：互联 TLS 曾需显式 enabled=true 才启用——证书齐备但未配
// enabled 的存量部署会静默降级为明文互联。修复后证书面齐备默认启用
// （ARCH-003 设计语义），显式 enabled:false 才可关。
func TestUnmarshalClusterConfig_TLSEnabledByDefaultWhenCertsPresent(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(`
cluster:
  enabled: true
  interconnectTls:
    certFile: /etc/croupier/ic.crt
    keyFile: /etc/croupier/ic.key
    caFile: /etc/croupier/ic-ca.crt
`), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if !cfg.Cluster.InterconnectTLS.EffectiveEnabled() {
		t.Fatal("certs present without explicit enabled must default to mTLS enabled")
	}
}

func TestUnmarshalClusterConfig_TLSExplicitDisabledStaysOff(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(`
cluster:
  enabled: true
  interconnectTls:
    enabled: false
    certFile: /etc/croupier/ic.crt
    keyFile: /etc/croupier/ic.key
    caFile: /etc/croupier/ic-ca.crt
`), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if cfg.Cluster.InterconnectTLS.EffectiveEnabled() {
		t.Fatal("explicit enabled:false must keep TLS off")
	}
}
