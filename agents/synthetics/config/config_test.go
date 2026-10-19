//go:build !no_go_plugin

package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateRequiresExplicitAgentScope(t *testing.T) {
	for _, field := range []string{"agent_id", "game_id", "env"} {
		c := valid()
		switch field {
		case "agent_id":
			c.Agent.ID = ""
		case "game_id":
			c.Agent.GameID = ""
		case "env":
			c.Agent.Env = ""
		}
		if err := c.Validate(); err == nil {
			t.Errorf("missing %s accepted", field)
		}
	}
}
func TestValidateRequiresExplicitTargets(t *testing.T) {
	c := valid()
	c.Synthetics.Targets = nil
	if err := c.Validate(); err == nil {
		t.Fatal("empty targets accepted")
	}
}

func TestValidatePluginDeclarationsAndTargetRefs(t *testing.T) {
	declared := func() Config {
		c := valid()
		c.Synthetics.Plugins = []PluginConfig{{Name: "echo", Command: "./echo", Timeout: time.Second}}
		return c
	}
	c := declared()
	c.Synthetics.Targets = []Target{{Name: "p", URL: "http://127.0.0.1", Method: "GET", Timeout: 1, ExpectedStatus: 200, Plugin: "echo"}}
	if err := c.Validate(); err != nil {
		t.Fatalf("declared plugin target rejected: %v", err)
	}
	c.Synthetics.Targets[0].Plugin = "ghost"
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "undeclared plugin") {
		t.Fatalf("undeclared plugin ref accepted: %v", err)
	}
	c = declared()
	c.Synthetics.Plugins[0].Command = ""
	if err := c.Validate(); err == nil {
		t.Fatal("plugin without command accepted")
	}
	c = declared()
	c.Synthetics.Plugins = append(c.Synthetics.Plugins, PluginConfig{Name: "echo", Command: "./dup", Timeout: time.Second})
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate plugin") {
		t.Fatalf("duplicate plugin accepted: %v", err)
	}
	c = declared()
	c.Synthetics.Plugins[0].Timeout = 0
	if err := c.Validate(); err == nil {
		t.Fatal("plugin without timeout accepted")
	}
}
