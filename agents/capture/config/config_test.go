package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "capture-agent.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

const validYAML = `
agent:
  id: capture-01
  serverAddr: 127.0.0.1:19090
  gameId: demo
  env: prod
  heartbeatFile: data/capture/agent-heartbeat
outboundTLS:
  enabled: true
  caFile: /etc/certs/ca.pem
capture:
  enabled: true
  bookmarkDir: data/capture/bookmarks
  bookmarkFlushIntervalSec: 3
  mysql:
    host: 127.0.0.1
    port: 3306
    user: repl
    password: secret
    serverId: 4242
    tables:
      game.items: true
`

func TestLoadValid(t *testing.T) {
	cfg, err := Load(writeCfg(t, validYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Agent.ID != "capture-01" || cfg.Agent.ServerAddr != "127.0.0.1:19090" ||
		cfg.Agent.GameID != "demo" || cfg.Agent.Env != "prod" ||
		cfg.Agent.HeartbeatFile != "data/capture/agent-heartbeat" {
		t.Fatalf("agent block mismatch: %+v", cfg.Agent)
	}
	if !cfg.OutboundTLS.Enabled || cfg.OutboundTLS.CAFile != "/etc/certs/ca.pem" {
		t.Fatalf("outboundTLS mismatch: %+v", cfg.OutboundTLS)
	}
	if !cfg.Capture.Enabled || cfg.Capture.MySQL == nil {
		t.Fatalf("capture block mismatch: %+v", cfg.Capture)
	}
	if cfg.Capture.MySQL.Host != "127.0.0.1" || cfg.Capture.MySQL.Port != 3306 ||
		cfg.Capture.MySQL.User != "repl" || cfg.Capture.MySQL.ServerID != 4242 ||
		!cfg.Capture.MySQL.Tables["game.items"] {
		t.Fatalf("mysql block mismatch: %+v", cfg.Capture.MySQL)
	}
}

func TestLoadDefaultsDisabledAndIntervals(t *testing.T) {
	cfg, err := Load(writeCfg(t, `
agent:
  id: capture-01
  serverAddr: 127.0.0.1:19090
  gameId: demo
  env: prod
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Capture.Enabled {
		t.Fatal("capture.enabled must default to false")
	}
	if got := cfg.FlushInterval(); got != 5*time.Second {
		t.Fatalf("FlushInterval default = %v, want 5s", got)
	}
	if got := cfg.BookmarkPath(); got != "data/capture/bookmarks/mysql.json" {
		t.Fatalf("BookmarkPath default = %q", got)
	}
	if got := cfg.FlushInterval(); got <= 0 {
		t.Fatal("FlushInterval must be positive")
	}
}

func TestLoadFlushIntervalOverride(t *testing.T) {
	cfg, err := Load(writeCfg(t, `
agent: {id: a, serverAddr: s:1, gameId: g, env: prod}
capture:
  bookmarkFlushIntervalSec: 9
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.FlushInterval(); got != 9*time.Second {
		t.Fatalf("FlushInterval = %v, want 9s", got)
	}
	if got := cfg.BookmarkPath(); got != "data/capture/bookmarks/mysql.json" {
		t.Fatalf("BookmarkPath = %q", got)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestValidateRequiredFields(t *testing.T) {
	cases := map[string]string{
		"missing id":         "agent: {serverAddr: s:1, gameId: g}",
		"missing serverAddr": "agent: {id: a, gameId: g}",
		"missing gameId":     "agent: {id: a, serverAddr: s:1}",
	}
	for name, body := range cases {
		cfg, err := Load(writeCfg(t, body))
		if err == nil {
			t.Fatalf("%s: expected validation error", name)
		}
		_ = cfg
	}
}

func TestValidateEnabledRequiresMySQLAndTables(t *testing.T) {
	if _, err := Load(writeCfg(t, `
agent: {id: a, serverAddr: s:1, gameId: g, env: prod}
capture: {enabled: true}
`)); err == nil {
		t.Fatal("expected error: capture.enabled without mysql")
	}
	if _, err := Load(writeCfg(t, `
agent: {id: a, serverAddr: s:1, gameId: g, env: prod}
capture:
  enabled: true
  mysql: {host: h, port: 3306, user: u, serverId: 1}
`)); err == nil {
		t.Fatal("expected error: empty tables whitelist")
	}
}

// canonicalYAML sourceType/dsnEnv 契约面配置（凭据只走环境变量引用）。
const canonicalYAML = `
agent: {id: capture-01, serverAddr: 127.0.0.1:19090, gameId: demo, env: prod}
capture:
  enabled: true
  bookmarkDir: data/capture/bookmarks
  sourceType: postgres
  dsnEnv: CROUPIER_CAPTURE_PG_DSN
  options:
    tables: game.items, game.coin
    slotName: croupier_slot
`

func TestLoadCanonicalSourceConfig(t *testing.T) {
	cfg, err := Load(writeCfg(t, canonicalYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Capture.SourceType != "postgres" || cfg.Capture.DSNEnv != "CROUPIER_CAPTURE_PG_DSN" {
		t.Fatalf("capture source block = %+v", cfg.Capture)
	}
	tables := cfg.Capture.Tables()
	if len(tables) != 2 || tables[0] != "game.coin" || tables[1] != "game.items" {
		t.Fatalf("Tables() = %v, want sorted [game.coin game.items]", tables)
	}
	sc := cfg.Capture.SourceConfig("cap-1")
	if sc.SourceID != "cap-1" || sc.SourceType != "postgres" ||
		sc.DSNEnv != "CROUPIER_CAPTURE_PG_DSN" || sc.Options["slotName"] != "croupier_slot" {
		t.Fatalf("SourceConfig = %+v", sc)
	}
	if got := cfg.BookmarkPath(); got != "data/capture/bookmarks/postgres.json" {
		t.Fatalf("BookmarkPath = %q", got)
	}
}

// legacy 配置面：Tables 排序输出；SourceConfig 不携带凭据通道（凭据留 YAML
// 源实现内部，不进契约）；位点文件与 canonical mysql 同名（迁移不丢续读）。
func TestLegacySourceConfigShape(t *testing.T) {
	cfg, err := Load(writeCfg(t, validYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tables := cfg.Capture.Tables()
	if len(tables) != 1 || tables[0] != "game.items" {
		t.Fatalf("Tables() = %v", tables)
	}
	sc := cfg.Capture.SourceConfig("cap-1")
	if sc.SourceID != "cap-1" || sc.SourceType != "mysql" || sc.DSNEnv != "" || sc.Options != nil {
		t.Fatalf("legacy SourceConfig must not carry credential channel: %+v", sc)
	}
	if got := cfg.BookmarkPath(); got != "data/capture/bookmarks/mysql.json" {
		t.Fatalf("BookmarkPath = %q", got)
	}
}

func TestValidateCanonicalSourceConfig(t *testing.T) {
	cases := map[string]string{
		"missing sourceType": `
agent: {id: a, serverAddr: s:1, gameId: g, env: prod}
capture:
  enabled: true
  dsnEnv: D
  options: {tables: a.b}
`,
		"unknown sourceType": `
agent: {id: a, serverAddr: s:1, gameId: g, env: prod}
capture:
  enabled: true
  sourceType: oracle
  dsnEnv: D
  options: {tables: a.b}
`,
		"missing dsnEnv": `
agent: {id: a, serverAddr: s:1, gameId: g, env: prod}
capture:
  enabled: true
  sourceType: mysql
  options: {tables: a.b}
`,
		"empty tables": `
agent: {id: a, serverAddr: s:1, gameId: g, env: prod}
capture:
  enabled: true
  sourceType: postgres
  dsnEnv: D
`,
	}
	for name, body := range cases {
		if _, err := Load(writeCfg(t, body)); err == nil {
			t.Fatalf("%s: expected validation error", name)
		}
	}
	// mongodb 受纳（闭集扩展随 Connector 落地）：canonical 配置通过校验。
	ok := `
agent: {id: a, serverAddr: s:1, gameId: g, env: prod}
capture:
  enabled: true
  sourceType: mongodb
  dsnEnv: TEST_MONGO_DSN
  options: {tables: game.players}
`
	if cfg, err := Load(writeCfg(t, ok)); err != nil {
		t.Fatalf("mongodb config should load: %v", err)
	} else if got := cfg.BookmarkPath(); got != "data/capture/bookmarks/mongodb.json" {
		t.Fatalf("mongodb BookmarkPath = %q", got)
	}
}
