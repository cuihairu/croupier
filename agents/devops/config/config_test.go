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
  ssh:
    user: ops
    privateKeyFile: /tmp/id_ed25519
    knownHostsFile: /tmp/known_hosts
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
	base := Config{Agent: Agent{ID: "a", ServerAddr: "s", GameID: "g", Env: "prod"}}
	if err := base.Validate(); err != nil {
		t.Fatalf("minimal valid: %v", err)
	}

	noID := Config{Agent: Agent{ServerAddr: "s", GameID: "g", Env: "prod"}}
	if err := noID.Validate(); err == nil {
		t.Fatal("missing id accepted")
	}
	noAddr := Config{Agent: Agent{ID: "a", GameID: "g", Env: "prod"}}
	if err := noAddr.Validate(); err == nil {
		t.Fatal("missing serverAddr accepted")
	}
	noGame := Config{Agent: Agent{ID: "a", ServerAddr: "s", Env: "prod"}}
	if err := noGame.Validate(); err == nil {
		t.Fatal("missing gameId accepted")
	}
	noEnv := Config{Agent: Agent{ID: "a", ServerAddr: "s", GameID: "g"}}
	if err := noEnv.Validate(); err == nil {
		t.Fatal("missing env accepted")
	}
	blankEnv := Config{Agent: Agent{ID: "a", ServerAddr: "s", GameID: "g", Env: "  "}}
	if err := blankEnv.Validate(); err == nil {
		t.Fatal("blank env accepted")
	}
}

func TestValidateEnabledRequiresAllowlist(t *testing.T) {
	enabledNoList := Config{
		Agent:  Agent{ID: "a", ServerAddr: "s", GameID: "g", Env: "prod"},
		Devops: Devops{Enabled: true},
	}
	if err := enabledNoList.Validate(); err == nil {
		t.Fatal("enabled without allowlist accepted")
	}

	badPort := Config{
		Agent: Agent{ID: "a", ServerAddr: "s", GameID: "g", Env: "prod"},
		Devops: Devops{Enabled: true, Allowlist: []AllowHost{
			{Host: "h", Ports: []int{0}, Protocols: []string{"ssh"}},
		}},
	}
	if err := badPort.Validate(); err == nil {
		t.Fatal("invalid port accepted")
	}

	noProtocols := Config{
		Agent: Agent{ID: "a", ServerAddr: "s", GameID: "g", Env: "prod"},
		Devops: Devops{Enabled: true, Allowlist: []AllowHost{
			{Host: "h", Ports: []int{22}},
		}},
	}
	if err := noProtocols.Validate(); err == nil {
		t.Fatal("empty protocols accepted")
	}

	disabled := Config{
		Agent:  Agent{ID: "a", ServerAddr: "s", GameID: "g", Env: "prod"},
		Devops: Devops{Enabled: false},
	}
	if err := disabled.Validate(); err != nil {
		t.Fatalf("disabled with empty allowlist should pass: %v", err)
	}
}

func TestValidateEnabledRequiresSSH(t *testing.T) {
	list := []AllowHost{{Host: "h", Ports: []int{22}, Protocols: []string{"ssh"}}}
	base := Config{
		Agent:  Agent{ID: "a", ServerAddr: "s", GameID: "g", Env: "prod"},
		Devops: Devops{Enabled: true, Allowlist: list},
	}

	noUser := base
	noUser.Devops.SSH = SSH{PrivateKeyFile: "k", KnownHostsFile: "kh"}
	if err := noUser.Validate(); err == nil {
		t.Fatal("ssh without user accepted")
	}

	noAuth := base
	noAuth.Devops.SSH = SSH{User: "ops", KnownHostsFile: "kh"}
	if err := noAuth.Validate(); err == nil {
		t.Fatal("ssh without credentials accepted")
	}

	noHostKeyPolicy := base
	noHostKeyPolicy.Devops.SSH = SSH{User: "ops", PrivateKeyFile: "k"}
	if err := noHostKeyPolicy.Validate(); err == nil {
		t.Fatal("ssh without host key policy accepted")
	}

	insecureExplicit := base
	insecureExplicit.Devops.SSH = SSH{User: "ops", PrivateKeyFile: "k", InsecureHostKey: true}
	if err := insecureExplicit.Validate(); err != nil {
		t.Fatalf("explicit insecureHostKey should pass: %v", err)
	}

	passwordAuth := base
	passwordAuth.Devops.SSH = SSH{User: "ops", PasswordFile: "p", KnownHostsFile: "kh"}
	if err := passwordAuth.Validate(); err != nil {
		t.Fatalf("password-only auth should pass: %v", err)
	}
}

func TestParseAllowlistJSON(t *testing.T) {
	entries, err := ParseAllowlistJSON([]byte(
		`[{"host":"10.0.1.20","ports":[22,2222],"protocols":["ssh"]},{"host":"DB-01","ports":[3389],"protocols":["rdp"]}]`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(entries) != 2 || entries[1].Host != "DB-01" {
		t.Fatalf("entries = %+v", entries)
	}
	if err := ValidateAllowlist(entries); err != nil {
		t.Fatalf("validate: %v", err)
	}

	if _, err := ParseAllowlistJSON([]byte(`{"host":"x"}`)); err == nil {
		t.Fatal("object payload accepted")
	}
	if _, err := ParseAllowlistJSON([]byte(`[{"host":"x","ports":[22],"protocols":["ssh"],"extra":1}]`)); err == nil {
		t.Fatal("unknown field accepted")
	}
	if err := ValidateAllowlist([]AllowHost{{Host: "x", Ports: []int{22}, Protocols: []string{"telnet"}}}); err == nil {
		t.Fatal("unknown protocol accepted by ValidateAllowlist")
	}
}
