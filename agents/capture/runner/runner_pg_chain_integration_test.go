//go:build integration

package runner

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/config"
	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/cuihairu/croupier/agents/capture/gate"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"

	// pgx stdlib：database/sql 驱动注册（建表/发布/槽位探测用普通连接）。
	_ "github.com/jackc/pgx/v5/stdlib"
)

// PG 活栈全链（todo 4306 P0 闭环「规则→面板」的 agent 侧证据）：真 PG
// （wal_level=logical）→ PostgresConnector → runSourceOnce 消费链（事件
// 粗过滤 → 三闸评估 → sweep 销账 → 告警上报）→ 位点落盘与重启续读。
//
// 关键口径：PG 行像按列名键化（ColumnsMapped=true，pgoutput Relation 携带
// 列名）——闸规则键 spec 用列名（如 "player_id"），与 MySQL 位置下标形态
// （"0","1"）对偶。本测试锁死「列名 spec 在 PG 上正确取键、闸命中、告警
// 携带正确键值」，证明三闸链非 MySQL 专属。
//
// 环境变量：TEST_POSTGRES_CAPTURE_DSN（缺席跳过；须 wal_level=logical，
// 与 CI capture-matrix 同跑法：docker run postgres:16 -c wal_level=logical）。

func mustChainExec(t *testing.T, dsn string, stmts ...string) {
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

// chainDiagnostics 失败现场：源库实际行 + runner 事件级日志（判定「哪条
// 事件被哪个角色消费」）。
func chainDiagnostics(logBuf *bytes.Buffer, dsn string) string {
	var out strings.Builder
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Sprintf("diagnostics: db open %v", err)
	}
	defer func() { _ = db.Close() }()
	rows, err := db.Query(`SELECT t, v FROM (
		SELECT 'items' t, player_id::text || '/' || item_id::text || '/' || amount::text v FROM game.items
		UNION ALL
		SELECT 'ledger', player_id::text || '/' || item_id::text || '/' || memo FROM game.ledger
	) x ORDER BY 1, 2`)
	if err != nil {
		out.WriteString(fmt.Sprintf("rows query: %v\n", err))
	} else {
		for rows.Next() {
			var t, v string
			if err := rows.Scan(&t, &v); err == nil {
				fmt.Fprintf(&out, "%s=%s\n", t, v)
			}
		}
		_ = rows.Close()
	}
	out.WriteString("--- runner log ---\n")
	out.WriteString(logBuf.String())
	return out.String()
}

func waitSlotActive(t *testing.T, dsn, slot string) {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("slot probe open: %v", err)
	}
	defer func() { _ = db.Close() }()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		var active bool
		if err := db.QueryRow(`SELECT active FROM pg_replication_slots WHERE slot_name = $1`, slot).Scan(&active); err == nil && active {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("replication slot %s never became active", slot)
}

func waitForStats(t *testing.T, r *Runner, match func(Stats) bool, diag ...func() string) Stats {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		st := r.Stats()
		if match(st) {
			return st
		}
		time.Sleep(50 * time.Millisecond)
	}
	msg := fmt.Sprintf("stats condition not met in time; last = %+v", r.Stats())
	for _, d := range diag {
		msg += "\n" + d()
	}
	t.Fatalf("%s", msg)
	return Stats{}
}

// captureSend 捕获告警上报（测试桩：同包替换 sendAlert 接缝）。
type captureSend struct {
	mu   sync.Mutex
	sent []*opsv1.CaptureRuleHitReport
}

func (s *captureSend) send(_ context.Context, rep *opsv1.CaptureRuleHitReport) (*opsv1.CaptureRuleHitAck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, rep)
	return &opsv1.CaptureRuleHitAck{}, nil
}

func (s *captureSend) snapshot() []*opsv1.CaptureRuleHitReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*opsv1.CaptureRuleHitReport(nil), s.sent...)
}

