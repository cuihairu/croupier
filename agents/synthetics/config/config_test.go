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

func TestValidateRequiresExplicitTargetCategory(t *testing.T) {
	c := valid()
	c.Synthetics.Targets[0].TargetCategory = ""
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "targetCategory must be game|third_party") {
		t.Fatalf("blank target category accepted: %v", err)
	}

	c = valid()
	c.Synthetics.Targets[0].TargetCategory = "partner"
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "targetCategory must be game|third_party") {
		t.Fatalf("unknown target category accepted: %v", err)
	}
}

func TestValidatePluginDeclarationsAndTargetRefs(t *testing.T) {
	declared := func() Config {
		c := valid()
		c.Synthetics.Plugins = []PluginConfig{{Name: "echo", Command: "./echo", Timeout: time.Second}}
		return c
	}
	c := declared()
	c.Synthetics.Targets = []Target{{Name: "p", URL: "http://127.0.0.1", Method: "GET", Timeout: 1, ExpectedStatus: 200, Plugin: "echo", TargetCategory: "game"}}
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

func TestParseTargetsJSON_RoundTrip(t *testing.T) {
	json := `[{"name":"web","url":"http://127.0.0.1","method":"GET","timeout":"3s","expectedStatus":200,"targetCategory":"game","secretEnv":["TOKEN"]}]`
	targets, err := ParseTargetsJSON(json)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("len = %d", len(targets))
	}
	if targets[0].Name != "web" || targets[0].Timeout != 3*time.Second || targets[0].TargetCategory != "game" || len(targets[0].SecretEnv) != 1 {
		t.Fatalf("target = %+v", targets[0])
	}
	out, err := MarshalTargetsJSON(targets)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if out == "" {
		t.Fatal("marshal returned empty")
	}
}

func TestParseTargetsJSON_EmptyAndBad(t *testing.T) {
	if _, err := ParseTargetsJSON(""); err == nil {
		t.Fatal("empty payload must error")
	}
	if _, err := ParseTargetsJSON(`not-json`); err == nil {
		t.Fatal("bad json must error")
	}
	if _, err := ParseTargetsJSON(`[{"name":"web","timeout":"bad"}]`); err == nil {
		t.Fatal("bad duration must error")
	}
}

func TestValidateTargets_RejectsBadPull(t *testing.T) {
	cfg := Config{Synthetics: Synthetics{Interval: time.Second, Targets: []Target{{Name: "local", URL: "http://127.0.0.1", Method: "GET", Timeout: time.Second, ExpectedStatus: 200, TargetCategory: "game"}}}}
	bad := []Target{{Name: "x", URL: "http://127.0.0.1", Method: "GET", Timeout: time.Second, ExpectedStatus: 200, TargetCategory: "bogus"}}
	if err := cfg.Synthetics.ValidateTargets(bad); err == nil {
		t.Fatal("bad category must be rejected")
	}
	dup := []Target{
		{Name: "x", URL: "http://127.0.0.1", Method: "GET", Timeout: time.Second, ExpectedStatus: 200, TargetCategory: "game"},
		{Name: "x", URL: "http://127.0.0.1", Method: "GET", Timeout: time.Second, ExpectedStatus: 200, TargetCategory: "game"},
	}
	if err := cfg.Synthetics.ValidateTargets(dup); err == nil {
		t.Fatal("duplicate target must be rejected")
	}
	// Pulled target referencing an undeclared plugin is rejected.
	refPlugin := []Target{{Name: "x", URL: "http://127.0.0.1", Method: "GET", Timeout: time.Second, ExpectedStatus: 200, TargetCategory: "game", Plugin: "nope"}}
	if err := cfg.Synthetics.ValidateTargets(refPlugin); err == nil {
		t.Fatal("undeclared plugin ref must be rejected")
	}
}
