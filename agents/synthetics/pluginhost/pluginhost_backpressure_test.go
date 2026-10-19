//go:build !no_go_plugin

package pluginhost

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/config"
)

var concRe = regexp.MustCompile(`cur=(\d+) max=(\d+)`)

func parseConc(detail string) (cur, max int, ok bool) {
	m := concRe.FindStringSubmatch(detail)
	if m == nil {
		return 0, 0, false
	}
	c, _ := strconv.Atoi(m[1])
	x, _ := strconv.Atoi(m[2])
	return c, x, true
}

// TestControllerBackpressureSlotsBoundedRejectionAndQueueing pins the
// slice-2 concurrency contract (todo.md:36): concurrent plugin calls are
// accepted up to the configured slot limit, extra callers queue for a
// slot at most for the bounded window and are then rejected with
// plugin_backpressure, the plugin-side observed concurrency never exceeds
// the limit, and the slot pool drains fully afterwards. The controller
// path spawns no goroutines; saturation cannot create unbounded work.
func TestControllerBackpressureSlotsBoundedRejectionAndQueueing(t *testing.T) {
	os.Setenv("ECHO_PROBE_CONC", "1")
	os.Setenv("ECHO_PROBE_HOLD_MS", "1000")
	defer func() {
		os.Unsetenv("ECHO_PROBE_CONC")
		os.Unsetenv("ECHO_PROBE_HOLD_MS")
	}()
	c := NewController(config.Synthetics{
		Plugins:             []config.PluginConfig{{Name: "echo", Command: echoprobeBin, Timeout: 10 * time.Second}},
		PluginMaxConcurrent: 2,
		PluginQueueWindow:   300 * time.Millisecond,
	}, nil)
	t.Cleanup(c.Stop)
	target := config.Target{Name: "web", Plugin: "echo", Timeout: 5 * time.Second}

	// Two slow checks occupy both slots; poll until saturation.
	flight := make([]chan CheckOutcome, 2)
	for i := range flight {
		flight[i] = make(chan CheckOutcome, 1)
		go func(o chan CheckOutcome) { o <- c.Check(context.Background(), target) }(flight[i])
	}
	deadline := time.Now().Add(3 * time.Second)
	for c.InFlight() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if c.InFlight() < 2 {
		t.Fatal("two slow checks never saturated the two slots")
	}

	// Rejected caller: waits the bounded 300ms window and is then
	// rejected instead of piling up behind the busy plugin.
	start := time.Now()
	rejected := c.Check(context.Background(), target)
	elapsed := time.Since(start)
	if !strings.HasPrefix(rejected.Cause, "plugin_backpressure") {
		t.Fatalf("saturated caller must be rejected with plugin_backpressure: %s", rejected.Cause)
	}
	if elapsed < 250*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("rejection must follow the bounded ~300ms queue wait, got %s", elapsed)
	}

	// Extra concurrent callers while both slots stay busy; all rejected,
	// observed plugin concurrency stays ≤ limit.
	extra := make([]chan CheckOutcome, 2)
	for i := range extra {
		extra[i] = make(chan CheckOutcome, 1)
		go func(o chan CheckOutcome) { o <- c.Check(context.Background(), target) }(extra[i])
	}
	for i := range extra {
		o := <-extra[i]
		if !strings.HasPrefix(o.Cause, "plugin_backpressure") {
			t.Fatalf("extra caller %d must be bounded-rejected, got: %s", i, o.Cause)
		}
	}
	// The two original in-flight calls complete healthy, and the plugin
	// never observed more concurrency than the slot capacity.
	for i := 0; i < 2; i++ {
		o := <-flight[i]
		if o.Cause != "" {
			t.Fatalf("in-flight caller %d degraded: %s", i, o.Cause)
		}
		cur, max, ok := parseConc(o.Detail)
		if !ok || cur < 1 || cur > 2 || max > 2 {
			t.Fatalf("plugin observed concurrency must stay within the 2-slot limit: %q", o.Detail)
		}
	}
	if c.InFlight() != 0 {
		t.Fatalf("slot pool must drain to zero after calls return, in flight: %d", c.InFlight())
	}
	if o := c.Check(context.Background(), target); o.Cause != "" || !o.Result.OK {
		t.Fatalf("released caller must acquire a slot and run again: %s", o.Cause)
	}
}

// TestControllerBackpressureQueuedCallerGetsFreedSlot pins the queueing
// half of the bounded policy: when a slot frees inside the queue window
// the waiting caller proceeds instead of being rejected, still under the
// same slot capacity.
func TestControllerBackpressureQueuedCallerGetsFreedSlot(t *testing.T) {
	os.Setenv("ECHO_PROBE_CONC", "1")
	os.Setenv("ECHO_PROBE_HOLD_MS", "300")
	defer func() {
		os.Unsetenv("ECHO_PROBE_CONC")
		os.Unsetenv("ECHO_PROBE_HOLD_MS")
	}()
	c := NewController(config.Synthetics{
		Plugins:             []config.PluginConfig{{Name: "echo", Command: echoprobeBin, Timeout: 10 * time.Second}},
		PluginMaxConcurrent: 2,
		PluginQueueWindow:   5 * time.Second,
	}, nil)
	t.Cleanup(c.Stop)
	target := config.Target{Name: "web", Plugin: "echo", Timeout: 5 * time.Second}

	flight := make([]chan CheckOutcome, 2)
	for i := range flight {
		flight[i] = make(chan CheckOutcome, 1)
		go func(o chan CheckOutcome) { o <- c.Check(context.Background(), target) }(flight[i])
	}
	deadline := time.Now().Add(3 * time.Second)
	for c.InFlight() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if c.InFlight() < 2 {
		t.Fatal("two slow checks never saturated the two slots")
	}

	start := time.Now()
	out := c.Check(context.Background(), target) // queues for a freed slot
	elapsed := time.Since(start)
	if out.Cause != "" || !out.Result.OK {
		t.Fatalf("caller within the queue window must succeed after the slot frees: %s", out.Cause)
	}
	if elapsed < 250*time.Millisecond || elapsed > 5*time.Second {
		t.Fatalf("queued caller must wait for the busy plugins (~300ms), got %s", elapsed)
	}
	if _, max, ok := parseConc(out.Detail); !ok || max > 2 {
		t.Fatalf("queued execution must stay within the 2-slot limit: %q", out.Detail)
	}
	for i := 0; i < 2; i++ {
		o := <-flight[i]
		if o.Cause != "" {
			t.Fatalf("original caller %d degraded: %s", i, o.Cause)
		}
	}
}
