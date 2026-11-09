package pg

// connector.go 补测：Open 的关闭态/已打开/坏 DSN/不可达收口、resolveStart
// 位点直通与 confirmed 坏值、Snapshot 的校验守卫链（poll 活跃/空与重复表/
// 形态与标识符闭集/空清单）、ensureReplConn 的缓存复用与连接失败弃连、
// snapshot 元数据纯函数面（splitQualifiedTable/quoteIdent/selectNames/
// rowImage/snapshotDatumJSON 全臂）、SchemaHistorySource 三面（无会话
// fail-closed / 位点形态 / 未记录与命中）、AckCheckpoint 守卫、Close 的
// 关闭守卫与 pump join 收尾。
//
// 网络路径（Discover 行集解析、临时 slot 快照事务、pump 接收循环、
// slotConfirmedLSN 的槽查询与创建、Close 的 repl/conn 直关）由
// fakepg_test.go 的进程内假 PG wire 服务器覆盖。
//
// 诚实跳过（结构性不可达 / 需流内故障注入，见 fakepg_test.go 文件头）：
// Snapshot 的 rand.Read 失败与生成 slot 名超长校验分支（时间戳+随机后缀
// 恒在 63 字符内）；rowImage/snapshotDatumJSON 的 json.Marshal 失败分支
// （string 序列化恒成功）；快照读连接拨号失败（与 repl 同 DSN，repl 先
// 失败）。

import (
	"context"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/jackc/pgx/v5/pgconn"
)

const pgExtraDSNEnv = "CROUPIER_CAPTURE_PG_DSN_EXTRA_TEST"

func TestOpenClosedAndAlreadyOpenGuards(t *testing.T) {
	const dsn = "postgres://u:p@127.0.0.1:1/db?connect_timeout=1"

	// 关闭态拒绝
	c := NewPostgresConnector(connector.SourceConfig{DSNEnv: pgExtraDSNEnv}, nil)
	c.closed.Store(true)
	if err := c.Open(context.Background(), connector.SourceConfig{DSNEnv: pgExtraDSNEnv}); err == nil ||
		!strings.Contains(err.Error(), "closed") {
		t.Fatalf("open after close = %v", err)
	}

	// 已打开拒绝（连接体占位即可，Open 不触碰连接方法）
	c2 := NewPostgresConnector(connector.SourceConfig{}, nil)
	c2.conn = &pgconn.PgConn{}
	if err := c2.Open(context.Background(), connector.SourceConfig{DSNEnv: pgExtraDSNEnv}); err == nil ||
		!strings.Contains(err.Error(), "already open") {
		t.Fatalf("double open = %v", err)
	}

	// 坏 DSN（解析失败，只提环境变量名）
	t.Setenv(pgExtraDSNEnv, "postgres://%zz@host/db")
	c3 := NewPostgresConnector(connector.SourceConfig{}, nil)
	if err := c3.Open(context.Background(), connector.SourceConfig{DSNEnv: pgExtraDSNEnv}); err == nil ||
		!strings.Contains(err.Error(), "parse dsn") {
		t.Fatalf("bad dsn = %v", err)
	}

	// 不可达：连接失败收口（127.0.0.1:1 即刻拒绝）
	t.Setenv(pgExtraDSNEnv, dsn)
	c4 := NewPostgresConnector(connector.SourceConfig{}, nil)
	if err := c4.Open(context.Background(), connector.SourceConfig{DSNEnv: pgExtraDSNEnv}); err == nil ||
		!strings.Contains(err.Error(), "connect via") {
		t.Fatalf("unreachable = %v", err)
	}
}

func TestResolveStartConfirmedGarbageAndFromPosition(t *testing.T) {
	// confirmed 位点坏值（:104）
	if _, err := resolveStartLSN(connector.Position{}, "garbage-lsn"); err == nil ||
		!strings.Contains(err.Error(), "parse slot confirmed position") {
		t.Fatalf("bad confirmed = %v", err)
	}

	// resolveStart 的位点直通臂（不触碰连接）
	c := NewPostgresConnector(connector.SourceConfig{}, nil)
	if _, err := c.resolveStart(context.Background(), "slot",
		connector.Position{Kind: "lsn", Value: "not-an-lsn"}); err == nil {
		t.Fatal("bad from position accepted")
	}
	lsn, err := c.resolveStart(context.Background(), "slot",
		connector.Position{Kind: "lsn", Value: "12345678901"})
	if err != nil || lsn != 12345678901 {
		t.Fatalf("from position = %d, %v", lsn, err)
	}
}

