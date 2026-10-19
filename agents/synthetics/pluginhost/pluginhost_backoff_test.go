//go:build !no_go_plugin

package pluginhost

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/config"
	"github.com/cuihairu/croupier/agents/synthetics/probe"
)

// backoffProbeController builds a controller with small test windows.
func backoffProbeController(t *testing.T, breakAfter int) *Controller {
	c := NewController(config.Synthetics{
		Plugins:          []config.PluginConfig{{Name: "echo", Command: echoprobeBin, Timeout: 10 * time.Second}},
		PluginBackoff:    150 * time.Millisecond,
		PluginBackoffMax: 600 * time.Millisecond,
		PluginBreakAfter: breakAfter,
		PluginOpenWindow: 300 * time.Millisecond,
	}, nil)
	t.Cleanup(c.Stop)
	return c
}

// TestControllerCrashBackoffAndCircuitBreaker pins the slice-2 crash
// contract (todo.md:36): a plugin process crash is recorded as a failure,
// the single lifecycle owner restarts it only after the exponential
// backoff gate, and consecutive failures open the degradation circuit
// that stops touching the subprocess until its window elapses; the first
// healthy call closes the circuit again.
func TestControllerCrashBackoffAndCircuitBreaker(t *testing.T) {
	os.Setenv("ECHO_PROBE_EXIT_AFTER_CHECKS", "1") // every spawn crashes on its first check
	defer os.Unsetenv("ECHO_PROBE_EXIT_AFTER_CHECKS")
	c := backoffProbeController(t, 2)
	ctx := context.Background()
	target := config.Target{Name: "web", Plugin: "echo", Timeout: 2 * time.Second}

	// First attempt: start fails at the crashed call → explicit cause,
	// backoff gate armed (consecutive failure #1, next retry in 150ms).
	out := c.Check(ctx, target)
	if !strings.HasPrefix(out.Cause, "plugin_unavailable") {
		t.Fatalf("crashing plugin must degrade with plugin_unavailable: %q", out.Cause)
	}
	if v := c.Version("echo"); v != "" {
		t.Fatalf("broken resident must not expose version metadata: %q", v)
	}

	// Immediate retry: still inside the backoff gate → no restart storm.
	out = c.Check(ctx, target)
	if !strings.HasPrefix(out.Cause, "plugin_backoff") {
		t.Fatalf("restart before the backoff gate must degrade with plugin_backoff: %q", out.Cause)
	}

	// Past the gate the host retries once; the crash repeats and the
	// breaker (breakAfter=2) opens the circuit on the spot.
	time.Sleep(220 * time.Millisecond)
	out = c.Check(ctx, target)
	if !strings.HasPrefix(out.Cause, "plugin_circuit_open") {
		t.Fatalf("second consecutive crash must open the degradation circuit: %q", out.Cause)
	}

	// While the circuit is open calls degrade without touching the
	// subprocess: the cause is immediate, no fresh version metadata.
	start := time.Now()
	out = c.Check(ctx, target)
	if !strings.HasPrefix(out.Cause, "plugin_circuit_open") {
		t.Fatalf("open circuit must degrade every call: %q", out.Cause)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("open-circuit degradation must be immediate, took %s", elapsed)
	}

	// After the open window exactly one probe path is allowed; the still
	// crashing plugin fails it and re-opens the circuit.
	time.Sleep(380 * time.Millisecond)
	out = c.Check(ctx, target)
	if !strings.HasPrefix(out.Cause, "plugin_circuit_open") {
		t.Fatalf("failed half-open probe must re-open the circuit: %q", out.Cause)
	}

	// Recovery: with the crash injection removed, the next probe past the
	// window launches a healthy process and the first success closes the
	// circuit and clears the backoff state.
	os.Unsetenv("ECHO_PROBE_EXIT_AFTER_CHECKS")
	time.Sleep(380 * time.Millisecond)
	out = c.Check(ctx, target)
	if out.Cause != "" || !out.Result.OK || out.Result.Target != "web" {
		t.Fatalf("healthy plugin must close the circuit and answer: %+v", out)
	}
	if v := c.Version("echo"); v != "1.0.0" {
		t.Fatalf("recovered plugin must expose describe metadata again: %q", v)
	}
	// Resident reuse: the closed circuit stays closed.
	if out := c.Check(ctx, target); out.Cause != "" || !out.Result.OK {
		t.Fatalf("resident plugin must keep answering: %q", out.Cause)
	}
}

// TestControllerPerPluginSwitchDegradesWithoutLaunch pins the per-plugin
// enabled switch (todo.md:36 第二片): a plugin declared enabled:false
// never starts and any check routed at it degrades with plugin_disabled,
// which the runner maps onto the native HTTP probe fallback.
func TestControllerPerPluginSwitchDegradesWithoutLaunch(t *testing.T) {
	off := false
	c := NewController(config.Synthetics{
		Plugins: []config.PluginConfig{{Name: "echo", Command: echoprobeBin, Timeout: 10 * time.Second, Enabled: &off}},
	}, nil)
	t.Cleanup(c.Stop)
	out := c.Check(context.Background(), config.Target{Name: "web", Plugin: "echo", Timeout: 2 * time.Second})
	if !strings.HasPrefix(out.Cause, "plugin_disabled") {
		t.Fatalf("disabled plugin must degrade without launching: %q", out.Cause)
	}
	if c.Version("echo") != "" {
		t.Fatal("disabled plugin must not expose handshake metadata")
	}
	if out.Result != (probe.Result{}) {
		t.Fatalf("degraded result must be zero: %+v", out.Result)
	}
}
