package pluginhost

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/config"
	"github.com/cuihairu/croupier/agents/synthetics/probe"
)

var echoprobeBin string

// TestMain compiles the slice-1 example plugin once so the tests exercise
// the real go-plugin subprocess protocol end to end.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pluginhost-test")
	if err != nil {
		panic(err)
	}
	echoprobeBin = filepath.Join(dir, "echoprobe")
	cmd := exec.Command("go", "build", "-o", echoprobeBin, "./examples/echoprobe")
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("build example plugin: " + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func echoController(t *testing.T, command string, timeout time.Duration) *Controller {
	c := NewController([]config.PluginConfig{{Name: "echo", Command: command, Timeout: timeout}}, nil)
	t.Cleanup(c.Stop)
	return c
}

func TestControllerLoadsExamplePluginAndReturnsCheckResult(t *testing.T) {
	c := echoController(t, echoprobeBin, 10*time.Second)
	out := c.Check(context.Background(), config.Target{Name: "web", Plugin: "echo", Timeout: 5 * time.Second})
	if out.Cause != "" {
		t.Fatalf("unexpected cause %q", out.Cause)
	}
	res := out.Result
	if !res.OK || res.Status != 200 || res.Target != "web" || res.Error != "" || res.At.IsZero() {
		t.Fatalf("check result: %+v", res)
	}
	// Version metadata comes from the Describe handshake issued after the
	// plugin launched; non-empty proves the handshake completed.
	if got := c.Version("echo"); got != "1.0.0" {
		t.Fatalf("describe handshake version: %q", got)
	}
	// The resident process is reused across checks.
	out = c.Check(context.Background(), config.Target{Name: "web2", Plugin: "echo", Timeout: 5 * time.Second})
	if out.Cause != "" || !out.Result.OK || out.Result.Target != "web2" {
		t.Fatalf("resident reuse check: outcome=%+v", out)
	}
}

func TestControllerMissingPluginBinaryDegradesWithCause(t *testing.T) {
	c := echoController(t, "/nonexistent/echoprobe", 5*time.Second)
	out := c.Check(context.Background(), config.Target{Name: "web", Plugin: "echo", Timeout: 2 * time.Second})
	if !strings.HasPrefix(out.Cause, "plugin_unavailable") {
		t.Fatalf("cause=%q want plugin_unavailable", out.Cause)
	}
	if out.Result != (probe.Result{}) {
		t.Fatalf("degraded result must be zero: %+v", out.Result)
	}
	if c.Version("echo") != "" {
		t.Fatal("failed plugin must not expose version metadata")
	}
}

func TestControllerPluginTimeoutDegradesWithCause(t *testing.T) {
	os.Setenv("ECHO_PROBE_DELAY_MS", "2000")
	defer os.Unsetenv("ECHO_PROBE_DELAY_MS")
	c := echoController(t, echoprobeBin, 10*time.Second)
	out := c.Check(context.Background(), config.Target{Name: "web", Plugin: "echo", Timeout: 300 * time.Millisecond})
	if !strings.HasPrefix(out.Cause, "plugin_timeout") {
		t.Fatalf("cause=%q want plugin_timeout", out.Cause)
	}
	if out.Result != (probe.Result{}) {
		t.Fatalf("timeout must not fabricate a result: %+v", out.Result)
	}
	// The broken resident process was released; a follow-up check re-launches.
	c.Check(context.Background(), config.Target{Name: "web", Plugin: "echo", Timeout: 300 * time.Millisecond})
	if c.Version("echo") != "" {
		t.Fatal("timed-out plugin process should stay collected")
	}
}

// TestControllerCheckResultRoundTripCarriesPluginFields proves result
// fidelity across the real go-plugin subprocess: plugin-reported failures
// reach the shared result chain as target failures carrying the plugin's
// own error text (no degradation cause, no fabricated success), and the
// success path carries status, latency and the bounded detail string.
func TestControllerCheckResultRoundTripCarriesPluginFields(t *testing.T) {
	// Failure first: the plugin subprocess must launch with the env set.
	os.Setenv("ECHO_PROBE_FAIL", "1")
	failed := echoController(t, echoprobeBin, 10*time.Second)
	out := failed.Check(context.Background(), config.Target{Name: "web", Plugin: "echo", Timeout: 5 * time.Second})
	if out.Cause != "" {
		t.Fatalf("plugin-reported failure must not degrade: %q", out.Cause)
	}
	if res := out.Result; res.OK || res.Status != 0 || res.Error != "echo: forced failure" || res.Target != "web" || res.At.IsZero() {
		t.Fatalf("failure round trip: %+v", res)
	}
	failed.Stop()
	os.Unsetenv("ECHO_PROBE_FAIL")

	healthy := echoController(t, echoprobeBin, 10*time.Second)
	out = healthy.Check(context.Background(), config.Target{Name: "web", Plugin: "echo", Timeout: 5 * time.Second})
	if out.Cause != "" {
		t.Fatalf("unexpected cause %q", out.Cause)
	}
	if res := out.Result; !res.OK || res.Status != 200 || res.Error != "" || res.Latency < 0 {
		t.Fatalf("success round trip: %+v", res)
	}
	if out.Detail != "echo: web" {
		t.Fatalf("bounded detail round trip: %q", out.Detail)
	}
}
