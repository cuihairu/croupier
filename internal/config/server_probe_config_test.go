package config

// server `probe:` 段解析：与 agent probe.targets 同契约（core/healthprobe
// TargetConfig，键小驼峰）；空段零值不装配。

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestServerProbeConfig_UnmarshalYAML_Canonical(t *testing.T) {
	input := `
probe:
  maxDetailsPerWindow: 5
  targets:
    - name: billing-api
      type: http
      url: http://billing.internal:8080/healthz
      timeout: 5s
      interval: 30s
    - name: meta-db
      type: tcp
      address: 127.0.0.1:5432
      failThreshold: 2
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Probe.Targets) != 2 {
		t.Fatalf("targets = %d, want 2", len(cfg.Probe.Targets))
	}
	first := cfg.Probe.Targets[0]
	if first.Name != "billing-api" || first.Type != "http" {
		t.Fatalf("first = %+v", first)
	}
	if first.Timeout != 5*time.Second || first.Interval != 30*time.Second {
		t.Fatalf("durations = %v/%v, want 5s/30s", first.Timeout, first.Interval)
	}
	second := cfg.Probe.Targets[1]
	if second.Address != "127.0.0.1:5432" || second.FailThreshold != 2 {
		t.Fatalf("second = %+v", second)
	}
	if cfg.Probe.MaxDetailsPerWindow != 5 {
		t.Fatalf("maxDetailsPerWindow = %d, want 5", cfg.Probe.MaxDetailsPerWindow)
	}
}

func TestServerProbeConfig_EmptySectionKeepsZeroValue(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("server:\n  port: 18780\n"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Probe.Targets != nil || cfg.Probe.MaxDetailsPerWindow != 0 {
		t.Fatalf("probe = %+v, want zero value", cfg.Probe)
	}
}
