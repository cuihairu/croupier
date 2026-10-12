package source

import (
	"context"
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
func TestBridgeEvent(t *testing.T) {
	ce, err := bridgeEvent("mysql-itest", "mysql", ChangeEvent{
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
		t.Fatalf("bridgeEvent: %v", err)
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

// 身份回落：sourceID 缺省取源类型（契约事件自检要求非空，不静默降为空）。
func TestBridgeEventSourceIDFallback(t *testing.T) {
	ce, err := bridgeEvent("", "mysql", ChangeEvent{
		Database: "game", Table: "items", Op: OpDelete,
		BeforeJSON: json.RawMessage(`[7]`), GTID: "0-1-9", TsUnixMs: 1748736000000,
	})
	if err != nil {
		t.Fatalf("bridgeEvent: %v", err)
	}
	if ce.SourceID != "mysql" || ce.Operation != connector.OpDelete {
		t.Fatalf("event = %+v", ce)
	}
}

// 位点 kind 按源类型（checkpoint 还原真值：mysql=gtid，postgres=lsn）。
func TestPositionKindOf(t *testing.T) {
	if got := PositionKindOf("mysql"); got != "gtid" {
		t.Fatalf("mysql kind = %q", got)
	}
	if got := PositionKindOf("postgres"); got != "lsn" {
		t.Fatalf("postgres kind = %q", got)
	}
	if got := PositionKindOf(""); got != "gtid" {
		t.Fatalf("unknown kind fallback = %q", got)
	}
}

// ProviderConnector：Provider 形态源经桥接泵出契约事件（白名单二次过滤 +
// 未知操作终止流）。
func TestProviderConnectorPoll(t *testing.T) {
	fp := &fakePumpSource{events: []ChangeEvent{
		{Database: "game", Table: "items", Op: OpInsert, AfterJSON: json.RawMessage(`[1,"a"]`), GTID: "g1", TsUnixMs: 1748736000000},
		{Database: "game", Table: "other", Op: OpInsert, AfterJSON: json.RawMessage(`[2]`), GTID: "g2", TsUnixMs: 1748736000001},
		{Database: "game", Table: "items", Op: Op("truncate"), GTID: "g3", TsUnixMs: 1748736000002},
		{Database: "game", Table: "items", Op: OpDelete, BeforeJSON: json.RawMessage(`[3]`), GTID: "g4", TsUnixMs: 1748736000003},
	}}
	p := NewProviderConnector(fp, "mysql", nil)
	ctx := t.Context()
	events, err := p.Poll(ctx, connector.PollRequest{Tables: []string{"game.items"}})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	var got []connector.ChangeEvent
	for ev := range events {
		got = append(got, ev)
	}
	if len(got) != 1 || got[0].Position.Value != "g1" {
		t.Fatalf("events = %+v, want only g1 (whitelist drops g2, unknown op aborts before g4)", got)
	}
}

// fakePumpSource 最小 Provider（推完即关流）。
type fakePumpSource struct {
	events []ChangeEvent
}

func (f *fakePumpSource) Open(_ context.Context, _ Bookmark) (<-chan ChangeEvent, error) {
	ch := make(chan ChangeEvent, len(f.events))
	for _, ev := range f.events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func (f *fakePumpSource) Bookmark() Bookmark { return Bookmark{} }
func (f *fakePumpSource) Close() error       { return nil }

// ProviderConnector 守卫：Open 留存配置（SourceID 透传）、无 Provider 拒绝。
func TestProviderConnectorGuards(t *testing.T) {
	p := NewProviderConnector(nil, "mysql", nil)
	if _, err := p.Poll(t.Context(), connector.PollRequest{}); err == nil {
		t.Fatal("poll without provider accepted")
	}
	if _, err := p.Checkpoint(t.Context()); err == nil {
		t.Fatal("checkpoint without provider accepted")
	}
	if err := p.Open(t.Context(), connector.SourceConfig{SourceID: "leg"}); err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := p.Close(t.Context()); err != nil {
		t.Fatalf("close: %v", err)
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
