//go:build integration

package pg

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"

	// pgx stdlib：database/sql 驱动注册（walsender 计数探测用普通连接）。
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Poll 启动序列失败不得残留 walsender（todo 4301 增量：重开循环 55006 级联
// 耗尽 max_wal_senders 的现场回归）。槽被 A 连接器占用时，B 连接器的 Poll
// 必在 START_REPLICATION 处报 55006——此前的实现直接 return，B 刚拨好的
// replication 连接以 startup walsender 形态挂在 PG 侧永不释放；agent 重开
// 循环逐轮泄漏一个，直至 max_wal_senders 耗尽、全员无法重连。
//
// 环境变量：TEST_POSTGRES_CAPTURE_DSN（缺席跳过；与 runner_pg_chain 集成
// 测试同跑法）。
func TestPostgresConnectorPollStartErrorDropsReplicationConn(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_CAPTURE_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_CAPTURE_DSN not set")
	}
	probe, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("probe open: %v", err)
	}
	defer func() { _ = probe.Close() }()

	slot := "cap_leak_" + strconv.FormatInt(time.Now().UnixNano()%1_000_000, 10)
	cfgA := connector.SourceConfig{
		SourceID: "leak-a", SourceType: "postgres", DSNEnv: "TEST_POSTGRES_CAPTURE_DSN",
		Options: map[string]string{
			"slotName":    slot,
			"publication": "cap_e2e_pub",
		},
	}

	// A：空 From → resolveStart 自动建槽并流式占住。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := NewPostgresConnector(cfgA, slog.New(slog.DiscardHandler))
	if err := a.Open(ctx, cfgA); err != nil {
		t.Fatalf("A open: %v", err)
	}
	eventsA, err := a.Poll(ctx, connector.PollRequest{Tables: []string{"game.items"}})
	if err != nil {
		t.Fatalf("A poll: %v", err)
	}
	waitWalsenderCount(t, probe, 1, 10*time.Second)

	// B：同槽第二连接器 → START_REPLICATION 55006。
	b := NewPostgresConnector(cfgA, slog.New(slog.DiscardHandler))
	if err := b.Open(ctx, cfgA); err != nil {
		t.Fatalf("B open: %v", err)
	}
	_, err = b.Poll(ctx, connector.PollRequest{Tables: []string{"game.items"}})
	if err == nil {
		t.Fatal("B poll must fail while slot is held by A")
	}
	if !strings.Contains(err.Error(), "is active for PID") && !strings.Contains(err.Error(), "55006") {
		t.Fatalf("B poll error = %v, want slot-active 55006", err)
	}

	// 核心断言：B 的失败连接必须立即弃置——walsender 计数回落到 1（仅剩
	// A）。泄漏形态下 B 的 startup sender 永久残留，此断言超时。
	waitWalsenderCount(t, probe, 1, 5*time.Second)

	// A 仍持有槽（B 的失败不影响在途流）。
	var active bool
	if err := probe.QueryRow(`SELECT active FROM pg_replication_slots WHERE slot_name = $1`, slot).Scan(&active); err != nil || !active {
		t.Fatalf("slot %s active = %v err = %v, want held by A", slot, active, err)
	}

	// 收尾：A 退场后再删槽（持久槽留给位点恢复，不随连接删除）。
	drainA(eventsA)
	cancel()
	_ = a.Close(context.Background())
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := probe.Exec(`SELECT pg_drop_replication_slot($1) WHERE EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name = $1)`, slot)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Logf("drop slot %s: %v (leftover, cleanup best effort)", slot, err)
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// drainA 消费 A 的事件通道（后台）：不消费会因 64 容量背压堵住 pump，
// 不影响本用例断言，但收尾时避免 goroutine 悬挂。
func drainA(events <-chan connector.ChangeEvent) {
	go func() {
		for range events {
		}
	}()
}

// waitWalsenderCount 等待 pg_stat_replication 行数到达 n（walsender 全集，
// 探测连接自身不在此视图）。
func waitWalsenderCount(t *testing.T, db *sql.DB, n int, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM pg_stat_replication`).Scan(&count); err == nil && count == n {
			return
		}
		if time.Now().After(deadline) {
			rows, qerr := db.Query(`SELECT pid, state FROM pg_stat_replication ORDER BY pid`)
			detail := fmt.Sprintf("walsender count never reached %d", n)
			if qerr == nil {
				defer func() { _ = rows.Close() }()
				for rows.Next() {
					var pid int
					var state string
					_ = rows.Scan(&pid, &state)
					detail += fmt.Sprintf("; pid=%d state=%s", pid, state)
				}
			}
			t.Fatalf("%s", detail)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
