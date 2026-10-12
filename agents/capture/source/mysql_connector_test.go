package source

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/connector"
)

// 行像键化：位置下标键（ColumnsMapped=false 语义），空/坏 JSON 不伪造。
func TestPositionImage(t *testing.T) {
	img := positionImage(json.RawMessage(`[1,"gold",null]`))
	if len(img) != 3 || string(img["0"]) != "1" || string(img["1"]) != `"gold"` || string(img["2"]) != "null" {
		t.Fatalf("image = %v", img)
	}
	if positionImage(nil) != nil {
		t.Fatal("nil image should stay nil")
	}
	if positionImage(json.RawMessage(`"scalar"`)) != nil {
		t.Fatal("non-array image should be nil, not fabricated")
	}
	if positionImage(json.RawMessage(`[1]`)) == nil {
		t.Fatal("valid image lost")
	}
}

// 操作闭集互转 + 未知操作暴露（不静默映射）。
func TestConnectorOperation(t *testing.T) {
	tests := map[Op]connector.Operation{
		OpInsert: connector.OpInsert,
		OpUpdate: connector.OpUpdate,
		OpDelete: connector.OpDelete,
	}
	for src, want := range tests {
		got, err := connectorOperation(src)
		if err != nil || got != want {
			t.Fatalf("op %v = %v, %v", src, got, err)
		}
	}
	if _, err := connectorOperation(Op("truncate")); err == nil {
		t.Fatal("unknown source op accepted")
	}
}

// 契约映射：source.ChangeEvent → connector.ChangeEvent 字段对齐（位点
// kind=gtid、行像位置键化、账号/时间戳保留）。
func TestMySQLConnectorToContract(t *testing.T) {
	t.Setenv("CROUPIER_CAPTURE_MYSQL_DSN_TEST", "u:p@tcp(h:3306)/")
	c := NewMySQLConnector(connector.SourceConfig{SourceID: "mysql-itest", SourceType: "mysql"}, nil)
	c.cfg = connector.SourceConfig{SourceID: "mysql-itest", SourceType: "mysql"}
	c.src = &MySQLSource{}

	ce, err := c.toContract(ChangeEvent{
		SourceType: "mysql",
		Database:   "game",
		Table:      "items",
		Op:         OpUpdate,
		BeforeJSON: json.RawMessage(`[1,"copper"]`),
		AfterJSON:  json.RawMessage(`[1,"gold"]`),
		GTID:       "0-1-5",
		Account:    "probe_user",
		TsUnixMs:   1748736000000,
	})
	if err != nil {
		t.Fatalf("toContract: %v", err)
	}
	if ce.SourceID != "mysql-itest" || ce.SourceType != "mysql" || ce.Database != "game" || ce.Table != "items" {
		t.Fatalf("identity = %+v", ce)
	}
	if ce.Operation != connector.OpUpdate {
		t.Fatalf("op = %v", ce.Operation)
	}
	if ce.ColumnsMapped {
		t.Fatal("mysql binlog rows are positional, columnsMapped must be false")
	}
	if string(ce.Before["0"]) != "1" || string(ce.Before["1"]) != `"copper"` || len(ce.Before) != 2 {
		t.Fatalf("before = %v", ce.Before)
	}
	if string(ce.After["1"]) != `"gold"` {
		t.Fatalf("after = %v", ce.After)
	}
	if ce.Position.Kind != "gtid" || ce.Position.Value != "0-1-5" || !ce.Position.Valid() {
		t.Fatalf("position = %+v", ce.Position)
	}
	if ce.Account != "probe_user" || ce.TsUnixMs != 1748736000000 {
		t.Fatalf("account/ts = %q/%d", ce.Account, ce.TsUnixMs)
	}
	// 契约自检门槛：桥接产出的事件须过 connector.ChangeEvent.Validate。
	if err := ce.Validate(); err != nil {
		t.Fatalf("bridged event fails contract validation: %v", err)
	}
}