func TestSnapshotValidationGuards(t *testing.T) {
	c := NewPostgresConnector(connector.SourceConfig{SourceID: "pg-x"}, nil)
	c.conn = &pgconn.PgConn{} // 只过「已打开」守卫，校验臂在触碰连接前返回

	// poll 活跃时拒绝快照
	c.pumpCancel = func() {}
	_, err := c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items"}})
	if err == nil || !strings.Contains(err.Error(), "not allowed while poll is active") {
		t.Fatalf("snapshot during poll = %v", err)
	}
	c.pumpCancel = nil

	// 空表名 / 重复表名
	_, err = c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{""}})
	if err == nil || !strings.Contains(err.Error(), `table "" invalid`) {
		t.Fatalf("empty table = %v", err)
	}
	_, err = c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items", "game.items"}})
	if err == nil || !strings.Contains(err.Error(), "game.items") {
		t.Fatalf("duplicate table = %v", err)
	}

	// 形态与标识符闭集
	for _, table := range []string{"nodot", ".tbl", "db.", "Game.items"} {
		_, err = c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{table}})
		if err == nil {
			t.Fatalf("table %q accepted", table)
		}
	}

	// 空清单
	_, err = c.Snapshot(context.Background(), connector.SnapshotRequest{})
	if err == nil || !strings.Contains(err.Error(), "explicit table list") {
		t.Fatalf("empty list = %v", err)
	}
}

func TestEnsureReplConnCacheAndFailure(t *testing.T) {
	c := NewPostgresConnector(connector.SourceConfig{}, nil)

	// 缓存复用：已有复制连接直接返回（不新建、不关闭）
	fake := &pgconn.PgConn{}
	c.repl = fake
	got, err := c.ensureReplConn()
	if err != nil || got != fake {
		t.Fatalf("cached repl = %v, %v (want same handle)", got, err)
	}
	c.repl = nil

	// 不可达：克隆配置加 replication 参数后连接失败收口
	pcc, err := pgconn.ParseConfig("postgres://u:p@127.0.0.1:1/db?connect_timeout=1")
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	c.pcc = pcc
	if _, err = c.ensureReplConn(); err == nil ||
		!strings.Contains(err.Error(), "connect replication connection") {
		t.Fatalf("repl connect = %v", err)
	}
	if c.repl != nil {
		t.Fatal("failed repl connect must not cache a handle")
	}
}

func TestSplitQualifiedTableShapes(t *testing.T) {
	schema, name, err := splitQualifiedTable("game.items")
	if err != nil || schema != "game" || name != "items" {
		t.Fatalf("valid = %q %q %v", schema, name, err)
	}
	for _, table := range []string{"nodot", ".tbl", "db.", "a.b.c"} {
		if _, _, err = splitQualifiedTable(table); err == nil {
			t.Fatalf("table %q accepted", table)
		}
	}
}

func TestQuoteIdentAndSelectNames(t *testing.T) {
	if got := quoteIdent(`we"ird`); got != `"we""ird"` {
		t.Fatalf("quoteIdent = %q", got)
	}
	m := snapshotMetadata{columns: []connector.ColumnMeta{
		{Name: "id"}, {Name: `na"me`},
	}}
	got := m.selectNames()
	if len(got) != 2 || got[0] != `"id"` || got[1] != `"na""me"` {
		t.Fatalf("selectNames = %v", got)
	}
}

func TestRowImageArms(t *testing.T) {
	m := snapshotMetadata{columns: []connector.ColumnMeta{
		{Name: "flag", Type: "boolean"},
		{Name: "gold", Type: "numeric"},
		{Name: "note", Type: "text"},
	}}

	// 长度不符拒绝
	if _, err := m.rowImage([][]byte{{'t'}}); err == nil {
		t.Fatal("row length mismatch accepted")
	}

	image, err := m.rowImage([][]byte{[]byte("t"), []byte("123.45"), []byte("hello")})
	if err != nil {
		t.Fatalf("rowImage: %v", err)
	}
	if string(image["flag"]) != "true" || string(image["gold"]) != "123.45" || string(image["note"]) != `"hello"` {
		t.Fatalf("image = %v", image)
	}

	// NULL 列像 → null
	image, err = m.rowImage([][]byte{nil, []byte("1"), []byte("x")})
	if err != nil || string(image["flag"]) != "null" {
		t.Fatalf("null column = %v, %v", image["flag"], err)
	}

	// 数值域非 JSON 字面（NaN/Infinity）与坏数值 → 字符串兜底
	image, err = m.rowImage([][]byte{[]byte("f"), []byte("NaN"), []byte("12x")})
	if err != nil || string(image["flag"]) != "false" || string(image["gold"]) != `"NaN"` || string(image["note"]) != `"12x"` {
		t.Fatalf("non-json arms = %v, %v", image, err)
	}
}

