// Package config loads the explicitly configured HTTP checks for synthetics-agent.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cuihairu/croupier/core/agentscope"
	corehealthprobe "github.com/cuihairu/croupier/core/healthprobe"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Agent       Agent      `yaml:"agent"`
	OutboundTLS TLS        `yaml:"outboundTLS"`
	Synthetics  Synthetics `yaml:"synthetics"`
}

const (
	TypeHTTP = "http"
	TypeICMP = "icmp"
)

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
	// Type selects the native implementation for non-plugin targets.
	// Empty and http preserve the existing HTTP behavior; icmp uses a raw
	// ICMP echo probe and does not depend on an HTTP fallback endpoint.
	Type string `yaml:"type"`
	// PayloadJSON carries target parameters to a non-HTTP probe plugin,
	// such as a TCP host/port assertion. It is plugin-specific JSON and
	// is not interpreted by the native HTTP fallback.
	PayloadJSON string `yaml:"payloadJson"`
	// Address is the ICMP host or IP.
	Address string `yaml:"address"`
	// Count is the number of ICMP echo attempts required for success.
	Count int `yaml:"count"`
	// TargetCategory is the required ownership class from todo.md:31:
	// game or third_party. It is independent of protocol and travels with
	// every durable result so operators never guess ownership.
	TargetCategory string `yaml:"targetCategory"`
	// SecretEnv lists environment variable NAMES whose values are the
	// target's credentials (todo.md:36 凭证边界: config carries references,
	// never inline secret values). At check time the plugin host resolves
	// them from its own environment and passes them to the plugin on that
	// one call only; unset or empty references degrade the check with
	// plugin_credential_missing before any plugin process is touched.
	SecretEnv []string `yaml:"secretEnv"`
}

func (t *Target) UnmarshalYAML(n *yaml.Node) error {
	var raw struct {
		Name           string   `yaml:"name"`
		URL            string   `yaml:"url"`
		Method         string   `yaml:"method"`
		Timeout        string   `yaml:"timeout"`
		ExpectedStatus int      `yaml:"expectedStatus"`
		BodyContains   string   `yaml:"bodyContains"`
		Plugin         string   `yaml:"plugin"`
		PayloadJSON    string   `yaml:"payloadJson"`
		Type           string   `yaml:"type"`
		Address        string   `yaml:"address"`
		Count          int      `yaml:"count"`
		TargetCategory string   `yaml:"targetCategory"`
		SecretEnv      []string `yaml:"secretEnv"`
	}
	if err := n.Decode(&raw); err != nil {
		return err
	}
	var d time.Duration
	if raw.Timeout != "" {
		var err error
		d, err = time.ParseDuration(raw.Timeout)
		if err != nil {
			return fmt.Errorf("timeout: %w", err)
		}
	}
	*t = Target{Name: raw.Name, URL: raw.URL, Method: raw.Method, Timeout: d, ExpectedStatus: raw.ExpectedStatus, BodyContains: raw.BodyContains, Plugin: raw.Plugin, PayloadJSON: raw.PayloadJSON, Type: raw.Type, Address: raw.Address, Count: raw.Count, TargetCategory: raw.TargetCategory, SecretEnv: raw.SecretEnv}
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
	// 注册身份四字段走公共校验规则（core/agentscope：去空白即拒、按声明顺序
	// 报第一个缺项——原 map 遍历使首个报错字段随 map 序漂移）。
	if err := agentscope.Identity(c.Agent.ID, c.Agent.ServerAddr, c.Agent.GameID, c.Agent.Env); err != nil {
		return err
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
	return validateTargets(c.Synthetics.Targets, plugins)
}

// validateTargets checks a target list with the same per-target rules as
// Config.Validate (empty name/duplicate, ownership class, http/icmp shape,
// timeout/method/expectedStatus, plugin ref, secretEnv, payloadJson). The
// plugins map carries the declared plugin names; pulled targets reference
// the same local plugin declarations.
func validateTargets(targets []Target, plugins map[string]bool) error {
	seen := map[string]bool{}
	for i, t := range targets {
		if strings.TrimSpace(t.Name) == "" {
			return fmt.Errorf("synthetics.targets[%d] name is required", i)
		}
		switch strings.TrimSpace(t.TargetCategory) {
		case corehealthprobe.TargetCategoryGame, corehealthprobe.TargetCategoryThirdParty:
		default:
			return fmt.Errorf("target %q: targetCategory must be game|third_party", t.Name)
		}
		icmp := strings.EqualFold(strings.TrimSpace(t.Type), TypeICMP)
		if !icmp && strings.TrimSpace(t.URL) == "" {
			return fmt.Errorf("synthetics.targets[%d] type http requires url", i)
		}
		if seen[t.Name] {
			return fmt.Errorf("duplicate target %q", t.Name)
		}
		seen[t.Name] = true
		if !icmp && t.Method != "GET" && t.Method != "HEAD" {
			return fmt.Errorf("target %q method must be GET or HEAD", t.Name)
		}
		if icmp {
			if t.Timeout <= 0 {
				return fmt.Errorf("target %q requires positive timeout", t.Name)
			}
		} else if t.Timeout <= 0 || t.ExpectedStatus < 100 || t.ExpectedStatus > 599 {
			return fmt.Errorf("target %q requires positive timeout and valid expectedStatus", t.Name)
		}
		if t.Plugin != "" && !plugins[t.Plugin] {
			return fmt.Errorf("target %q references undeclared plugin %q", t.Name, t.Plugin)
		}
		secretEnv := map[string]bool{}
		for _, name := range t.SecretEnv {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("target %q secretEnv entries must be non-blank env var names", t.Name)
			}
			if secretEnv[name] {
				return fmt.Errorf("target %q lists env var %q in secretEnv twice", t.Name, name)
			}
			secretEnv[name] = true
		}
		if t.PayloadJSON != "" && !json.Valid([]byte(t.PayloadJSON)) {
			return fmt.Errorf("target %q payloadJson must be valid JSON", t.Name)
		}
		switch strings.ToLower(t.Type) {
		case "", TypeHTTP:
		case TypeICMP:
			if strings.TrimSpace(t.Address) == "" {
				return fmt.Errorf("target %q type icmp requires address", t.Name)
			}
		default:
			return fmt.Errorf("target %q type must be http or icmp", t.Name)
		}
	}
	return nil
}

