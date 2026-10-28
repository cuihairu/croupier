package pg

import (
	"strings"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/connector"
)

// 复制标识白名单（slot/publication 名走 SQL 拼接，闭集校验杜绝注入面）。
func TestValidateReplicationIdent(t *testing.T) {
	for _, ok := range []string{"croupier_cap", "slot1", "a"} {
		if err := validateReplicationIdent("slot name", ok); err != nil {
			t.Fatalf("validate(%q): %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Upper", "with-dash", "with space", "quote'", "a;DROP TABLE x", strings.Repeat("a", 64)} {
		if err := validateReplicationIdent("slot name", bad); err == nil {
			t.Fatalf("validate(%q) accepted, want error", bad)
		}
	}
}

// 起点 LSN 决策：检查点位点续读 / 空 From 走槽 confirmed 位点 / 非法形态
// 早失败（发起复制前）。
func TestResolveStartLSN(t *testing.T) {
	// 检查点十进制串 → LSN。
	lsn, err := resolveStartLSN(connector.Position{Kind: "lsn", Value: "12345678901"}, "")
	if err != nil {
		t.Fatalf("lsn checkpoint: %v", err)
	}
	if lsn != 12345678901 {
		t.Fatalf("lsn checkpoint = %d, want 12345678901", lsn)
	}
	// "X/Y" hex 形态也收（服务端原生态位）。
	lsn, err = resolveStartLSN(connector.Position{Kind: "lsn", Value: "3/1"}, "")
	if err != nil {
		t.Fatalf("lsn hex: %v", err)
	}
	if lsn != 0x3_00000001 {
		t.Fatalf("lsn hex = %d, want %d", lsn, 0x3_00000001)
	}
	// 空 From → slot confirmed。
	lsn, err = resolveStartLSN(connector.Position{}, "0/16B37D0")
	if err != nil {
		t.Fatalf("confirmed: %v", err)
	}
	if lsn == 0 {
		t.Fatalf("confirmed lsn = 0")
	}
	// 错误路径。
	if _, err := resolveStartLSN(connector.Position{Kind: "lsn", Value: "not-an-lsn"}, ""); err == nil {
		t.Fatal("bad checkpoint accepted")
	}
	if _, err := resolveStartLSN(connector.Position{Kind: "gtid", Value: "0-1-5"}, ""); err == nil {
		t.Fatal("gtid position accepted by pg source")
	}
	if _, err := resolveStartLSN(connector.Position{}, ""); err == nil {
		t.Fatal("empty position without confirmed accepted")
	}
}

// Open 凭据红线：环境变量缺席报错且只提变量名，不回显取值。
func TestPostgresConnectorOpenMissingEnv(t *testing.T) {
	t.Setenv("CROUPIER_CAPTURE_PG_DSN_MISSING", "")
	c := NewPostgresConnector(connector.SourceConfig{
		SourceID:   "pg-test",
		SourceType: "postgres",
		DSNEnv:     "CROUPIER_CAPTURE_PG_DSN_MISSING",
	}, nil)
	if err := c.Open(t.Context(), c.cfg); err == nil {
		t.Fatal("open without env accepted")
	} else if !strings.Contains(err.Error(), "CROUPIER_CAPTURE_PG_DSN_MISSING") {
		t.Fatalf("error should name the env var, got: %v", err)
	}
	// dsnEnv 缺失。
	c2 := NewPostgresConnector(connector.SourceConfig{SourceID: "pg-test", SourceType: "postgres"}, nil)
	if err := c2.Open(t.Context(), c2.cfg); err == nil {
		t.Fatal("open without dsnEnv accepted")
	}
}

// Poll 契约守卫：未 Open 拒绝；Options 缺 slotName/publication 拒绝（发
// 起 I/O 前失败）；非 open 连接的 Discover/Snapshot/Checkpoint 同样拒绝。
func TestPostgresConnectorContractGuards(t *testing.T) {
	c := NewPostgresConnector(connector.SourceConfig{
		SourceID:   "pg-test",
		SourceType: "postgres",
		DSNEnv:     "CROUPIER_CAPTURE_PG_DSN_MISSING",
		Options:    map[string]string{"slotName": "cap_slot", "publication": "cap_pub"},
	}, nil)
	if _, err := c.Poll(t.Context(), connector.PollRequest{}); err == nil {
		t.Fatal("poll before open accepted")
	}
	if _, err := c.Discover(t.Context(), connector.DiscoveryRequest{}); err == nil {
		t.Fatal("discover before open accepted")
	}
	if _, err := c.Snapshot(t.Context(), connector.SnapshotRequest{}); err == nil {
		t.Fatal("snapshot before open accepted")
	}
}

// Options 契约：slotName/publication 必填且过白名单（Poll 发起 I/O 前的
// 校验，单测直测纯函数）。
func TestPostgresReplicationOptions(t *testing.T) {
	c := NewPostgresConnector(connector.SourceConfig{}, nil)
	if _, _, err := c.replicationOptions(); err == nil || !strings.Contains(err.Error(), "slotName") {
		t.Fatalf("missing slotName: %v", err)
	}
	c.cfg.Options = map[string]string{"slotName": "cap_slot"}
	if _, _, err := c.replicationOptions(); err == nil || !strings.Contains(err.Error(), "publication") {
		t.Fatalf("missing publication: %v", err)
	}
	c.cfg.Options["publication"] = "CAP-PUB"
	if _, _, err := c.replicationOptions(); err == nil {
		t.Fatal("uppercase publication accepted")
	}
	c.cfg.Options["publication"] = "cap_pub"
	if slot, pub, err := c.replicationOptions(); err != nil || slot != "cap_slot" || pub != "cap_pub" {
		t.Fatalf("valid options: slot=%q pub=%q err=%v", slot, pub, err)
	}
}

// Snapshot 契约守卫：未 Open 与空表清单均显式拒绝。
func TestPostgresSnapshotErrUnsupported(t *testing.T) {
	c := NewPostgresConnector(connector.SourceConfig{}, nil)
	for _, req := range []connector.SnapshotRequest{{}, {Tables: []string{}}} {
		if _, err := c.Snapshot(t.Context(), req); err == nil {
			t.Fatal("snapshot accepted, want error")
		}
	}
	if _, err := c.Snapshot(t.Context(), connector.SnapshotRequest{Tables: []string{"game.items"}}); err == nil {
		t.Fatal("snapshot before open accepted")
	}
}

func TestPostgresCapabilities(t *testing.T) {
	caps := PostgresCapabilities()
	if !caps.BeforeImage && !caps.AfterImage {
		t.Fatalf("capabilities = %+v", caps)
	}
	if !caps.TransactionMetadata || !caps.Snapshot || !caps.ResumableCheckpoint || !caps.SchemaHistory {
		t.Fatalf("capabilities = %+v", caps)
	}
	if caps.Pushdown {
		t.Fatalf("capabilities = %+v", caps)
	}
}

func TestPostgresAckCheckpoint(t *testing.T) {
	want := connector.Position{Kind: "lsn", Value: "12345678901"}
	c := NewPostgresConnector(connector.SourceConfig{SourceID: "pg-ack"}, nil)
	if err := c.AckCheckpoint(t.Context(), want); err != nil {
		t.Fatalf("ack checkpoint: %v", err)
	}
	got, err := c.Checkpoint(t.Context())
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if got != want {
		t.Fatalf("checkpoint = %+v, want %+v", got, want)
	}
}

// 「槽位早于发布」流中报错的识别与处置提示（E2E 实测踩中的死循环形态：
// START_REPLICATION 成功但 change 回调的历史快照看不到后建的发布，每次重连
// 都重读同一旧位点）。
func TestPublicationPrematureHint(t *testing.T) {
	hint, ok := publicationPrematureHint(`publication "cap_e2e_pub" does not exist`)
	if !ok {
		t.Fatal("premature publication error not recognized")
	}
	if !strings.Contains(hint, "cap_e2e_pub") || !strings.Contains(hint, "pg_replication_slot_advance") {
		t.Fatalf("hint = %q, want publication name and remediation", hint)
	}

	for _, bad := range []string{
		"",
		"publication cap_e2e_pub does not exist", // 无引号：不是本识别面
		`publication "" does not exist`,          // 空名
		`publication "a
b" does not exist`, // 换行注入面
		"replication slot \"cap_e2e_slot\" is active", // 其他错误不误报
		`publication "cap_e2e_pub" is already exists`,
		"unknown failure",
	} {
		if _, ok := publicationPrematureHint(bad); ok {
			t.Fatalf("hint accepted %q, want no match", bad)
		}
	}
}
