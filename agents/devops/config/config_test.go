package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "devops-agent.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const validBody = `
agent:
  id: devops-01
  serverAddr: "127.0.0.1:19090"
  gameId: demo
  env: prod
devops:
  enabled: true
  allowlist:
    - host: 10.0.1.20
      ports: [22]
      protocols: [ssh]
`

func TestLoadValid(t *testing.T) {
	cfg, err := Load(writeCfg(t, validBody))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.Devops.Enabled || len(cfg.Devops.Allowlist) != 1 {
		t.Fatalf("cfg = %+v", cfg.Devops)
	}
	if cfg.SessionDir() != "data/devops/sessions" || cfg.ExeclogDir() != "data/devops/execlog" {
		t.Fatalf("defaults = %s / %s", cfg.SessionDir(), cfg.ExeclogDir())
	}
}

func TestLoadOverridesDefaults(t *testing.T) {
	body := validBody + `
  sessionDir: /tmp/s
  execlogDir: /tmp/e
`
	cfg, err := Load(writeCfg(t, body))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.SessionDir() != "/tmp/s" || cfg.ExeclogDir() != "/tmp/e" {
		t.Fatalf("dirs = %s / %s", cfg.SessionDir(), cfg.ExeclogDir())
	}
}

func TestValidateRequired(t *testing.T) {
	base := Config{Agent: Agent{ID: "a", ServerAddr: "s", GameID: "g"}}
	if err := base.Validate(); err != nil {
		t.Fatalf("minimal valid: %v", err)
	}

	noID := Config{Agent: Agent{ServerAddr: "s", GameID: "g"}}
	if err := noID.Validate(); err == nil {
		t.Fatal("missing id accepted")
	}
	noAddr := Config{Agent: Agent{ID: "a", GameID: "g"}}
	if err := noAddr.Validate(); err == nil {
		t.Fatal("missing serverAddr accepted")
	}
	noGame := Config{Agent: Agent{ID: "a", ServerAddr: "s"}}
	if err := noGame.Validate(); err == nil {
		t.Fatal("missing gameId accepted")
	}
}

func TestValidateEnabledRequiresAllowlist(t *testing.T) {
	enabledNoList := Config{
		Agent:  Agent{ID: "a", ServerAddr: "s", GameID: "g"},
		Devops: Devops{Enabled: true},
	}
	if err := enabledNoList.Validate(); err == nil {
		t.Fatal("enabled without allowlist accepted")
	}

	badPort := Config{
		Agent: Agent{ID: "a", ServerAddr: "s", GameID: "g"},
		Devops: Devops{Enabled: true, Allowlist: []AllowHost{
			{Host: "h", Ports: []int{0}, Protocols: []string{"ssh"}},
		}},
	}
	if err := badPort.Validate(); err == nil {
		t.Fatal("invalid port accepted")
	}

	noProtocols := Config{
		Agent: Agent{ID: "a", ServerAddr: "s", GameID: "g"},
		Devops: Devops{Enabled: true, Allowlist: []AllowHost{
			{Host: "h", Ports: []int{22}},
		}},
	}
	if err := noProtocols.Validate(); err == nil {
		t.Fatal("empty protocols accepted")
	}

	disabled := Config{
		Agent:  Agent{ID: "a", ServerAddr: "s", GameID: "g"},
		Devops: Devops{Enabled: false},
	}
	if err := disabled.Validate(); err != nil {
		t.Fatalf("disabled with empty allowlist should pass: %v", err)
	}
}
