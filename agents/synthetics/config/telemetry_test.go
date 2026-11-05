// Telemetry block parsing (OBS-AGENT-001). No build tag: telemetry is not
// plugin-gated, so it runs in both default and no_go_plugin builds.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTelemetryDefaultsOffAndBlockParses(t *testing.T) {
	// 缺省：telemetry.enabled=false（总开关收口，零开销 noop 路径）。
	if valid().Telemetry.Enabled {
		t.Fatal("telemetry.enabled must default to false")
	}

	p := filepath.Join(t.TempDir(), "syn.yaml")
	body := []byte(`
agent:
  id: a
  serverAddr: s
  gameId: g
  env: e
synthetics:
  interval: 1s
  targets:
    - name: t
      url: http://127.0.0.1
      method: GET
      timeout: 1s
      expectedStatus: 200
      targetCategory: game
telemetry:
  enabled: true
  otlpEndpoint: http://collector:4318
  insecure: true
  serviceName: croupier-synthetics
`)
	if err := os.WriteFile(p, body, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.Telemetry.Enabled || cfg.Telemetry.OTLPEndpoint != "http://collector:4318" ||
		!cfg.Telemetry.Insecure || cfg.Telemetry.ServiceName != "croupier-synthetics" {
		t.Fatalf("telemetry = %+v", cfg.Telemetry)
	}
}
