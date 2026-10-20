//go:build !no_go_plugin

package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/config"
)

// TestMain compiles the example plugin once; the runner-level tests route
// it through the real go-plugin protocol into the AlertEvent chain.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "runner-plugin-test")
	if err != nil {
		panic(err)
	}
	bin := filepath.Join(dir, "echoprobe")
	cmd := exec.Command("go", "build", "-o", bin, "../pluginhost/examples/echoprobe")
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("build example plugin: " + string(out))
	}
	os.Setenv("RUNNER_PLUGIN_TEST_BIN", bin)
	code := m.Run()
	os.RemoveAll(dir)
	os.Unsetenv("RUNNER_PLUGIN_TEST_BIN")
	os.Exit(code)
}

func pluginRunner(t *testing.T, command string) *Runner {
	r := New(config.Config{Agent: config.Agent{ID: "syn-1", GameID: "demo", Env: "prod"}, Synthetics: config.Synthetics{Interval: time.Minute, Plugins: []config.PluginConfig{{Name: "echo", Command: command, Timeout: 10 * time.Second}}}}, nil)
	t.Cleanup(r.stopPlugins)
	return r
}

func (r *Runner) stopPlugins() {
	if r.plugins != nil {
		r.plugins.Stop()
	}
}

// TestRunProbePluginResultFeedsAlertEventChain proves the healthy plugin
// path end to end at runner level: the plugin answer is reported as
// probeType=plugin with identity metadata and the bounded plugin detail,
// riding the same AlertEvent contract as native HTTP results.
func TestRunProbePluginResultFeedsAlertEventChain(t *testing.T) {
	r := pluginRunner(t, os.Getenv("RUNNER_PLUGIN_TEST_BIN"))
	target := config.Target{Name: "web", URL: "http://localhost", Method: "GET", Timeout: 5 * time.Second, ExpectedStatus: 200, Plugin: "echo"}
	res, details := r.runProbe(context.Background(), target)
	if details["probeType"] != "plugin" || details["pluginName"] != "echo" || details["pluginVersion"] != "1.0.0" || details["pluginDetail"] != "echo: web" || details["status"] != "success" || details["errorClass"] != "" {
		t.Fatalf("plugin details: %v", details)
	}
	if !res.OK || res.Status != 200 || res.Target != "web" || res.Latency < 0 {
		t.Fatalf("plugin result: %+v", res)
	}
	// Result reproducibility: identical outcome on the second interval tick
	// through the resident plugin process, bounded detail included.
	out := r.plugins.Check(context.Background(), target)
	if out.Cause != "" || !out.Result.OK || out.Detail != "echo: web" {
		t.Fatalf("resident plugin check: outcome=%+v", out)
	}
	// Plumb the plugin outcome into the same AlertEvent shape as HTTP:
	ev := buildProbeEvent(r.cfg, target, out.Result, details)
	if ev.Kind != "probe.detail" || ev.AgentId != "syn-1" || ev.GameId != "demo" || ev.Env != "prod" || ev.Detail.Target != "web" || !ev.Detail.Ok || ev.Detail.Details["probeType"] != "plugin" || ev.Detail.Details["pluginDetail"] != "echo: web" {
		t.Fatalf("event: %+v", ev.Detail)
	}
}

// TestRunProbePluginMissingFallsBackToNativeHTTP pins the slice-1 fallback
// semantics (todo.md:36): a declared plugin that cannot run never takes
// the target down — the native HTTP probe answers instead, and the
// degradation cause travels through the existing AlertEvent detail map so
// plugin faults stay distinguishable from target faults.
func TestRunProbePluginMissingFallsBackToNativeHTTP(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("healthy"))
	}))
	defer s.Close()
	r := pluginRunner(t, s.URL+"/does-not-exist") // runner's plugin command points at a missing binary
	res, details := r.runProbe(context.Background(), config.Target{Name: "web", URL: s.URL, Method: "GET", Timeout: 5 * time.Second, ExpectedStatus: 200, BodyContains: "healthy", Plugin: "echo"})
	if !res.OK || res.Status != 200 {
		t.Fatalf("fallback native HTTP probe must keep the target alive: %+v", res)
	}
	if details["probeType"] != "http" || details["pluginFallback"] != "true" || !strings.HasPrefix(details["pluginCause"], "plugin_unavailable") {
		t.Fatalf("fallback must be explicit: %v", details)
	}
	ev := buildProbeEvent(r.cfg, config.Target{Name: "web", Plugin: "echo"}, res, details)
	if ev.Kind != "probe.detail" || !ev.Detail.Ok || ev.Detail.Details["pluginFallback"] != "true" {
		t.Fatalf("fallback event: %+v", ev)
	}
}

func TestRunProbeICMPPluginBusinessFailureIsNotHTTPFallback(t *testing.T) {
	r := pluginRunner(t, os.Getenv("RUNNER_PLUGIN_TEST_BIN"))
	os.Setenv("ECHO_PROBE_FAIL", "1")
	t.Cleanup(func() { os.Unsetenv("ECHO_PROBE_FAIL") })
	target := config.Target{
		Name: "gateway", Type: config.TypeICMP, Address: "127.0.0.1",
		URL: "http://127.0.0.1:1/degraded", Method: "GET", Timeout: 5 * time.Second,
		ExpectedStatus: 200, Plugin: "echo",
	}
	res, details := r.runProbe(context.Background(), target)
	if res.OK || !strings.Contains(res.Error, "echo: forced failure") {
		t.Fatalf("ICMP plugin failure must stay a target failure: %+v", res)
	}
	if details["probeType"] != "plugin:icmp" || details["pluginFallback"] == "true" || details["pluginCause"] != "" {
		t.Fatalf("plugin ICMP details: %v", details)
	}
}
