// Package config loads the explicitly configured HTTP checks for synthetics-agent.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Agent       Agent      `yaml:"agent"`
	OutboundTLS TLS        `yaml:"outboundTLS"`
	Synthetics  Synthetics `yaml:"synthetics"`
}
type Agent struct {
	ID            string `yaml:"id"`
	ServerAddr    string `yaml:"serverAddr"`
	GameID        string `yaml:"gameId"`
	Env           string `yaml:"env"`
	HeartbeatFile string `yaml:"heartbeatFile"`
}
type TLS struct {
	Enabled            bool   `yaml:"enabled"`
	CertFile           string `yaml:"certFile"`
	KeyFile            string `yaml:"keyFile"`
	CAFile             string `yaml:"caFile"`
	ServerName         string `yaml:"serverName"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
}

// PluginConfig declares one external probe plugin process (todo.md:36).
// Commands are explicit config entries: nothing is launched unless the
// config declares it and a target references it by name.
type PluginConfig struct {
	Name    string        `yaml:"name"`
	Command string        `yaml:"command"`
	Args    []string      `yaml:"args"`
	Timeout time.Duration `yaml:"timeout"`
	// Enabled is the per-plugin runtime switch (todo.md:36 第二片).
	// nil defaults to enabled; false keeps the plugin process never
	// launched and plugin-routed targets degrade to the native HTTP
	// probe with an explicit plugin_disabled cause.
	Enabled *bool `yaml:"enabled"`
}

func (p *PluginConfig) UnmarshalYAML(n *yaml.Node) error {
	var raw struct {
		Name    string   `yaml:"name"`
		Command string   `yaml:"command"`
		Args    []string `yaml:"args"`
		Timeout string   `yaml:"timeout"`
		Enabled *bool    `yaml:"enabled"`
	}
	if err := n.Decode(&raw); err != nil {
		return err
	}
	d, err := time.ParseDuration(raw.Timeout)
	if err != nil {
		return fmt.Errorf("plugin %q timeout: %w", raw.Name, err)
	}
	*p = PluginConfig{Name: raw.Name, Command: raw.Command, Args: raw.Args, Timeout: d, Enabled: raw.Enabled}
	return nil
}

func (p *PluginConfig) IsEnabled() bool { return p.Enabled == nil || *p.Enabled }

// parseOptDuration parses an optional yaml duration string; empty means
// "unset", which lets the plugin host apply its own defaults.
func parseOptDuration(raw, label string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", label, err)
	}
	return d, nil
}

type Synthetics struct {
	Interval time.Duration  `yaml:"interval"`
	Targets  []Target       `yaml:"targets"`
	Plugins  []PluginConfig `yaml:"plugins"`

	// No-op go-plugin runtime knobs (todo.md:36 第二片). All fields are
	// optional: zero values select the host defaults in
	// pluginhost.Default* constants, so a config that declares nothing
	// keeps the slice-1 behavior.
	//
	// PluginsEnabled is the runtime master switch: default enabled
	// (默认集成、可禁用口径). false never starts any plugin subprocess and
	// plugin-routed targets directly degrade to the native HTTP probe
	// with an explicit plugin_disabled cause.
	PluginsEnabled *bool `yaml:"pluginsEnabled"`
	// PluginMaxConcurrent caps concurrent plugin calls across all
	// plugins; callers queue for a free slot at most PluginQueueWindow
	// long, then are rejected (bounded queue, no unbounded goroutines).
	PluginMaxConcurrent int           `yaml:"pluginMaxConcurrent"`
	PluginQueueWindow   time.Duration `yaml:"pluginQueueWindow"`
	// PluginBackoff / PluginBackoffMax shape the exponential restart
	// backoff after a plugin process crash; PluginBreakAfter consecutive
	// failures open the PluginOpenWindow circuit that degrades without
	// touching the subprocess.
	PluginBackoff    time.Duration `yaml:"pluginBackoff"`
	PluginBackoffMax time.Duration `yaml:"pluginBackoffMax"`
	PluginBreakAfter int           `yaml:"pluginBreakAfter"`
	PluginOpenWindow time.Duration `yaml:"pluginOpenWindow"`
}

func (s *Synthetics) UnmarshalYAML(n *yaml.Node) error {
	var raw struct {
		Interval string         `yaml:"interval"`
		Targets  []Target       `yaml:"targets"`
		Plugins  []PluginConfig `yaml:"plugins"`
		// Plugin runtime knobs (todo.md:36 第二片).
		PluginsEnabled      *bool  `yaml:"pluginsEnabled"`
		PluginMaxConcurrent int    `yaml:"pluginMaxConcurrent"`
		PluginQueueWindow   string `yaml:"pluginQueueWindow"`
		PluginBackoff       string `yaml:"pluginBackoff"`
		PluginBackoffMax    string `yaml:"pluginBackoffMax"`
		PluginBreakAfter    int    `yaml:"pluginBreakAfter"`
		PluginOpenWindow    string `yaml:"pluginOpenWindow"`
	}
	if err := n.Decode(&raw); err != nil {
		return err
	}
	d, err := time.ParseDuration(raw.Interval)
	if err != nil {
		return fmt.Errorf("interval: %w", err)
	}
	knobs := map[string]struct {
		raw string
		dst *time.Duration
	}{
		"pluginQueueWindow": {raw.PluginQueueWindow, new(time.Duration)},
		"pluginBackoff":     {raw.PluginBackoff, new(time.Duration)},
		"pluginBackoffMax":  {raw.PluginBackoffMax, new(time.Duration)},
		"pluginOpenWindow":  {raw.PluginOpenWindow, new(time.Duration)},
	}
	parsed := map[string]time.Duration{}
	for k, f := range knobs {
		v, err := parseOptDuration(f.raw, k)
		if err != nil {
			return err
		}
		parsed[k] = v
	}
	s.Interval = d
	s.Targets = raw.Targets
	s.Plugins = raw.Plugins
	s.PluginsEnabled = raw.PluginsEnabled
	s.PluginMaxConcurrent = raw.PluginMaxConcurrent
	s.PluginQueueWindow = parsed["pluginQueueWindow"]
	s.PluginBackoff = parsed["pluginBackoff"]
	s.PluginBackoffMax = parsed["pluginBackoffMax"]
	s.PluginBreakAfter = raw.PluginBreakAfter
	s.PluginOpenWindow = parsed["pluginOpenWindow"]
	return nil
}

// PluginsOn reports the normalized runtime master switch (todo.md:36).
// The nil value keeps the 默认集成 contract: the plugin host is built in
// and only an explicit pluginsEnabled: false turns it off.
func (s Synthetics) PluginsOn() bool { return s.PluginsEnabled == nil || *s.PluginsEnabled }

type Target struct {
	Name           string        `yaml:"name"`
	URL            string        `yaml:"url"`
	Method         string        `yaml:"method"`
	Timeout        time.Duration `yaml:"timeout"`
	ExpectedStatus int           `yaml:"expectedStatus"`
	BodyContains   string        `yaml:"bodyContains"`
	// Plugin, when set, routes this target through the external probe
	// plugin runtime (go-plugin local gRPC) instead of the native HTTP
	// probe. The HTTP fields stay mandatory: they are the fallback probe
	// parameters used when the plugin side degrades (see pluginhost).
	Plugin string `yaml:"plugin"`
}

func (t *Target) UnmarshalYAML(n *yaml.Node) error {
	var raw struct {
		Name           string `yaml:"name"`
		URL            string `yaml:"url"`
		Method         string `yaml:"method"`
		Timeout        string `yaml:"timeout"`
		ExpectedStatus int    `yaml:"expectedStatus"`
		BodyContains   string `yaml:"bodyContains"`
		Plugin         string `yaml:"plugin"`
	}
	if err := n.Decode(&raw); err != nil {
		return err
	}
	d, err := time.ParseDuration(raw.Timeout)
	if err != nil {
		return fmt.Errorf("timeout: %w", err)
	}
	*t = Target{Name: raw.Name, URL: raw.URL, Method: raw.Method, Timeout: d, ExpectedStatus: raw.ExpectedStatus, BodyContains: raw.BodyContains, Plugin: raw.Plugin}
	return nil
}

func Load(path string) (Config, error) {
	var c Config
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	if e = yaml.Unmarshal(b, &c); e != nil {
		return c, e
	}
	return c, c.Validate()
}
func (c Config) Validate() error {
	for key, v := range map[string]string{"agent.id": c.Agent.ID, "agent.serverAddr": c.Agent.ServerAddr, "agent.gameId": c.Agent.GameID, "agent.env": c.Agent.Env} {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	if len(c.Synthetics.Targets) == 0 {
		return fmt.Errorf("synthetics.targets must explicitly configure at least one target")
	}
	if c.Synthetics.Interval <= 0 {
		return fmt.Errorf("synthetics.interval must be positive")
	}
	// Plugin runtime knobs: negative values are configuration mistakes,
	// unset (zero) values select the plugin-host defaults.
	k := c.Synthetics
	for name, bad := range map[string]bool{
		"pluginMaxConcurrent": k.PluginMaxConcurrent < 0,
		"pluginQueueWindow":   k.PluginQueueWindow < 0,
		"pluginBackoff":       k.PluginBackoff < 0,
		"pluginBackoffMax":    k.PluginBackoffMax < 0,
		"pluginBreakAfter":    k.PluginBreakAfter < 0,
		"pluginOpenWindow":    k.PluginOpenWindow < 0,
	} {
		if bad {
			return fmt.Errorf("synthetics.%s must not be negative", name)
		}
	}
	if err := validatePluginBuild(c); err != nil {
		return err
	}
	seen := map[string]bool{}
	plugins := map[string]bool{}
	for i, p := range c.Synthetics.Plugins {
		if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Command) == "" {
			return fmt.Errorf("synthetics.plugins[%d] name and command are required", i)
		}
		if plugins[p.Name] {
			return fmt.Errorf("duplicate plugin %q", p.Name)
		}
		plugins[p.Name] = true
		if p.Timeout <= 0 {
			return fmt.Errorf("plugin %q requires positive timeout", p.Name)
		}
	}
	for i, t := range c.Synthetics.Targets {
		if strings.TrimSpace(t.Name) == "" || strings.TrimSpace(t.URL) == "" {
			return fmt.Errorf("synthetics.targets[%d] name and url are required", i)
		}
		if seen[t.Name] {
			return fmt.Errorf("duplicate target %q", t.Name)
		}
		seen[t.Name] = true
		if t.Method != "GET" && t.Method != "HEAD" {
			return fmt.Errorf("target %q method must be GET or HEAD", t.Name)
		}
		if t.Timeout <= 0 || t.ExpectedStatus < 100 || t.ExpectedStatus > 599 {
			return fmt.Errorf("target %q requires positive timeout and valid expectedStatus", t.Name)
		}
		if t.Plugin != "" && !plugins[t.Plugin] {
			return fmt.Errorf("target %q references undeclared plugin %q", t.Name, t.Plugin)
		}
	}
	return nil
}
