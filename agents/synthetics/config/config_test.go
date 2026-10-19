package config

import "testing"

func valid() Config {
	return Config{Agent: Agent{ID: "a", ServerAddr: "s", GameID: "g", Env: "e"}, Synthetics: Synthetics{Interval: 1, Targets: []Target{{Name: "t", URL: "http://127.0.0.1", Method: "GET", Timeout: 1, ExpectedStatus: 200}}}}
}
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
