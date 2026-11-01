package metrics

import (
	"context"
	"testing"
	"time"
)

// TestCounterRate covers the delta-per-second rate derivation, including the
// counter-reset (interface recreated / agent reboot) guard.
func TestCounterRate(t *testing.T) {
	cases := []struct {
		name      string
		cur, prev uint64
		dt        float64
		want      float64
	}{
		{"normal", 1000, 500, 5, 100},
		{"zero dt", 1000, 500, 0, 0},
		{"negative dt", 1000, 500, -1, 0},
		{"counter reset backwards", 100, 500, 5, 0},
		{"no movement", 500, 500, 5, 0},
		{"fractional", 101, 100, 2, 0.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := counterRate(tc.cur, tc.prev, tc.dt)
			if got != tc.want {
				t.Fatalf("counterRate(%d,%d,%v) = %v, want %v", tc.cur, tc.prev, tc.dt, got, tc.want)
			}
		})
	}
}

// TestCollector_CollectProcessAndPortFaces verifies the OPS-010 process and
// port faces ride the report. The environment may lack process enumeration
// (restricted container) — in that case both faces must be empty, never
// fabricated. When present, every process must carry a valid pid and the
// port face must only contain known protocols.
func TestCollector_CollectProcessAndPortFaces(t *testing.T) {
	c := NewCollector("test-agent")
	report := c.Collect(context.Background())

	// Faces must be non-nil (present even when empty — "no data" vs missing).
	if report.GetProcesses() == nil {
		t.Fatal("processes face is nil, want non-nil slice")
	}
	if report.GetPorts() == nil {
		t.Fatal("ports face is nil, want non-nil slice")
	}

	validProto := map[string]bool{"tcp": true, "tcp6": true, "udp": true, "udp6": true}
	for _, p := range report.GetPorts() {
		if p.GetPort() <= 0 {
			t.Fatalf("port face has invalid port %d", p.GetPort())
		}
		if !validProto[p.GetProtocol()] {
			t.Fatalf("port face has unknown protocol %q", p.GetProtocol())
		}
	}
	for _, proc := range report.GetProcesses() {
		if proc.GetPid() <= 0 {
			t.Fatalf("process face has invalid pid %d", proc.GetPid())
		}
	}
}

// TestCollector_NetworkRateNonNegative verifies network rate fields are
// present and never negative on a real scan (first scan may be 0 — no prior
// sample; a second scan yields a delta ≥ 0).
func TestCollector_NetworkRateNonNegative(t *testing.T) {
	c := NewCollector("test-agent")
	first := c.Collect(context.Background())
	for _, n := range first.GetNetworks() {
		if n.GetBytesSentRate() < 0 || n.GetBytesRecvRate() < 0 {
			t.Fatalf("first scan network rate negative on %s", n.GetInterface())
		}
	}
	// Force a second scan with a small sleep so a counter delta can register.
	time.Sleep(50 * time.Millisecond)
	second := c.Collect(context.Background())
	for _, n := range second.GetNetworks() {
		if n.GetBytesSentRate() < 0 || n.GetBytesRecvRate() < 0 {
			t.Fatalf("second scan network rate negative on %s", n.GetInterface())
		}
	}
}
