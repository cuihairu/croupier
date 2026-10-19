//go:build !no_go_plugin

package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/config"
)

// TestRunProbePluginRuntimeSwitchTwoStates pins the slice-2 runtime
// switch (todo.md:36): pluginsEnabled: false (the only way to turn the
// 默认集成 runtime off) builds no plugin controller and routes
// plugin-declared targets straight to the native HTTP probe with an
// explicit plugin_disabled cause; the enabled state routes the same
// target through the resident plugin as in slice 1.
func TestRunProbePluginRuntimeSwitchTwoStates(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("healthy"))
	}))
	defer s.Close()
	pluginTarget := config.Target{Name: "web", URL: s.URL, Method: "GET", Timeout: 5 * time.Second, ExpectedStatus: 200, BodyContains: "healthy", Plugin: "echo"}
	base := config.Config{
		Agent:      config.Agent{ID: "syn-1", GameID: "demo", Env: "prod"},
		Synthetics: config.Synthetics{Interval: time.Minute, Plugins: []config.PluginConfig{{Name: "echo", Command: os.Getenv("RUNNER_PLUGIN_TEST_BIN"), Timeout: 10 * time.Second}}},
	}

	off := false
	disabled := base
	disabled.Synthetics.PluginsEnabled = &off
	r := New(disabled, nil)
	if r.plugins != nil {
		t.Fatal("runtime switch off must not create the plugin controller")
	}
	res, details := r.runProbe(context.Background(), pluginTarget)
	if !res.OK || res.Status != 200 {
		t.Fatalf("disabled runtime must keep the target alive via the native HTTP probe: %+v", res)
	}
	if details["probeType"] != "http" || details["pluginFallback"] != "true" || !strings.HasPrefix(details["pluginCause"], "plugin_disabled") {
		t.Fatalf("disabled state must mark the explicit fallback: %v", details)
	}

	on := true
	enabled := base
	enabled.Synthetics.PluginsEnabled = &on
	r2 := New(enabled, nil)
	if r2.plugins == nil {
		t.Fatal("runtime switch on must create the plugin controller")
	}
	defer r2.stopPlugins()
	res, details = r2.runProbe(context.Background(), pluginTarget)
	if !res.OK || res.Status != 200 {
		t.Fatalf("enabled runtime must answer through the plugin: %+v", res)
	}
	if details["probeType"] != "plugin" || details["pluginName"] != "echo" || details["pluginVersion"] != "1.0.0" || details["status"] != "success" {
		t.Fatalf("enabled state must route through the plugin: %v", details)
	}
}

// TestRunProbePerPluginSwitchRoutesNativeFallback pins the per-plugin
// switch (todo.md:32): with the runtime master switch on, a plugin
// declared enabled:false still builds the controller but never launches
// its subprocess, and plugin-routed targets degrade to the native HTTP
// probe with an explicit plugin_disabled cause — disabling one plugin
// must not turn off the runtime for the others.
func TestRunProbePerPluginSwitchRoutesNativeFallback(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("healthy"))
	}))
	defer s.Close()
	pluginTarget := config.Target{Name: "web", URL: s.URL, Method: "GET", Timeout: 5 * time.Second, ExpectedStatus: 200, BodyContains: "healthy", Plugin: "echo"}
	off := false
	cfg := config.Config{
		Agent:      config.Agent{ID: "syn-1", GameID: "demo", Env: "prod"},
		Synthetics: config.Synthetics{Interval: time.Minute, Plugins: []config.PluginConfig{{Name: "echo", Command: os.Getenv("RUNNER_PLUGIN_TEST_BIN"), Timeout: 10 * time.Second, Enabled: &off}}},
	}
	r := New(cfg, nil)
	if r.plugins == nil {
		t.Fatal("master switch on must still create the plugin controller for the remaining plugins")
	}
	defer r.stopPlugins()
	res, details := r.runProbe(context.Background(), pluginTarget)
	if !res.OK || res.Status != 200 {
		t.Fatalf("disabled plugin must keep the target alive via the native HTTP probe: %+v", res)
	}
	if details["probeType"] != "http" || details["pluginFallback"] != "true" || !strings.HasPrefix(details["pluginCause"], "plugin_disabled") {
		t.Fatalf("per-plugin disabled state must mark the explicit fallback: %v", details)
	}
	if v := r.plugins.Version("echo"); v != "" {
		t.Fatalf("disabled plugin must never launch its subprocess: %q", v)
	}
}