// 契约守卫：Open 前拒绝（不静默空转）；Snapshot 能力缺失显式报错。
func TestMySQLConnectorGuards(t *testing.T) {
	c := NewMySQLConnector(connector.SourceConfig{SourceID: "x"}, nil)
	if _, err := c.Discover(t.Context(), connector.DiscoveryRequest{}); err == nil {
		t.Fatal("discover before open accepted")
	}
	if _, err := c.Poll(t.Context(), connector.PollRequest{}); err == nil {
		t.Fatal("poll before open accepted")
	}
	if _, err := c.Checkpoint(t.Context()); err == nil {
		t.Fatal("checkpoint before open accepted")
	}
	if _, err := c.Snapshot(t.Context(), connector.SnapshotRequest{}); !errors.Is(err, connector.ErrUnsupportedSnapshot) {
		t.Fatalf("snapshot = %v, want ErrUnsupportedSnapshot", err)
	}
	if err := c.Close(t.Context()); err != nil {
		t.Fatalf("close before open = %v", err)
	}
}

// Open 凭据与参数解析：只走 DSNEnv 通道，serverId/tables 必填。
func TestMySQLConnectorOpen(t *testing.T) {
	const env = "CROUPIER_CAPTURE_MYSQL_DSN_TEST"
	t.Setenv(env, "probe_user:probe_pass@tcp(127.0.0.1:3306)/")

	// dsnEnv 缺失。
	if err := NewMySQLConnector(connector.SourceConfig{}, nil).Open(t.Context(), connector.SourceConfig{}); err == nil {
		t.Fatal("open without dsnEnv accepted")
	}
	// 环境变量未设置（错误只提变量名，不回显取值）。
	if err := NewMySQLConnector(connector.SourceConfig{DSNEnv: env + "_MISSING"}, nil).
		Open(t.Context(), connector.SourceConfig{DSNEnv: env + "_MISSING"}); err == nil ||
		!contains(err.Error(), env+"_MISSING") {
		t.Fatalf("missing env: %v", err)
	}
	// DSN 解析失败。
	t.Setenv(env, "bad@@dsn")
	if err := NewMySQLConnector(connector.SourceConfig{DSNEnv: env, Options: map[string]string{"serverId": "1", "tables": "a.b"}}, nil).
		Open(t.Context(), connector.SourceConfig{DSNEnv: env, Options: map[string]string{"serverId": "1", "tables": "a.b"}}); err == nil {
		t.Fatal("malformed dsn accepted")
	}
	t.Setenv(env, "u:p@tcp(127.0.0.1:bad)/")
	if err := NewMySQLConnector(connector.SourceConfig{DSNEnv: env, Options: map[string]string{"serverId": "1", "tables": "a.b"}}, nil).
		Open(t.Context(), connector.SourceConfig{DSNEnv: env, Options: map[string]string{"serverId": "1", "tables": "a.b"}}); err == nil {
		t.Fatal("bad port accepted")
	}
	// serverId 必填；tables 必填。
	t.Setenv(env, "probe_user:probe_pass@tcp(127.0.0.1:3307)/")
	for _, opts := range []map[string]string{
		{"tables": "a.b"},
		{"serverId": "1"},
		{"serverId": "0", "tables": "a.b"},
	} {
		cfg := connector.SourceConfig{DSNEnv: env, Options: opts}
		if err := NewMySQLConnector(cfg, nil).Open(t.Context(), cfg); err == nil {
			t.Fatalf("open accepted incomplete options %v", opts)
		}
	}
}

// Open 成功路径：DSN 解析结果落到 MySQLSource 配置（不建连接）。
func TestMySQLConnectorOpenSuccess(t *testing.T) {
	const env = "CROUPIER_CAPTURE_MYSQL_DSN_TEST"
	t.Setenv(env, "probe_user:probe_pass@tcp(cdc-host:3307)/")
	cfg := connector.SourceConfig{
		SourceID:   "mysql-itest",
		SourceType: "mysql",
		DSNEnv:     env,
		Options: map[string]string{
			"serverId": "424242",
			"flavor":   "mariadb",
			"tables":   "game.items, game.coin ",
		},
	}
	c := NewMySQLConnector(cfg, nil)
	if err := c.Open(t.Context(), cfg); err != nil {
		t.Fatalf("open: %v", err)
	}
	if c.cfg.SourceID != "mysql-itest" {
		t.Fatalf("cfg not stored: %+v", c.cfg)
	}
	pos, err := c.Checkpoint(t.Context())
	if err != nil {
		t.Fatalf("checkpoint after open: %v", err)
	}
	if pos.Kind != "gtid" || pos.Value != "" {
		t.Fatalf("empty bookmark position = %+v", pos)
	}
	if err := c.Close(t.Context()); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