func TestSnapshotDatumJSONArms(t *testing.T) {
	for _, pair := range []struct {
		typ, in, want string
	}{
		{"boolean", "t", "true"},
		{"boolean", "true", "true"},
		{"boolean", "f", "false"},
		{"boolean", "false", "false"},
		{"boolean", "yes", `"yes"`},
		{"integer", "42", "42"},
		{"numeric", "1.5", "1.5"},
		{"numeric", "NaN", `"NaN"`},
		{"double precision", "Infinity", `"Infinity"`},
		{"bigint", "-Infinity", `"-Infinity"`},
		{"integer", "12x", `"12x"`},
		{"text", "hi", `"hi"`},
	} {
		got, err := snapshotDatumJSON(pair.typ, []byte(pair.in))
		if err != nil || string(got) != pair.want {
			t.Fatalf("snapshotDatumJSON(%s, %s) = %s, %v (want %s)", pair.typ, pair.in, got, err, pair.want)
		}
	}
	// 结果是独立副本（不改写传入切片底层数组）
	v := []byte("42")
	out, _ := snapshotDatumJSON("integer", v)
	v[0] = '9'
	if string(out) != "42" {
		t.Fatal("datum json must copy the input")
	}
}

func TestSchemaHistorySourceFailsClosed(t *testing.T) {
	c := NewPostgresConnector(connector.SourceConfig{SourceID: "pg-h"}, nil)

	// 无活动会话 → fail-closed
	if _, err := c.SchemaAt(context.Background(), "game.items",
		connector.Position{Kind: "lsn", Value: "200"}); err == nil ||
		!strings.Contains(err.Error(), "no active poll session") {
		t.Fatalf("schema at without session = %v", err)
	}
	if _, err := c.CurrentSchemaVersions(context.Background()); err == nil {
		t.Fatal("versions without session accepted")
	}

	// 经解码器建立两版历史（v1@100 三列 → v2@500 加列）
	d := NewDecoder("pg-h")
	mustFeedAt(t, d, "100", wireMessage('R', relPayload(itemsOID, "game", "items", 'd',
		relCol(FlagReplicaIdentity, "id", 23), relCol(0, "name", 25), relCol(0, "gold", 1700))))
	mustFeedAt(t, d, "500", wireMessage('R', relPayload(itemsOID, "game", "items", 'd',
		relCol(FlagReplicaIdentity, "id", 23), relCol(0, "name", 25), relCol(0, "gold", 1700),
		relCol(0, "extra", 25))))
	c.schemaMu.Lock()
	c.schemaHist = d.SchemaHistory()
	c.schemaMu.Unlock()

	// 位点形态守卫：非 lsn kind 拒绝
	_, err := c.SchemaAt(context.Background(), "game.items",
		connector.Position{Kind: "gtid", Value: "1"})
	if err == nil || !strings.Contains(err.Error(), "needs an lsn position") {
		t.Fatalf("gtid lookup = %v", err)
	}

	// 未记录：位点早于首版
	if _, err = c.SchemaAt(context.Background(), "game.items",
		connector.Position{Kind: "lsn", Value: "50"}); err == nil ||
		!strings.Contains(err.Error(), "no schema version recorded") {
		t.Fatalf("pre-first-version lookup = %v", err)
	}

	// 命中 v1（无 extra）与当前版本号 2
	v1, err := c.SchemaAt(context.Background(), "game.items",
		connector.Position{Kind: "lsn", Value: "100"})
	if err != nil || v1.HasColumn("extra") {
		t.Fatalf("v1 lookup = %+v, %v", v1, err)
	}
	versions, err := c.CurrentSchemaVersions(context.Background())
	if err != nil || versions["game.items"] != 2 {
		t.Fatalf("versions = %v, %v", versions, err)
	}

	// 快照非 nil 且内容一致（Clone 面）
	snap := c.schemaHistorySnapshot()
	if snap == nil {
		t.Fatal("snapshot nil with wired history")
	}
	if _, rev, ok := snap.At("game.items", "100"); !ok || rev != 1 {
		t.Fatalf("snapshot At = rev %d found %v", rev, ok)
	}
}

