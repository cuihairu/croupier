//go:build !no_go_plugin

// Command hostdriver drives the REAL Croupier plugin hosts (the sidecar
// function host and the synthetics probe controller) against one
// external plugin process given on the command line. It exists for the
// plugin SDK test suites (sdks/plugins/python/tests via pytest,
// sdks/plugins/js/test via jest): the suite builds this binary and runs
// scenarios against the reference plugins, so the proof is a full
// host⇄plugin round trip through the real go-plugin handshake — not a
// mock of either side.
//
// Usage:
//
//	hostdriver -mode func|probe -scenario <name> -command <prog> [-args a,b]
//
// -args is a comma-separated plugin argv (empty = none). Each scenario
// prints "PASS <scenario>: <detail>" or "FAIL <scenario>: <reason>" and
// exits 0/1; the assertions live here so pytest stays a thin runner.
//
// Scenarios (func mode drives funcplugin.NewHost, probe mode drives
// pluginhost.NewController — the same hosts production code uses):
//
//	func-register      describe+list: version 1.2.3 and the four declared
//	                   plugin.pyecho.* functions register with metadata
//	func-invoke        plugin.pyecho.echo maps and returns the envelope
//	func-envelope      fail_unavailable/fail_rejected translate; unknown
//	                   ids and malformed params are bad_response
//	func-timeout       slow sleep under a short call deadline reports the
//	                   plugin_timeout host degradation cause
//	probe-check-ok     a passing target maps onto the shared probe result
//	probe-check-fail   a failing target maps as ok=false with the reason
//	probe-version      the controller stamps the described plugin version
//	probe-tcp          payloadJson reaches a real TCP reference probe and
//	                   success/refused/timeout retain their distinct
//	                   target/plugin semantics
//
// funcbench mode (hostdriver -mode funcbench -scenario bench-invoke) is a
// latency measurement tool, not an assertion scenario: it prints one
// BENCH-inline and one BENCH-plugin stats line (mean/p50/p95/max per
// call) quantifying the extra IPC the plugin hop adds over an in-process
// call; see runFuncBench for what each side measures.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/agents/sidecar/funcplugin"
	synconfig "github.com/cuihairu/croupier/agents/synthetics/config"
	"github.com/cuihairu/croupier/agents/synthetics/pluginhost"
)

