package agent

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cuihairu/croupier/agents/sidecar/funcplugin"
)

// FunctionPluginConfig declares one external function adapter plugin
// process (todo.md:35). Wire-adjacent shape converted by cmd; Enabled
// nil defaults to enabled (默认集成). Compiled in both builds: the
// no_go_plugin build still parses the config to fail closed on plugin
// declarations.
type FunctionPluginConfig struct {
	Name    string        `json:"name" yaml:"name"`
	Command string        `json:"command" yaml:"command"`
	Args    []string      `json:"args,omitempty" yaml:"args,omitempty"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	Enabled *bool         `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// FunctionPluginsConfig is the functions.plugins face of the sidecar
// config (todo.md:35). Enabled is the runtime master switch — nil
// defaults to true (默认集成, mirroring synthetics.pluginsEnabled);
// false keeps every plugin subprocess never launched and dispatch
// degrades with an explicit plugin_disabled cause. No plugin subprocess
// is launched unless a plugin is declared here.
type FunctionPluginsConfig struct {
	Enabled *bool                  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Plugins []FunctionPluginConfig `json:"plugins,omitempty" yaml:"plugins,omitempty"`
}

// IsEnabled reports the runtime master switch; nil defaults to enabled.
func (c *FunctionPluginsConfig) IsEnabled() bool { return c.Enabled == nil || *c.Enabled }

// IsEnabled reports the per-plugin runtime switch; nil defaults to enabled.
func (c FunctionPluginConfig) IsEnabled() bool { return c.Enabled == nil || *c.Enabled }

// WithFunctionPlugins assembles the function adapter plugin hosting:
// validates the agent scope (fail closed on blank gameId/env — plugin
// functions inherit the sidecar scope and never declare their own),
// starts the declared plugins and wires the dispatch into the local
// handler. The declared functions register through the same function
// chain as SDK functions (declare.Metadata → registration/approval/audit
// chain).
//
// The build-dependent part lives in startFunctionPlugins: the default
// build runs the real go-plugin host, the no_go_plugin build fails
// closed when the config still declares plugins.
func (a *App) WithFunctionPlugins(cfg *FunctionPluginsConfig) error {
	if a == nil || cfg == nil || len(cfg.Plugins) == 0 {
		return nil
	}
	if a.upstream == nil {
		return fmt.Errorf("agent upstream not initialized")
	}
	if err := ValidateAgentScope(a.upstream.GameID(), a.upstream.Env()); err != nil {
		return fmt.Errorf("function plugins: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("function plugins: %w", err)
	}
	enabledPlugins := make([]funcplugin.PluginConfig, 0, len(cfg.Plugins))
	for _, p := range cfg.Plugins {
		if !p.IsEnabled() {
			continue
		}
		enabledPlugins = append(enabledPlugins, funcplugin.PluginConfig{
			Name:    p.Name,
			Command: p.Command,
			Args:    p.Args,
			Timeout: p.Timeout,
		})
	}
	if len(enabledPlugins) == 0 {
		slog.Info("function plugins configured but disabled", "plugins", len(cfg.Plugins))
		return nil
	}
	if !cfg.IsEnabled() {
		slog.Info("function plugins disabled", "plugins", len(cfg.Plugins))
		return nil
	}
	return a.startFunctionPlugins(enabledPlugins)
}

// Validate rejects an invalid functions.plugins declaration before
// assembly: plugin identity and timeouts must be usable and duplicate
// declarations are configuration mistakes. An enabled switch with an
// empty plugin list is NOT an error — it is the declared no-op.
func (c *FunctionPluginsConfig) Validate() error {
	seen := map[string]bool{}
	for i, p := range c.Plugins {
		if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Command) == "" {
			return fmt.Errorf("functions.plugins[%d] name and command are required", i)
		}
		if seen[p.Name] {
			return fmt.Errorf("duplicate function plugin %q", p.Name)
		}
		seen[p.Name] = true
		if p.Timeout <= 0 {
			return fmt.Errorf("function plugin %q requires positive timeout", p.Name)
		}
	}
	return nil
}

// FunctionPlugins returns the wired host (nil when not configured or in
// trimmed builds).
func (a *App) FunctionPlugins() *funcplugin.Host {
	if a == nil {
		return nil
	}
	return a.funcHost
}
