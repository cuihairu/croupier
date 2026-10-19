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
type Synthetics struct {
	Interval time.Duration `yaml:"interval"`
	Targets  []Target      `yaml:"targets"`
}

func (s *Synthetics) UnmarshalYAML(n *yaml.Node) error {
	var raw struct {
		Interval string   `yaml:"interval"`
		Targets  []Target `yaml:"targets"`
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
	return nil
}

type Target struct {
	Name           string        `yaml:"name"`
	URL            string        `yaml:"url"`
	Method         string        `yaml:"method"`
	Timeout        time.Duration `yaml:"timeout"`
	ExpectedStatus int           `yaml:"expectedStatus"`
	BodyContains   string        `yaml:"bodyContains"`
}

func (t *Target) UnmarshalYAML(n *yaml.Node) error {
	var raw struct {
		Name           string `yaml:"name"`
		URL            string `yaml:"url"`
		Method         string `yaml:"method"`
		Timeout        string `yaml:"timeout"`
		ExpectedStatus int    `yaml:"expectedStatus"`
		BodyContains   string `yaml:"bodyContains"`
	}
	if err := n.Decode(&raw); err != nil {
		return err
	}
	d, err := time.ParseDuration(raw.Timeout)
	if err != nil {
		return fmt.Errorf("timeout: %w", err)
	}
	*t = Target{Name: raw.Name, URL: raw.URL, Method: raw.Method, Timeout: d, ExpectedStatus: raw.ExpectedStatus, BodyContains: raw.BodyContains}
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
	}
	return nil
}
