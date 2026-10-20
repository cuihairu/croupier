//go:build !no_go_plugin

package config

import (
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestSyntheticsPluginKnobsYamlTwoStates pins the yaml surface of the
// slice-2 plugin knobs (todo.md:36): pluginsEnabled defaults to on
// (默认集成) and parses an explicit false; per-plugin enabled:false
// parses; optional duration knobs parse; negative values are rejected.
func TestSyntheticsPluginPayloadValidation(t *testing.T) {
	t.Run("parses and forwards plugin payload", func(t *testing.T) {
		c, err := Load("testdata/plugin_payload.yaml")
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		target := c.Synthetics.Targets[0]
		if target.PayloadJSON != `{"expect":"tcp"}` {
			t.Fatalf("PayloadJSON = %q", target.PayloadJSON)
		}
	})

	t.Run("rejects invalid payload", func(t *testing.T) {
		_, err := Load("testdata/plugin_payload_invalid.yaml")
		if err == nil || !strings.Contains(err.Error(), "payloadJson must be valid JSON") {
			t.Fatalf("Load() error = %v, want payload JSON validation", err)
		}
	})
}

func TestSyntheticsICMPTargetParsingAndValidation(t *testing.T) {
	c, err := Load("testdata/target_type_icmp.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	target := c.Synthetics.Targets[0]
	if target.Type != TypeICMP || target.Address != "127.0.0.1" || target.Count != 2 {
		t.Fatalf("ICMP target = %+v", target)
	}

	_, err = Load("testdata/target_type_icmp_invalid.yaml")
	if err == nil || !strings.Contains(err.Error(), "type icmp requires address") {
		t.Fatalf("Load() error = %v, want ICMP address validation", err)
	}
}

func TestSyntheticsPluginKnobsYamlTwoStatesAndValidation(t *testing.T) {
	base := func(toggles string) string {
		return `
agent: {id: a, serverAddr: s, gameId: g, env: e}
synthetics:
  interval: 30s
` + toggles + `
  targets:
    - {name: t, url: "http://127.0.0.1", method: GET, timeout: 1s, expectedStatus: 200}
`
	}
	load := func(t *testing.T, text string) Config {
		t.Helper()
		var c Config
		if err := yaml.Unmarshal([]byte(text), &c); err != nil {
			t.Fatalf("parse yaml: %v", err)
		}
		return c
	}

	// Default: switch absent → on.
	c := load(t, base(""))
	if !c.Synthetics.PluginsOn() {
		t.Fatal("plugins absent must default to enabled integration")
	}
	// Explicit off.
	c = load(t, base("  pluginsEnabled: false"))
	if c.Synthetics.PluginsOn() {
		t.Fatal("pluginsEnabled: false must disable the runtime switch")
	}
	// Explicit on.
	c = load(t, base("  pluginsEnabled: true"))
	if !c.Synthetics.PluginsOn() {
		t.Fatal("pluginsEnabled: true must enable the runtime switch")
	}
	// Per-plugin switch off.
	c = load(t, base("")+`
  pluginsEnabled: true
  plugins:
    - {name: echo, command: ./echo, timeout: 1s, enabled: false}
`)
	if len(c.Synthetics.Plugins) != 1 || c.Synthetics.Plugins[0].IsEnabled() {
		t.Fatalf("per-plugin enabled:false must parse through: %+v", c.Synthetics.Plugins)
	}
	// Optional knobs parse.
	c = load(t, base("")+`
  pluginsEnabled: false
  pluginMaxConcurrent: 3
  pluginQueueWindow: 250ms
  pluginBackoff: 500ms
  pluginBackoffMax: 10s
  pluginBreakAfter: 4
  pluginOpenWindow: 5s
`)
	k := c.Synthetics
	if k.PluginMaxConcurrent != 3 || k.PluginQueueWindow != 250*time.Millisecond || k.PluginBackoff != 500*time.Millisecond || k.PluginBackoffMax != 10*time.Second || k.PluginBreakAfter != 4 || k.PluginOpenWindow != 5*time.Second {
		t.Fatalf("knob parse: %+v", k)
	}

	// Validation: negative values rejected.
	negatives := func() Config {
		return load(t, base(""))
	}
	for key, mutate := range map[string]func(*Config){
		"pluginMaxConcurrent": func(c *Config) { c.Synthetics.PluginMaxConcurrent = -1 },
		"pluginQueueWindow":   func(c *Config) { c.Synthetics.PluginQueueWindow = -time.Second },
		"pluginBackoff":       func(c *Config) { c.Synthetics.PluginBackoff = -time.Second },
		"pluginBackoffMax":    func(c *Config) { c.Synthetics.PluginBackoffMax = -time.Second },
		"pluginBreakAfter":    func(c *Config) { c.Synthetics.PluginBreakAfter = -1 },
		"pluginOpenWindow":    func(c *Config) { c.Synthetics.PluginOpenWindow = -time.Second },
	} {
		c = negatives()
		mutate(&c)
		err := c.Validate()
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Fatalf("negative synthetics.%s accepted: %v", key, err)
		}
	}
}