func main() {
	mode := flag.String("mode", "func", "host to drive: func | probe | funcbench")
	scenario := flag.String("scenario", "", "scenario name (see the package comment)")
	command := flag.String("command", "", "plugin executable the host launches")
	args := flag.String("args", "", "comma-separated plugin argv; empty = none")
	n := flag.Int("n", 300, "funcbench: timed samples per measurement")
	warmup := flag.Int("warmup", 20, "funcbench: untimed warmup invokes before sampling")
	flag.Parse()

	if *scenario == "" || *command == "" {
		fmt.Fprintln(os.Stderr, "hostdriver: -scenario and -command are required")
		os.Exit(2)
	}
	var pluginArgs []string
	if strings.TrimSpace(*args) != "" {
		pluginArgs = strings.Split(*args, ",")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var err error
	switch *mode {
	case "func":
		err = runFuncScenario(ctx, *scenario, *command, pluginArgs)
	case "probe":
		err = runProbeScenario(*scenario, *command, pluginArgs)
	case "funcbench":
		err = runFuncBench(ctx, *scenario, *command, pluginArgs, *n, *warmup)
	default:
		err = fmt.Errorf("unknown -mode %q", *mode)
	}
	if err != nil {
		fmt.Printf("FAIL %s: %v\n", *scenario, err)
		os.Exit(1)
	}
}

// benchStats summarises per-call wall times over one measurement.
type benchStats struct {
	N    int
	Mean time.Duration
	P50  time.Duration
	P95  time.Duration
	Max  time.Duration
}

// benchMeasure runs fn n times and summarises the per-call wall times.
func benchMeasure(n int, fn func() error) (benchStats, error) {
	if n <= 0 {
		return benchStats{}, fmt.Errorf("sample count must be positive, got %d", n)
	}
	samples := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		if err := fn(); err != nil {
			return benchStats{}, err
		}
		samples = append(samples, time.Since(start))
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	var total time.Duration
	for _, s := range samples {
		total += s
	}
	pick := func(p float64) time.Duration {
		idx := int(p * float64(len(samples)))
		if idx >= len(samples) {
			idx = len(samples) - 1
		}
		if idx < 0 {
			idx = 0
		}
		return samples[idx]
	}
	return benchStats{
		N:    n,
		Mean: total / time.Duration(n),
		P50:  pick(0.50),
		P95:  pick(0.95),
		Max:  samples[len(samples)-1],
	}, nil
}

func printBench(label string, s benchStats) {
	fmt.Printf("BENCH %s n=%d mean=%.3fms p50=%.3fms p95=%.3fms max=%.3fms\n",
		label, s.N, ms(s.Mean), ms(s.P50), ms(s.P95), ms(s.Max))
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// runFuncBench measures the extra latency the plugin hop adds to one
// function invocation: "inline" runs the reference plugin's echo work
// (JSON parse/validate/marshal) in-process as the zero-IPC baseline,
// "plugin" drives the same echo through the REAL host→plugin gRPC round
// trip, so (plugin - inline) ≈ local IPC + host dispatch + wire
// marshaling. Timing-sensitive by design, hence not a CI gate; the SDK
// suites only assert the output shape.
func runFuncBench(ctx context.Context, scenario, command string, pluginArgs []string, n, warmup int) error {
	if scenario != "bench-invoke" {
		return failf("unknown funcbench scenario %q", scenario)
	}
	if warmup < 0 {
		return failf("warmup must be >= 0, got %d", warmup)
	}

	inline, err := benchMeasure(n, func() error {
		var params struct {
			Message string `json:"message"`
		}
		if e := json.Unmarshal([]byte(`{"message":"bench"}`), &params); e != nil {
			return e
		}
		if params.Message == "" {
			return fmt.Errorf("message must be a non-empty string")
		}
		if _, e := json.Marshal(map[string]string{"message": params.Message}); e != nil {
			return e
		}
		return nil
	})
	if err != nil {
		return failf("inline baseline: %v", err)
	}

	h, err := funcplugin.NewHost(
		funcplugin.AgentScope{GameID: "game-1", Env: "prod"},
		[]funcplugin.PluginConfig{{
			Name:    "pyecho",
			Command: command,
			Args:    pluginArgs,
			Timeout: 10 * time.Second,
		}},
		nil,
	)
	if err != nil {
		return failf("NewHost: %v", err)
	}
	if err := h.Start(ctx); err != nil {
		return failf("host start: %v", err)
	}
	defer h.Stop()

	for i := 0; i < warmup; i++ {
		if _, cause := h.Invoke(ctx, "plugin.pyecho.echo", `{"message":"bench"}`, 5*time.Second); cause != "" {
			return failf("warmup invoke %d degraded with cause %s", i, cause)
		}
	}
	plugin, err := benchMeasure(n, func() error {
		out, cause := h.Invoke(ctx, "plugin.pyecho.echo", `{"message":"bench"}`, 5*time.Second)
		if cause != "" {
			return fmt.Errorf("degraded with cause %s", cause)
		}
		if !out.OK || out.ErrorCode != "" {
			return fmt.Errorf("envelope ok=%v code=%q", out.OK, out.ErrorCode)
		}
		return nil
	})
	if err != nil {
		return failf("plugin hop: %v", err)
	}

	printBench("inline", inline)
	printBench("plugin", plugin)
	return nil
}

// failf is the scenario assertion helper: a non-nil error fails the
// scenario with the failing fact in the message.
func failf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

func runFuncScenario(ctx context.Context, scenario, command string, pluginArgs []string) error {
	h, err := funcplugin.NewHost(
		funcplugin.AgentScope{GameID: "game-1", Env: "prod"},
		[]funcplugin.PluginConfig{{
			Name:    "pyecho",
			Command: command,
			Args:    pluginArgs,
			Timeout: 10 * time.Second,
		}},
		nil,
	)
	if err != nil {
		return failf("NewHost: %v", err)
	}
	if err := h.Start(ctx); err != nil {
		return failf("host start: %v", err)
	}
	defer h.Stop()

	switch scenario {
	case "func-register":
		want := map[string]bool{
			"plugin.pyecho.echo":             false,
			"plugin.pyecho.fail_unavailable": false,
			"plugin.pyecho.fail_rejected":    false,
			"plugin.pyecho.slow":             false,
		}
		regs := h.Registered()
		for _, reg := range regs {
			if reg.Plugin != "pyecho" {
				continue
			}
			if reg.Version != "1.2.3" {
				return failf("plugin %s described version %q, want 1.2.3", reg.Plugin, reg.Version)
			}
			if _, known := want[reg.Declared.ID]; !known {
				return failf("plugin declared unexpected function %s", reg.Declared.ID)
			}
			want[reg.Declared.ID] = true
			if reg.Metadata == nil {
				return failf("function %s registered without shared metadata", reg.Declared.ID)
			}
		}
		for id, seen := range want {
			if !seen {
				return failf("declared function %s did not register; got %d registrations", id, len(regs))
			}
		}
		fmt.Printf("PASS func-register: %d declared functions registered with shared metadata and version 1.2.3\n", len(regs))
		return nil

	case "func-invoke":
		out, cause := h.Invoke(ctx, "plugin.pyecho.echo", `{"message":"hello-from-go"}`, 5*time.Second)
		if cause != "" {
			return failf("invoke degraded with cause %s", cause)
		}
		if !out.OK || out.ErrorCode != "" {
			return failf("invoke envelope: ok=%v code=%q msg=%q", out.OK, out.ErrorCode, out.ErrorMessage)
		}
		if !strings.Contains(out.ResultJSON, "hello-from-go") {
			return failf("mapped result lost the message: %q", out.ResultJSON)
		}
		fmt.Printf("PASS func-invoke: echo carried %q back through the envelope\n", out.ResultJSON)
		return nil

	case "func-envelope":
		// Unregistered ids never reach the plugin — the host gates them
		// with a plugin_not_configured cause — so the plugin-side envelope
		// cases below are all host-routed (registered) ids.
		cases := []struct {
			id     string
			params string
			code   string
		}{
			{"plugin.pyecho.fail_unavailable", `{}`, "target_unavailable"},
			{"plugin.pyecho.fail_rejected", `{}`, "target_rejected"},
			{"plugin.pyecho.echo", `{not-json`, "bad_response"},
			{"plugin.pyecho.echo", `{"message":""}`, "bad_response"},
			{"plugin.pyecho.echo", `[]`, "bad_response"},
		}
		for _, tc := range cases {
			out, cause := h.Invoke(ctx, tc.id, tc.params, 5*time.Second)
			if cause != "" {
				return failf("%s degraded with cause %s; business failures must stay in the envelope", tc.id, cause)
			}
			if out.OK || out.ErrorCode != tc.code {
				return failf("%s: ok=%v code=%q, want ok=false code=%s", tc.id, out.OK, out.ErrorCode, tc.code)
			}
		}
		fmt.Println("PASS func-envelope: all five business failures stayed on the closed-set envelope")
		return nil

	case "func-timeout":
		started := time.Now()
		out, cause := h.Invoke(ctx, "plugin.pyecho.slow", `{"sleep_ms":15000}`, 500*time.Millisecond)
		elapsed := time.Since(started)
		if !strings.Contains(cause, "plugin_timeout") {
			return failf("short call deadline must report plugin_timeout, got cause %q ok=%v", cause, out.OK)
		}
		if elapsed > 5*time.Second {
			return failf("invoke was not bounded by the call deadline: %v", elapsed)
		}
		fmt.Printf("PASS func-timeout: call bounded in %v and reported %s\n", elapsed.Round(time.Millisecond), cause)
		return nil

	default:
		return failf("unknown func scenario %q", scenario)
	}
}

func runProbeScenario(scenario, command string, pluginArgs []string) error {
	ctl := pluginhost.NewController(
		synconfig.Synthetics{
			Plugins: []synconfig.PluginConfig{{
				Name:    "probe-echo",
				Command: command,
				Args:    pluginArgs,
				Timeout: 10 * time.Second,
			}},
		},
		nil,
	)
	defer ctl.Stop()

	switch scenario {
	case "probe-check-ok":
		out := ctl.Check(context.Background(), synconfig.Target{
			Name:    "ok-target",
			Plugin:  "probe-echo",
			Timeout: 5 * time.Second,
		})
		if out.Cause != "" {
			return failf("check degraded with cause %s", out.Cause)
		}
		if !out.Result.OK || out.Result.Target != "ok-target" || out.Result.Status != 200 {
			return failf("check result: ok=%v target=%q status=%d error=%q", out.Result.OK, out.Result.Target, out.Result.Status, out.Result.Error)
		}
		if !strings.Contains(out.Detail, "probe-echo checked ok-target") {
			return failf("plugin detail lost: %q", out.Detail)
		}
		fmt.Printf("PASS probe-check-ok: ok-target passed with status %d and detail\n", out.Result.Status)
		return nil

	case "probe-check-fail":
		out := ctl.Check(context.Background(), synconfig.Target{
			Name:    "fail-target",
			Plugin:  "probe-echo",
			Timeout: 5 * time.Second,
		})
		if out.Cause != "" {
			return failf("a business failure must stay out of the degradation causes, got %s", out.Cause)
		}
		if out.Result.OK || out.Result.Status != 500 || !strings.Contains(out.Result.Error, "synthetic probe failure") {
			return failf("check result: ok=%v status=%d error=%q", out.Result.OK, out.Result.Status, out.Result.Error)
		}
		fmt.Println("PASS probe-check-fail: fail-target reported ok=false with the synthetic reason")
		return nil

	case "probe-version":
		// Version reports the described metadata of a started process, so
		// the scenario runs one check first to bring the plugin up.
		out := ctl.Check(context.Background(), synconfig.Target{
			Name:    "ok-target",
			Plugin:  "probe-echo",
			Timeout: 5 * time.Second,
		})
		if out.Cause != "" {
			return failf("warmup check degraded with cause %s", out.Cause)
		}
		version := ctl.Version("probe-echo")
		if !strings.Contains(version, "1.0.0") {
			return failf("controller stamped version %q, want it to contain 1.0.0", version)
		}
		fmt.Printf("PASS probe-version: controller stamped %q\n", version)
		return nil

	case "probe-tcp":
		listener, listenErr := net.Listen("tcp", "127.0.0.1:0")
		if listenErr != nil {
			return fmt.Errorf("start local TCP target: %w", listenErr)
		}
		defer listener.Close()
		_, portStr, _ := net.SplitHostPort(listener.Addr().String())
		port, _ := strconv.Atoi(portStr)
		payload := fmt.Sprintf(`{"host":"127.0.0.1","port":%d}`, port)

		check := func(name string, timeoutMS int) pluginhost.CheckOutcome {
			return ctl.Check(context.Background(), synconfig.Target{
				Name:        name,
				Plugin:      "probe-echo",
				Timeout:     time.Duration(timeoutMS) * time.Millisecond,
				PayloadJSON: payload,
			})
		}

		open := check("tcp-open", 5000)
		if open.Cause != "" {
			return failf("open TCP check degraded with cause %s", open.Cause)
		}
		if !open.Result.OK || open.Result.Status != 200 {
			return failf("open TCP check: ok=%v status=%d detail=%q error=%q", open.Result.OK, open.Result.Status, open.Detail, open.Result.Error)
		}

		var closedPort int
		closedListener, closeErr := net.Listen("tcp", "127.0.0.1:0")
		if closeErr != nil {
			return fmt.Errorf("reserve closed TCP port: %w", closeErr)
		}
		_, closedPortStr, _ := net.SplitHostPort(closedListener.Addr().String())
		closedPort, _ = strconv.Atoi(closedPortStr)
		closedListener.Close()

		closedPayload := fmt.Sprintf(`{"host":"127.0.0.1","port":%d}`, closedPort)
		closed := ctl.Check(context.Background(), synconfig.Target{
			Name: "tcp-refused", Plugin: "probe-echo", Timeout: 2 * time.Second,
			PayloadJSON: closedPayload,
		})
		if closed.Cause != "" {
			return failf("refused TCP check degraded with cause %s", closed.Cause)
		}
		if closed.Result.OK || !strings.Contains(strings.ToLower(closed.Result.Error), "refused") {
			return failf("refused TCP check: ok=%v error=%q", closed.Result.OK, closed.Result.Error)
		}

		controllerCall := func(timeoutMS int, payload string) pluginhost.CheckOutcome {
			return ctl.Check(context.Background(), synconfig.Target{
				Name: "tcp-cancel", Plugin: "probe-echo", Timeout: time.Duration(timeoutMS) * time.Millisecond,
				PayloadJSON: payload,
			})
		}
		timedOut := controllerCall(80, `{"sleep_ms":1000}`)
		if !strings.Contains(timedOut.Cause, pluginhost.CauseTimeout) {
			return failf("timeout cause=%q want prefix %q", timedOut.Cause, pluginhost.CauseTimeout)
		}
		fmt.Printf("PASS probe-tcp: payload reached plugin; open=%v refused=%q timeout=%q\n", open.Result.OK, closed.Result.Error, timedOut.Cause)
		return nil

	default:
		return failf("unknown probe scenario %q", scenario)
	}
}
