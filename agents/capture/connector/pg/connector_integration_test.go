//go:build integration

package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"

	// pgx stdlib：database/sql 驱动注册（测试建表/发布用普通连接）。
	"github.com/cuihairu/croupier/pkg/cdc/schema"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// 活栈冒烟（todo 4306 增量二，CI capture-matrix 与本地 docker 同跑法）：
// wal_level=logical 的 postgres:16 走完整链——建表建发布 → 空位点 Poll
// （自动建槽）→ INSERT/UPDATE/DELETE 事件全收（列名映射/主键/前像能力）→
// Checkpoint=lsn 可续读 → 断开后从检查点位点重连收到后续事件。
//
// 环境变量：TEST_POSTGRES_CAPTURE_DSN（缺席跳过；与迁移矩阵的
// TEST_POSTGRES_DSN 区分——本测试要求源端 wal_level=logical，普通实例
// 会被 START_REPLICATION 拒绝，不能混用）。

func mustExecPG(t *testing.T, dsn string, stmts ...string) {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("admin open: %v", err)
	}
	defer func() { _ = db.Close() }()
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("admin exec %q: %v", s, err)
		}
	}
}

func waitPGEvent(t *testing.T, events <-chan connector.ChangeEvent, match func(connector.ChangeEvent) bool) connector.ChangeEvent {
	t.Helper()
	deadline := time.After(20 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatalf("event stream closed before match")
			}
			if match(ev) {
				return ev
			}
			if b, err := json.Marshal(ev); err == nil {
				t.Logf("non-matching event: %s", b)
			}
		case <-deadline:
			t.Fatalf("timed out waiting for matching event")
		}
	}
}

// imageHasMarker 行像任一键值含 marker 串（JSON 文本层匹配，测试够用）。
func imageHasMarker(ev connector.ChangeEvent, marker string) bool {
	for _, img := range []map[string]json.RawMessage{ev.Before, ev.After} {
		for _, raw := range img {
			if strings.Contains(string(raw), marker) {
				return true
			}
		}
	}
	return false
}

