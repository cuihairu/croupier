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
}

func (p *PluginConfig) UnmarshalYAML(n *yaml.Node) error {
	var raw struct {
		Name    string   `yaml:"name"`
		Command string   `yaml:"command"`
		Args    []string `yaml:"args"`
		Timeout string   `yaml:"timeout"`
	}
	if err := n.Decode(&raw); err != nil {
		return err
	}
	d, err := time.ParseDuration(raw.Timeout)
	if err != nil {
		return fmt.Errorf("plugin %q timeout: %w", raw.Name, err)
	}
	*p = PluginConfig{Name: raw.Name, Command: raw.Command, Args: raw.Args, Timeout: d}
	return nil
}

type Synthetics struct {
	Interval time.Duration  `yaml:"interval"`
	Targets  []Target       `yaml:"targets"`
	Plugins  []PluginConfig `yaml:"plugins"`
}

func (s *Synthetics) UnmarshalYAML(n *yaml.Node) error {
	var raw struct {
		Interval string         `yaml:"interval"`
		Targets  []Target       `yaml:"targets"`
		Plugins  []PluginConfig `yaml:"plugins"`
	}
	if err := n.Decode(&raw); err != nil {
		return err
	}
	d, err := time.ParseDuration(raw.Interval)
	if err != nil {
		return fmt.Errorf("interval: %w", err)
	}
	s.Interval = d
	s.Targets = raw.Targets
	s.Plugins = raw.Plugins
	return nil
}

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
