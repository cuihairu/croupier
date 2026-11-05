package main

import (
	"testing"

	"github.com/cuihairu/croupier/internal/telemetry"
)

// buildAgentRecorderConfig 映射（OBS-AGENT-001）：复用既有 telemetry 块总开关
// 与采集端；enable_metrics 关时不打点；use_tls 决定 exporter 传输。
func TestBuildAgentRecorderConfig_MapsTelemetryBlock(t *testing.T) {
	cases := []struct {
		name   string
		in     telemetry.TelemetryConfig
		expect struct {
			enabled  bool
			endpoint string
			insecure bool
			service  string
		}
	}{
		{
			name: "disabled defaults to off",
			in:   telemetry.TelemetryConfig{},
			expect: struct {
				enabled  bool
				endpoint string
				insecure bool
				service  string
			}{enabled: false, insecure: true, service: "croupier-agent"},
		},
		{
			name: "enabled with metrics and url",
			in: telemetry.TelemetryConfig{
				Enabled: true, EnableMetrics: true, CollectorURL: "http://collector:4318",
				UseTLS: false, ServiceName: "croupier-sidecar",
			},
			expect: struct {
				enabled  bool
				endpoint string
				insecure bool
				service  string
			}{enabled: true, endpoint: "http://collector:4318", insecure: true, service: "croupier-sidecar"},
		},
		{
			name: "use_tls clears insecure",
			in: telemetry.TelemetryConfig{
				Enabled: true, EnableMetrics: true, CollectorURL: "https://collector:4318", UseTLS: true,
			},
			expect: struct {
				enabled  bool
				endpoint string
				insecure bool
				service  string
			}{enabled: true, endpoint: "https://collector:4318", insecure: false, service: "croupier-agent"},
		},
		{
			name: "metrics off keeps recorder off",
			in: telemetry.TelemetryConfig{
				Enabled: true, EnableMetrics: false, CollectorURL: "http://collector:4318",
			},
			expect: struct {
				enabled  bool
				endpoint string
				insecure bool
				service  string
			}{enabled: false, endpoint: "http://collector:4318", insecure: true, service: "croupier-agent"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildAgentRecorderConfig(tc.in)
			if got.Enabled != tc.expect.enabled || got.OTLPEndpoint != tc.expect.endpoint ||
				got.Insecure != tc.expect.insecure || got.ServiceName != tc.expect.service {
				t.Fatalf("got %+v", got)
			}
		})
	}
}