func TestPostgresConnectorLogicalReplication(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_CAPTURE_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_CAPTURE_DSN not configured")
	}
	const slot = "capture_pg_test_slot"
	const pub = "capture_pg_test_pub"

	// 复位：旧槽/发布/表清掉（可重复跑；槽随库存在，先删槽再删表）。
	mustExecPG(t, dsn,
		`SELECT pg_drop_replication_slot('`+slot+`') WHERE EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name='`+slot+`')`,
		`DROP PUBLICATION IF EXISTS `+pub,
		`DROP TABLE IF EXISTS public.t1`,
		`CREATE TABLE public.t1 (id INT PRIMARY KEY, v TEXT NOT NULL)`,
		`CREATE PUBLICATION `+pub+` FOR TABLE public.t1`,
	)
	t.Cleanup(func() {
		// 连接已由 defer 关闭，但 pump 收敛可能有半拍延迟——删槽带重试。
		for i := 0; i < 10; i++ {
			db, err := sql.Open("pgx", dsn)
			if err != nil {
				break
			}
			_, err = db.Exec(`SELECT pg_drop_replication_slot('` + slot + `') WHERE EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name='` + slot + `')`)
			_ = db.Close()
			if err == nil {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		mustExecPG(t, dsn,
			`DROP PUBLICATION IF EXISTS `+pub,
			`DROP TABLE IF EXISTS public.t1`,
		)
	})

	cfg := connector.SourceConfig{
		SourceID:   "pg-itest",
		SourceType: "postgres",
		DSNEnv:     "TEST_POSTGRES_CAPTURE_DSN",
		Options:    map[string]string{"slotName": slot, "publication": pub},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c := NewPostgresConnector(cfg, nil)
	if err := c.Open(ctx, cfg); err != nil {
		t.Fatalf("open: %v", err)
	}
	// Fatalf（Goexit）也保证连接关闭，槽不被复制流占住（cleanup 删槽依赖）。
	defer func() { _ = c.Close(ctx) }()

	// Discover：元数据与能力同源。
	disc, err := c.Discover(ctx, connector.DiscoveryRequest{Tables: []string{"public.t1"}})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(disc.Tables) != 1 || disc.Tables[0].Name != "public.t1" {
		t.Fatalf("discover tables = %+v", disc.Tables)
	}
	if len(disc.Tables[0].PrimaryKey) != 1 || disc.Tables[0].PrimaryKey[0] != "id" {
		t.Fatalf("discover pk = %+v", disc.Tables[0].PrimaryKey)
	}
	if !disc.Capabilities.AfterImage || !disc.Capabilities.ResumableCheckpoint {
		t.Fatalf("capabilities = %+v", disc.Capabilities)
	}

	// 空位点 Poll：自动建槽，从 consistent point 起订。
	events, err := c.Poll(ctx, connector.PollRequest{Tables: []string{"public.t1"}})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("dml open: %v", err)
	}
	defer func() { _ = db.Close() }()

	// INSERT → 事件（列名映射/主键/后像）。
	if _, err := db.Exec(`INSERT INTO public.t1 (id, v) VALUES (1, 'marker-ins')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	ev := waitPGEvent(t, events, func(e connector.ChangeEvent) bool {
		return e.Operation == connector.OpInsert && imageHasMarker(e, "marker-ins")
	})
	if ev.Database != "public" || ev.Table != "t1" {
		t.Fatalf("insert identity: %+v", ev)
	}
	if !ev.ColumnsMapped || string(ev.After["v"]) != `"marker-ins"` {
		t.Fatalf("insert image: columnsMapped=%v after=%v", ev.ColumnsMapped, ev.After)
	}
	if len(ev.Key) != 1 || ev.Key[0] != "id" {
		t.Fatalf("insert key = %v, want [id]", ev.Key)
	}
	if ev.Position.Kind != "lsn" || !ev.Position.Valid() || ev.TsUnixMs <= 0 {
		t.Fatalf("insert position/ts: %+v", ev)
	}

	// UPDATE 非键列 → 前像真没有：REPLICA IDENTITY DEFAULT 只在键列变更时
	// 携带 'K' 旧元组——与能力声明 beforeImage=false 的诚实口径一致。
	if _, err := db.Exec(`UPDATE public.t1 SET v = 'marker-upd' WHERE id = 1`); err != nil {
		t.Fatalf("update: %v", err)
	}
	ev = waitPGEvent(t, events, func(e connector.ChangeEvent) bool {
		return e.Operation == connector.OpUpdate && imageHasMarker(e, "marker-upd")
	})
	if ev.Before != nil {
		t.Fatalf("non-key update before image should be nil (REPLICA IDENTITY DEFAULT), got %v", ev.Before)
	}
	if string(ev.After["v"]) != `"marker-upd"` {
		t.Fatalf("update after image: %v", ev.After)
	}

	// UPDATE 键列（id 1→2）→ 'K' 身份键旧元组出现（活体实测：'K' 元组全宽，
	// 非键列为 null 占位，非「仅键列」——Before={"id":1,"v":null}）。
	if _, err := db.Exec(`UPDATE public.t1 SET id = 2 WHERE id = 1`); err != nil {
		t.Fatalf("key update: %v", err)
	}
	ev = waitPGEvent(t, events, func(e connector.ChangeEvent) bool {
		return e.Operation == connector.OpUpdate &&
			string(e.Before["id"]) == "1" && string(e.After["id"]) == "2"
	})
	if string(ev.Before["id"]) != "1" {
		t.Fatalf("key update before = %v, want id=1", ev.Before)
	}
	if string(ev.After["id"]) != "2" {
		t.Fatalf("key update after = %v, want id=2", ev.After)
	}

	// Checkpoint：先确认位点，再读快照（b0e035d 起 Checkpoint 只回已确认位点），
	// lsn 位点可解析回读。
	if err := c.AckCheckpoint(ctx, ev.Position); err != nil {
		t.Fatalf("ack checkpoint: %v", err)
	}
	cp, err := c.Checkpoint(ctx)
	if err != nil || cp.Kind != "lsn" || cp.Value == "" {
		t.Fatalf("checkpoint = %+v err=%v", cp, err)
	}
	if _, err := ParseLSN(cp.Value); err != nil {
		t.Fatalf("checkpoint lsn %q unparseable: %v", cp.Value, err)
	}

	// 断开 → 从检查点续读 → DELETE 事件收到（槽保留位点语义）。
	if err := c.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := c.Poll(ctx, connector.PollRequest{}); err == nil {
		t.Fatal("poll after close accepted")
	}

	c2 := NewPostgresConnector(cfg, nil)
	if err := c2.Open(ctx, cfg); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = c2.Close(ctx) }()
	events2, err := c2.Poll(ctx, connector.PollRequest{From: cp, Tables: []string{"public.t1"}})
	if err != nil {
		t.Fatalf("repoll: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM public.t1 WHERE id = 2`); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// 从检查点续读可能重放边界事务（at-least-once），按操作闭集匹配 DELETE。
	// DELETE 在 REPLICA IDENTITY DEFAULT 下恒带 'K' 身份键旧元组。
	ev = waitPGEvent(t, events2, func(e connector.ChangeEvent) bool {
		return e.Operation == connector.OpDelete && len(e.After) == 0 && string(e.Before["id"]) == "2"
	})
	if ev.Table != "t1" {
		t.Fatalf("delete identity: %+v", ev)
	}
	if err := c2.Close(ctx); err != nil {
		t.Fatalf("close2: %v", err)
	}
}

func TestPostgresConnectorSnapshot(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_CAPTURE_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_CAPTURE_DSN not configured")
	}
	const pub = "snapshot_test_pub"
	mustExecPG(t, dsn,
		`DROP PUBLICATION IF EXISTS `+pub,
		`DROP TABLE IF EXISTS public.snapshot_rows`,
		`CREATE TABLE public.snapshot_rows (
			id BIGINT PRIMARY KEY,
			label TEXT,
			amount NUMERIC,
			enabled BOOLEAN,
			created_at TIMESTAMPTZ
		)`,
		`CREATE PUBLICATION `+pub+` FOR TABLE public.snapshot_rows`,
		`INSERT INTO public.snapshot_rows VALUES
			(1, 'one', 1.25, true, '2026-10-19T00:00:00Z'),
			(2, NULL, 2.50, false, '2026-10-19T01:00:00Z')`,
	)
	t.Cleanup(func() {
		mustExecPG(t, dsn,
			`DROP PUBLICATION IF EXISTS `+pub,
			`DROP TABLE IF EXISTS public.snapshot_rows`,
		)
	})

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("dml open: %v", err)
	}
	defer func() { _ = db.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg := connector.SourceConfig{
		SourceID:   "pg-snapshot-itest",
		SourceType: "postgres",
		DSNEnv:     "TEST_POSTGRES_CAPTURE_DSN",
		Options:    map[string]string{"slotName": "snapshot_poll_slot", "publication": pub},
	}
	c := NewPostgresConnector(cfg, nil)
	if err := c.Open(ctx, cfg); err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = c.Close(ctx) }()

	events, err := c.Snapshot(ctx, connector.SnapshotRequest{Tables: []string{"public.snapshot_rows"}})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	rows := make([]connector.ChangeEvent, 0, 2)
	for event := range events {
		rows = append(rows, event)
	}
	if len(rows) != 2 {
		t.Fatalf("snapshot rows = %d, want 2: %+v", len(rows), rows)
	}
	if rows[0].Database != "public" || rows[0].Table != "snapshot_rows" {
		t.Fatalf("snapshot identity = %+v", rows[0])
	}
	if rows[0].Operation != connector.OpInsert || !rows[0].ColumnsMapped {
		t.Fatalf("snapshot operation = %+v", rows[0])
	}
	if len(rows[0].Key) != 1 || rows[0].Key[0] != "id" {
		t.Fatalf("snapshot key = %v", rows[0].Key)
	}
	if got := string(rows[0].After["id"]); got != "1" {
		t.Fatalf("snapshot id = %s, want 1", got)
	}
	if got := string(rows[0].After["amount"]); got != "1.25" {
		t.Fatalf("snapshot amount = %s, want 1.25", got)
	}
	if got := string(rows[0].After["enabled"]); got != "true" {
		t.Fatalf("snapshot enabled = %s, want true", got)
	}
	if got := string(rows[1].After["label"]); got != "null" {
		t.Fatalf("snapshot label = %s, want null", got)
	}
	if rows[0].Position.Kind != "lsn" || rows[0].Position.Value == "" || rows[0].Position != rows[1].Position {
		t.Fatalf("snapshot positions = %+v and %+v", rows[0].Position, rows[1].Position)
	}
	if rows[0].Tx.ID != "" || rows[0].Tx.Seq != 1 || rows[0].Tx.End {
		t.Fatalf("first transaction = %+v", rows[0].Tx)
	}
	if rows[1].Tx.Seq != 2 || !rows[1].Tx.End {
		t.Fatalf("last transaction = %+v", rows[1].Tx)
	}
	for _, row := range rows {
		if err := row.Validate(); err != nil {
			t.Fatalf("row validation: %v", err)
		}
	}

	var slots int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM pg_replication_slots WHERE slot_name LIKE 'croupier_snapshot_%'`).Scan(&slots); err != nil {
		t.Fatalf("count snapshot slots: %v", err)
	}
	if slots != 0 {
		t.Fatalf("temporary snapshot slots = %d, want 0", slots)
	}
}

// schema 演进历史真实容器验收（todo 69）：DDL 变更（ALTER TABLE ADD COLUMN）
// 注入真实 postgres 复制流 → 连接器按消息位点记录列型映射版本化；恢复按
// 事务时点查询 schema（SchemaAt）——旧事务位点得到旧列集、DDL 后得到新
// 列集；引用时点外列的事件按历史 schema 拒绝（旧事件恢复拒绝路径）。
func TestPostgresConnectorSchemaEvolutionHistory(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_CAPTURE_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_CAPTURE_DSN not configured")
	}
	const slot = "capture_pg_schema_test_slot"
	const pub = "capture_pg_schema_test_pub"
	const table = "public.schema_evo"

	mustExecPG(t, dsn,
		`SELECT pg_drop_replication_slot('`+slot+`') WHERE EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name='`+slot+`')`,
		`DROP PUBLICATION IF EXISTS `+pub,
		`DROP TABLE IF EXISTS `+table,
		`CREATE TABLE `+table+` (id INT PRIMARY KEY, v TEXT NOT NULL)`,
		`CREATE PUBLICATION `+pub+` FOR TABLE `+table,
	)
	t.Cleanup(func() {
		for i := 0; i < 10; i++ {
			db, err := sql.Open("pgx", dsn)
			if err != nil {
				break
			}
			_, err = db.Exec(`SELECT pg_drop_replication_slot('` + slot + `') WHERE EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name='` + slot + `')`)
			_ = db.Close()
			if err == nil {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		mustExecPG(t, dsn, `DROP PUBLICATION IF EXISTS `+pub, `DROP TABLE IF EXISTS `+table)
	})

	cfg := connector.SourceConfig{
		SourceID:   "pg-schema-itest",
		SourceType: "postgres",
		DSNEnv:     "TEST_POSTGRES_CAPTURE_DSN",
		Options:    map[string]string{"slotName": slot, "publication": pub},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c := NewPostgresConnector(cfg, nil)
	if err := c.Open(ctx, cfg); err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = c.Close(ctx) }()

	events, err := c.Poll(ctx, connector.PollRequest{Tables: []string{table}})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("dml open: %v", err)
	}
	defer func() { _ = db.Close() }()

	// v1 事件（DDL 前：id, v 两列）。
	if _, err := db.Exec(`INSERT INTO ` + table + ` (id, v) VALUES (1, 'v1')`); err != nil {
		t.Fatalf("insert v1: %v", err)
	}
	ev1 := waitPGEvent(t, events, func(e connector.ChangeEvent) bool {
		return e.Operation == connector.OpInsert && imageHasMarker(e, "v1")
	})
	if ev1.SchemaVersion != 1 {
		t.Fatalf("v1 event schema version = %d, want 1", ev1.SchemaVersion)
	}
	if ev1.Position.Kind != "lsn" || !ev1.Position.Valid() {
		t.Fatalf("v1 event position = %+v", ev1.Position)
	}

	// DDL 注入：加列 w。
	if _, err := db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN w INT`); err != nil {
		t.Fatalf("alter: %v", err)
	}

	// v2 事件（DDL 后：id, v, w 三列）。
	if _, err := db.Exec(`INSERT INTO ` + table + ` (id, v, w) VALUES (2, 'v2', 7)`); err != nil {
		t.Fatalf("insert v2: %v", err)
	}
	ev2 := waitPGEvent(t, events, func(e connector.ChangeEvent) bool {
		return e.Operation == connector.OpInsert && imageHasMarker(e, "v2")
	})
	if ev2.SchemaVersion != 2 {
		t.Fatalf("v2 event schema version = %d, want 2", ev2.SchemaVersion)
	}

	// 连接器暴露的 schema 历史：两版本，列集按 DDL 分割。
	hist := c.schemaHistorySnapshot()
	if err := hist.Validate(); err != nil {
		t.Fatalf("history validate: %v", err)
	}
	if got := len(hist.Versions(table)); got != 2 {
		t.Fatalf("history versions = %d, want 2", got)
	}
	s1, rev1, found1 := hist.At(table, ev1.Position.Value)
	if !found1 || rev1 != 1 {
		t.Fatalf("At(ev1 lsn) = rev %d found %v, want rev 1", rev1, found1)
	}
	if !s1.HasColumn("id") || !s1.HasColumn("v") || s1.HasColumn("w") {
		t.Fatalf("v1 schema columns = %v, want [id v]", s1.ColumnNames())
	}
	s2, rev2, found2 := hist.At(table, ev2.Position.Value)
	if !found2 || rev2 != 2 || !s2.HasColumn("w") {
		t.Fatalf("At(ev2 lsn) = rev %d found %v cols %v, want rev 2 with w", rev2, found2, s2.ColumnNames())
	}

	// 连接器 SchemaHistorySource 扩展面：SchemaAt 与 CurrentSchemaVersions。
	viaConn, err := c.SchemaAt(ctx, table, ev1.Position)
	if err != nil {
		t.Fatalf("SchemaAt(ev1): %v", err)
	}
	if viaConn.HasColumn("w") {
		t.Fatalf("SchemaAt(ev1) returned future column set: %v", viaConn.ColumnNames())
	}
	viaConn2, err := c.SchemaAt(ctx, table, ev2.Position)
	if err != nil {
		t.Fatalf("SchemaAt(ev2): %v", err)
	}
	if !viaConn2.HasColumn("w") {
		t.Fatalf("SchemaAt(ev2) missing w: %v", viaConn2.ColumnNames())
	}
	cur, err := c.CurrentSchemaVersions(ctx)
	if err != nil {
		t.Fatalf("CurrentSchemaVersions: %v", err)
	}
	if cur[table] != 2 {
		t.Fatalf("current schema versions = %v, want %s=2", cur, table)
	}

	// 旧事件恢复拒绝路径：v2 事件列集（含 w）在 v1 时点 schema 下被拒。
	if err := schema.ValidateColumns(s1, []string{"id", "v", "w"}); err == nil {
		t.Fatal("v2 event columns validated under v1 time-point schema must be rejected")
	}
	// v1 事件列集在 v1 时点 schema 下通过（不误伤）。
	if err := schema.ValidateColumns(s1, []string{"id", "v"}); err != nil {
		t.Fatalf("v1 event under v1 schema: %v", err)
	}
}
