//go:build !no_go_plugin

package main

import (
	"regexp"
	"testing"
)

// C-016: the probe credential must never be a fixed literal in the binary.
// resolveProbeSecret honours an injected HOSTDRIVER_PROBE_SECRET and otherwise
// generates a per-run random value.
func TestResolveProbeSecret_Injected(t *testing.T) {
	t.Setenv("HOSTDRIVER_PROBE_SECRET", "injected-secret-123")
	got, err := resolveProbeSecret()
	if err != nil {
		t.Fatalf("resolveProbeSecret: %v", err)
	}
	if got != "injected-secret-123" {
		t.Fatalf("expected injected value, got %q", got)
	}
}

func TestResolveProbeSecret_RandomWhenUnset(t *testing.T) {
	t.Setenv("HOSTDRIVER_PROBE_SECRET", "")
	first, err := resolveProbeSecret()
	if err != nil {
		t.Fatalf("resolveProbeSecret: %v", err)
	}
	second, err := resolveProbeSecret()
	if err != nil {
		t.Fatalf("resolveProbeSecret: %v", err)
	}
	if first == "" || second == "" {
		t.Fatal("random secret must not be empty")
	}
	if first == second {
		t.Fatal("two unset calls must not collide (random per run)")
	}
	if first == "hd-t0k3n-9f8e7d" {
		t.Fatal("resolved the removed fixed literal")
	}
	if !regexp.MustCompile(`^[0-9a-f]{48}$`).MatchString(first) {
		t.Fatalf("expected 48-char hex secret, got %q", first)
	}
}
