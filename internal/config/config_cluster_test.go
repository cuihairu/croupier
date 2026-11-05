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

// CR-FIX-03（P0-3）fail-closed：集群启用但互联证书面缺失/跳过校验时拒绝启动
// 并明示原因，除非显式 allowPlaintextInterconnect=true（仅受信隔离内网）。
func TestValidateInterconnectTLS_ClusterDisabledOK(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("server:\n  port: 18780\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Cluster.ValidateInterconnectTLS(); err != nil {
		t.Fatalf("disabled cluster must validate: %v", err)
	}
}

func TestValidateInterconnectTLS_MissingCertsFailsClosed(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("cluster:\n  enabled: true\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	err := cfg.Cluster.ValidateInterconnectTLS()
	if err == nil {
		t.Fatal("cluster enabled + no cert surface must fail closed")
	}
	// 明示原因：含证书面提示。
	if !contains(err.Error(), "certificate surface") {
		t.Fatalf("must state certificate-surface reason, got: %v", err)
	}
}

func TestValidateInterconnectTLS_ExplicitPlaintextAllows(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("cluster:\n  enabled: true\n  allowPlaintextInterconnect: true\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Cluster.ValidateInterconnectTLS(); err != nil {
		t.Fatalf("explicit allowPlaintextInterconnect must permit plaintext: %v", err)
	}
}

func TestValidateInterconnectTLS_CompleteCertsOK(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("cluster:\n  enabled: true\n  interconnectTls:\n    certFile: /etc/croupier/ic.crt\n    keyFile: /etc/croupier/ic.key\n    caFile: /etc/croupier/ic-ca.crt\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Cluster.ValidateInterconnectTLS(); err != nil {
		t.Fatalf("complete cert surface must validate: %v", err)
	}
}

func TestValidateInterconnectTLS_InsecureSkipFailsClosed(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("cluster:\n  enabled: true\n  insecureSkipTls: true\n  interconnectTls:\n    certFile: /etc/croupier/ic.crt\n    keyFile: /etc/croupier/ic.key\n    caFile: /etc/croupier/ic-ca.crt\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Cluster.ValidateInterconnectTLS(); err == nil {
		t.Fatal("insecureSkipTls=true must fail closed without explicit plaintext confirmation")
	}
	if err := cfg.Cluster.ValidateInterconnectTLS(); err != nil && !contains(err.Error(), "allowPlaintextInterconnect") {
		t.Fatalf("must suggest the explicit confirmation, got: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