func waitReportCount(t *testing.T, s *captureSend, n int) []*opsv1.CaptureRuleHitReport {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got := s.snapshot()
		if len(got) >= n {
			return got
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("reports = %d, want >= %d", len(s.snapshot()), n)
	return nil
}

// TestRunnerPostgresFullChain 真源→过滤→三闸→告警→位点→续读。
func TestRunnerPostgresFullChain(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_CAPTURE_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_CAPTURE_DSN not set")
	}
	suffix := strconv.FormatInt(time.Now().UnixNano()%1_000_000, 10)
	slot := "cap_runner_" + suffix
	pub := "cap_pub_" + suffix

	mustChainExec(t, dsn,
		`DROP PUBLICATION IF EXISTS `+pub,
		`DROP SCHEMA IF EXISTS game CASCADE`,
		`CREATE SCHEMA game`,
		`CREATE TABLE game.items (player_id int PRIMARY KEY, item_id int, amount int)`,
		`CREATE TABLE game.ledger (player_id int PRIMARY KEY, item_id int, memo text)`,
		`CREATE PUBLICATION `+pub+` FOR TABLE game.items, game.ledger`,
	)
	t.Cleanup(func() {
		// 槽在连接释放后才能删（best effort，失败仅日志不阻塞用例）。
		db, err := sql.Open("pgx", dsn)
		if err == nil {
			_, _ = db.Exec(`SELECT pg_drop_replication_slot($1) WHERE EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name = $1)`, slot)
			_, _ = db.Exec(`DROP PUBLICATION IF EXISTS ` + pub)
			_ = db.Close()
		}
	})

	dir := t.TempDir()
	if _, err := os.Stat(filepath.Join(dir, "postgres.json")); err == nil {
		t.Fatalf("fresh TempDir already holds a bookmark: %s", filepath.Join(dir, "postgres.json"))
	}
	cfg := config.Config{
		Agent: config.Agent{ID: "cap-pg-chain", ServerAddr: "127.0.0.1:1", GameID: "demo", Env: "prod"},
		Capture: config.Capture{
			Enabled:                  true,
			SourceType:               "postgres",
			DSNEnv:                   "TEST_POSTGRES_CAPTURE_DSN",
			BookmarkDir:              dir,
			BookmarkFlushIntervalSec: 1,
			Options: map[string]string{
				"tables":      "game.items,game.ledger",
				"slotName":    slot,
				"publication": pub,
			},
		},
	}
	// Gates 内层是匿名 struct（config.Gates），按 runner_gate_test.go 先例逐字段赋值。
	cfg.Capture.Gates.Reconcile.Rules = []gate.ReconcileRule{{
		Name: "item-ledger", ItemTable: "game.items", LedgerTable: "game.ledger",
		ItemKey: "player_id", LedgerKey: "player_id", WindowSec: 2, Severity: "warn",
	}}
	cfg.Capture.Gates.CEP.Burst = []gate.BurstRule{{
		Name: "drain", Table: "game.items", DeltaField: "amount",
		Threshold: 1000, Mode: "single", Severity: "critical",
	}}
	// 事件粗过滤白名单：insert/delete 放行（本用例全部 DML 为 insert），
	// 未列入的合法操作=NoMatch、非法操作形态=Error（阶段④验证计数链）。
	cfg.Capture.Gates.Event.Ops = []string{"insert", "delete"}
	// DEBUG 日志器缓冲：失败时导出事件级痕迹（consume 的 "change event" 行
	// 带 table/op/gtid，是判定「哪条事件进了哪个角色」的地面真值）。
	var logBuf bytes.Buffer
	r := New(cfg, slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	send := &captureSend{}
	r.sendAlert = send.send

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.startHitSender(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := r.runSourceOnce(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("runSourceOnce: %v", err)
		}
	}()

	// 就绪同步：复制槽 active = START_REPLICATION 已起，此后 DML 必达。
	waitSlotActive(t, dsn, slot)

	// ① 对账闸（列名键 spec）：道具变更入窗，无流水 → tick sweep（生产销账
	// 路径：事件循环内 Evaluate、位点 tick 内 Sweep 同循环串行，FlushInterval
	// =1s，WindowSec=2 → ~2-3s 内命中）超窗产出 reconcile.missing_ledger。
	// 测试 goroutine 不得旁路并发调 r.sweepGates——闸层状态不加锁（包注释
	// 并发契约），旁路调用曾致 concurrent map writes 假失败。
	mustChainExec(t, dsn, `INSERT INTO game.items VALUES (9001, 1, 500)`)
	waitForStats(t, r, func(st Stats) bool { return st.GateHits >= 1 },
		func() string { return chainDiagnostics(&logBuf, dsn) })
	reports := waitReportCount(t, send, 1)
	rep := reports[0]
	if got := rep.GetRule(); got != "reconcile.missing_ledger" {
		t.Fatalf("report rule = %q, want reconcile.missing_ledger", got)
	}
	if got := rep.GetKey(); got != "9001" {
		t.Fatalf("report key = %q, want 9001 (column-name spec extraction)", got)
	}

	// ② 流水销账：同键流水后新变更不命中（键提取两侧对偶成立）；9002 无
	// 流水 → tick sweep 超窗命中，总命中 2。
	mustChainExec(t, dsn, `INSERT INTO game.ledger VALUES (9001, 1, 'purchase')`)
	waitForStats(t, r, func(st Stats) bool { return st.Total >= 2 })
	mustChainExec(t, dsn, `INSERT INTO game.items VALUES (9002, 2, 300)`)
	waitForStats(t, r, func(st Stats) bool { return st.GateHits >= 2 },
		func() string { return chainDiagnostics(&logBuf, dsn) })

	// ③ CEP 突变闸（列名数值 spec）：amount 超阈值即时命中，无需 sweep。
	// burst 是表级规则（设计 §8.2：无行键提取，仅 probing 降级 (行键, 表)），
	// key=空=proto 允许的 missing-key sentinel；PG 特有证据是 detail 里带
	// "delta 5000"——证明 deltaOf 按列名 "amount" 从 PG 行像取到值（MySQL 侧
	// 用位置下标键形态，本测试锁死列名形态对 PG 成立）。
	mustChainExec(t, dsn, `INSERT INTO game.items VALUES (9003, 3, 5000)`)
	waitReportCount(t, send, 3)
	last := send.snapshot()[2]
	if got := last.GetRule(); got != "cep.burst_delta" {
		t.Fatalf("report rule = %q, want cep.burst_delta", got)
	}
	if got := last.GetKey(); got != "" {
		t.Fatalf("burst key = %q, want empty (table-level rule, missing-key sentinel)", got)
	}
	if !strings.Contains(last.GetDetail(), "5000") {
		t.Fatalf("burst detail = %q, want column-name delta extraction of 5000", last.GetDetail())
	}

	// ④ 事件粗过滤（EventFilterPolicy v1）：注入白名单外非法操作形态
	// （真源不会产出非 insert/update/delete 的 op），证明进闸前有计数可
	// 观测的丢弃链——FilteredError++，不静默、不进闸。跨 goroutine 调
	// consume 之所以安全：非法 op 在事件过滤层即返回，不触闸层状态
	// （单 goroutine 并发契约只约束闸层读写）。
	r.consume(connector.ChangeEvent{
		SourceType: "postgres", Database: "game", Table: "game.items",
		Operation: connector.Operation("truncate"), ColumnsMapped: true,
		Position: connector.Position{Kind: "lsn", Value: "1"}, TsUnixMs: time.Now().UnixMilli(),
	})
	waitForStats(t, r, func(st Stats) bool { return st.FilteredError >= 1 })

	// ⑤ 位点落盘：postgres.json 存在且为合法 lsn 位点（周期 tick 落盘，
	// BookmarkFlushIntervalSec=1 → 轮询等待文件出现）。
	var raw []byte
	var rerr error
	deadline := time.Now().Add(5 * time.Second)
	for {
		raw, rerr = os.ReadFile(filepath.Join(dir, "postgres.json"))
		if rerr == nil {
			break
		}
		if !os.IsNotExist(rerr) {
			t.Fatalf("bookmark read: %v", rerr)
		}
		if time.Now().After(deadline) {
			t.Fatalf("bookmark file never appeared under %s", dir)
		}
		time.Sleep(50 * time.Millisecond)
	}
	var bm struct {
		Position string `json:"position"`
	}
	if err := json.Unmarshal(raw, &bm); err != nil {
		t.Fatalf("bookmark parse: %v", err)
	}
	pos, perr := connector.ParsePosition(bm.Position)
	if perr != nil || pos.Kind != "lsn" || pos.Value == "" {
		t.Fatalf("bookmark = %+v err = %v, want lsn position", bm, perr)
	}

	// ⑥ 重启续读：首实例退场（flush-then-close）后，新实例只收新事件。
	cancel()
	<-done
	mustChainExec(t, dsn, `INSERT INTO game.items VALUES (9004, 4, 10)`)

	r2 := New(cfg, nil)
	send2 := &captureSend{}
	r2.sendAlert = send2.send
	ctx2, cancel2 := context.WithCancel(context.Background())
	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		_ = r2.runSourceOnce(ctx2)
	}()
	// 失败路径也必须确定停机（flush 会写 BookmarkDir）：本 cleanup 注册晚于
	// t.TempDir()，LIFO 保证先停 r2 再删目录，消除落盘竞态。
	t.Cleanup(func() {
		cancel2()
		<-done2
	})
	r2.startHitSender(ctx2)
	waitSlotActive(t, dsn, slot)
	waitForStats(t, r2, func(st Stats) bool { return st.Total >= 1 })
	if total2 := r2.Stats().Total; total2 != 1 {
		t.Fatalf("resumed runner total = %d, want 1 (no replay of consumed window)", total2)
	}
}
