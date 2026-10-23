// Credential-reference validation for synthetics targets (todo.md:36
// 凭证边界): config carries env var NAMES only, never secret values.
// Compiled in both builds — the validation lives in the shared config
// package, ahead of any plugin host.
package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSecretEnvValidation: non-blank, duplicate-free env var names; a
// secret value has no field to hide in.
func TestSecretEnvValidation(t *testing.T) {
	ok := valid()
	ok.Synthetics.Targets[0].SecretEnv = []string{"GAME_TOKEN", "DB_PASSWORD"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid secretEnv rejected: %v", err)
	}

	blank := valid()
	blank.Synthetics.Targets[0].SecretEnv = []string{"  "}
	if err := blank.Validate(); err == nil || !strings.Contains(err.Error(), "non-blank") {
		t.Fatalf("blank secretEnv name accepted: %v", err)
	}

	dup := valid()
	dup.Synthetics.Targets[0].SecretEnv = []string{"GAME_TOKEN", "GAME_TOKEN"}
	if err := dup.Validate(); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("duplicate secretEnv name accepted: %v", err)
	}
}

// TestSecretEnvYamlParsing: the yaml key is secretEnv and round-trips
// into the Target field through the custom unmarshaler.
func TestSecretEnvYamlParsing(t *testing.T) {
	var c Config
	in := `
agent: {id: a, serverAddr: s, gameId: g, env: e}
synthetics:
  interval: 1s
  targets:
    - name: t
      url: http://127.0.0.1
      method: GET
      timeout: 1s
      expectedStatus: 200
      targetCategory: game
      secretEnv: [GAME_TOKEN, DB_PASSWORD]
`
	if err := yaml.Unmarshal([]byte(in), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := c.Synthetics.Targets[0].SecretEnv
	if len(got) != 2 || got[0] != "GAME_TOKEN" || got[1] != "DB_PASSWORD" {
		t.Fatalf("secretEnv round trip: %v", got)
	}
}
