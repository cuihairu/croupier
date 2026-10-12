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

	// Checkpoint：lsn 位点可解析回读。
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