func TestAckCheckpointGuards(t *testing.T) {
	c := NewPostgresConnector(connector.SourceConfig{}, nil)

	// 关闭态拒绝
	c.closed.Store(true)
	if err := c.AckCheckpoint(context.Background(),
		connector.Position{Kind: "lsn", Value: "1"}); err == nil ||
		!strings.Contains(err.Error(), "closed") {
		t.Fatalf("ack after close = %v", err)
	}
	c.closed.Store(false)

	// 位点 kind 闭集
	if err := c.AckCheckpoint(context.Background(),
		connector.Position{Kind: "gtid", Value: "1"}); err == nil ||
		!strings.Contains(err.Error(), "invalid lsn checkpoint") {
		t.Fatalf("gtid ack = %v", err)
	}

	// 合法位点落 checkpoint
	if err := c.AckCheckpoint(context.Background(),
		connector.Position{Kind: "lsn", Value: "12345"}); err != nil {
		t.Fatalf("valid ack = %v", err)
	}
	pos, err := c.Checkpoint(context.Background())
	if err != nil || pos.Value != "12345" {
		t.Fatalf("checkpoint = %+v, %v", pos, err)
	}
}

func TestPollReplicationOptionsRequired(t *testing.T) {
	// 已打开但缺槽/发布名：Poll 在触碰复制连接前拒绝（ensureReplConn
	// 之前收口，不建连接）
	c := NewPostgresConnector(connector.SourceConfig{}, nil)
	c.conn = &pgconn.PgConn{}
	if _, err := c.Poll(context.Background(), connector.PollRequest{}); err == nil ||
		!strings.Contains(err.Error(), "options.slotName required") {
		t.Fatalf("poll without slot = %v", err)
	}

	// 发布名标识符闭集（replicationOptions 的 pub 校验臂）
	c.cfg.Options = map[string]string{"slotName": "slot_ok", "publication": "BAD PUB"}
	if _, _, err := c.replicationOptions(); err == nil ||
		!strings.Contains(err.Error(), "publication name") {
		t.Fatalf("bad publication = %v", err)
	}
}

func TestCloseJoinsStuckPumpWithTimeout(t *testing.T) {
	// pump 滞留：Close 防御超时（5s）后收尾，不无限挂死
	c := NewPostgresConnector(connector.SourceConfig{}, nil)
	c.pumpCancel = func() {} // 永不结束的 pump
	c.pumpDone = make(chan struct{})
	c.repl = &pgconn.PgConn{}
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("close stuck pump = %v", err)
	}
	if c.pumpCancel != nil || c.pumpDone != nil || c.repl != nil {
		t.Fatalf("stuck pump close handles = %v %v %v", c.pumpCancel, c.pumpDone, c.repl)
	}
}

func TestCloseJoinPumpAndGuards(t *testing.T) {
	// 未开连接的 Close 是 no-op，幂等
	c := NewPostgresConnector(connector.SourceConfig{}, nil)
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("close fresh = %v", err)
	}
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("close idempotent = %v", err)
	}

	// pump join 收尾：取消函数被调用、句柄复位、repl 缓存清空
	cancelled := false
	done := make(chan struct{})
	repl := &pgconn.PgConn{}
	c2 := NewPostgresConnector(connector.SourceConfig{}, nil)
	c2.repl = repl
	c2.pumpCancel = func() { cancelled = true }
	c2.pumpDone = done
	close(done)
	if err := c2.Close(context.Background()); err != nil {
		t.Fatalf("close with pump = %v", err)
	}
	if !cancelled || c2.pumpCancel != nil || c2.pumpDone != nil || c2.repl != nil {
		t.Fatalf("close join = cancelled %v cancel %v done %v repl %v",
			cancelled, c2.pumpCancel, c2.pumpDone, c2.repl)
	}
	if _, err := c2.Poll(context.Background(), connector.PollRequest{}); err == nil {
		t.Fatal("poll after close accepted")
	}
}