// ValidateTargets validates a pulled target list against the runner's local
// plugin declarations. Pulled targets cannot add plugins: a target that
// references a plugin name absent from the local declarations is rejected
// wholesale (the caller keeps last-good and retries next round).
func (s Synthetics) ValidateTargets(targets []Target) error {
	plugins := map[string]bool{}
	for _, p := range s.Plugins {
		plugins[p.Name] = true
	}
	return validateTargets(targets, plugins)
}

// targetJSON is the wire JSON shape of one synthetics target (the
// synthetics.targets[] config surface serialized for the config_versions
// payload). Duration fields use string forms ("3s") matching YAML, so the
// panel and the agent agree on the same human-readable shape.
type targetJSON struct {
	Name           string   `json:"name"`
	URL            string   `json:"url,omitempty"`
	Method         string   `json:"method,omitempty"`
	Timeout        string   `json:"timeout,omitempty"`
	ExpectedStatus int      `json:"expectedStatus,omitempty"`
	BodyContains   string   `json:"bodyContains,omitempty"`
	Plugin         string   `json:"plugin,omitempty"`
	PayloadJSON    string   `json:"payloadJson,omitempty"`
	Type           string   `json:"type,omitempty"`
	Address        string   `json:"address,omitempty"`
	Count          int      `json:"count,omitempty"`
	TargetCategory string   `json:"targetCategory"`
	SecretEnv      []string `json:"secretEnv,omitempty"`
}

// ParseTargetsJSON parses a synthetics.targets JSON payload (string
// durations) into config.Target values. Validation is the caller's job
// (ValidateTargets against local plugins).
func ParseTargetsJSON(content string) ([]Target, error) {
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("synthetics targets: empty payload")
	}
	var raw []targetJSON
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("synthetics targets: parse json: %w", err)
	}
	out := make([]Target, 0, len(raw))
	for i, r := range raw {
		var timeout time.Duration
		if r.Timeout != "" {
			d, err := time.ParseDuration(r.Timeout)
			if err != nil {
				return nil, fmt.Errorf("synthetics targets[%d] timeout: %w", i, err)
			}
			timeout = d
		}
		out = append(out, Target{
			Name: r.Name, URL: r.URL, Method: r.Method, Timeout: timeout,
			ExpectedStatus: r.ExpectedStatus, BodyContains: r.BodyContains,
			Plugin: r.Plugin, PayloadJSON: r.PayloadJSON, Type: r.Type,
			Address: r.Address, Count: r.Count, TargetCategory: r.TargetCategory,
			SecretEnv: r.SecretEnv,
		})
	}
	return out, nil
}

// MarshalTargetsJSON serializes targets to the wire JSON shape (string
// durations). Used by the panel publish pre-check and tests.
func MarshalTargetsJSON(targets []Target) (string, error) {
	raw := make([]targetJSON, 0, len(targets))
	for _, t := range targets {
		r := targetJSON{
			Name: t.Name, URL: t.URL, Method: t.Method,
			ExpectedStatus: t.ExpectedStatus, BodyContains: t.BodyContains,
			Plugin: t.Plugin, PayloadJSON: t.PayloadJSON, Type: t.Type,
			Address: t.Address, Count: t.Count, TargetCategory: t.TargetCategory,
			SecretEnv: t.SecretEnv,
		}
		if t.Timeout > 0 {
			r.Timeout = t.Timeout.String()
		}
		raw = append(raw, r)
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
