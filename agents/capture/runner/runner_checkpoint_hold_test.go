package runner

// AUD-059：告警持久化失败时保留可重放进度——事件上下文命中路由失败
// （spool 写失败/无 spool 队列满）时 consume 上抛，检查点不前移；源重投
// 后闸层重评、命中重生成重试。覆盖 journal / 无 journal 两种装配的重试
// 回放，以及销账命中（无源事件可重投）保持丢弃计数的边界。

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/cuihairu/croupier/agents/capture/gate"
	cdcfilter "github.com/cuihairu/croupier/pkg/cdc/filter"
	cdjournal "github.com/cuihairu/croupier/pkg/cdc/journal"
	"github.com/stretchr/testify/require"
)

// lineageHitFixture 装配 lineage 白名单闸（game.items 仅允许 app 账号），
// 非白名单账号的事件在 Evaluate 即产生命中（事件上下文命中源）。
func lineageHitFixture(t *testing.T) *Runner {
	t.Helper()
	r := New(testConfig(t, true), nil)
	lineageJSON := `{"lineage":{"rules":[{"name":"gw","table":"game.items","allowAccounts":["app"]}]}}`
	require.NoError(t, r.applyGates(context.Background(), []byte(lineageJSON)))
	return r
}

// lineageIntruderEvent 构造白名单外账号的写事件（触发 lineage 命中）。
// OpInsert 携带事件本体即可入 journal（UPDATE 需 before/after 行像）。
// TsUnixMs 必须固定：journal 按内容哈希去重，重投同一逻辑事件要求两轮
// 字节一致（真实源重投时事件体来自 binlog，不随重放墙钟变化）。
func lineageIntruderEvent(pos string) connector.ChangeEvent {
	return connector.ChangeEvent{
		SourceID: "s1", SourceType: "mysql", Database: "game", Table: "items",
		Operation: connector.OpInsert, Account: "intruder",
		Position: connector.Position{Kind: "gtid", Value: pos},
		TsUnixMs: 1700000000000,
	}
}

// brokenSpool 注入 spool 写失败（磁盘满口径）。
func brokenSpool(r *Runner) {
	r.hitSpool.writeFile = func(string, []byte) error {
		return errors.New("disk unavailable")
	}
}

func TestConsumeDurable_NoJournalRouteFailureHoldsCheckpoint(t *testing.T) {
	r := lineageHitFixture(t)
	brokenSpool(r)
	stub := &stubSource{events: []connector.ChangeEvent{lineageIntruderEvent("g-1")}}
	r.newConnector = func() connector.SourceConnector { return stub }

	err := r.runSourceOnce(context.Background())
	require.ErrorContains(t, err, "advance checkpoint")
	require.Equal(t, 0, stub.ackCnt, "checkpoint must not advance when hit persistence fails")
	require.EqualValues(t, 1, r.Stats().AlertPersistFailed)
	require.EqualValues(t, 0, r.Stats().AlertDropped)
	require.Equal(t, 1, stub.closeCnt, "source released for reopen")
}

func TestConsumeDurable_JournalAssemblyReplayRetriesHit(t *testing.T) {
	r := lineageHitFixture(t)
	journal, err := cdjournal.Open(filepath.Join(t.TempDir(), "events.jsonl"))
	require.NoError(t, err)
	defer func() { _ = journal.Close() }()
	r.eventJournal = journal
	r.journalFilter = cdcfilter.Filter{Include: []cdcfilter.Selector{{Database: "game", Table: "items"}}}

	stub := &stubSource{events: []connector.ChangeEvent{lineageIntruderEvent("g-1")}}
	r.newConnector = func() connector.SourceConnector { return stub }

	// 第一轮：命中持久化失败 → 检查点不前移（journal 已落，Append 幂等）
	brokenSpool(r)
	err = r.runSourceOnce(context.Background())
	require.ErrorContains(t, err, "advance checkpoint")
	require.Equal(t, 0, stub.ackCnt)
	require.EqualValues(t, 1, journal.LastSequence(), "event journaled before consume failed")

	// 第二轮（重开重投同一事件）：spool 恢复 → 命中重路由成功，检查点前移；
	// journal 幂等去重不重复落（事件交付后流关闭返回 errStreamEnded 可重开）
	r.hitSpool.writeFile = writeFileAtomic
	stub = &stubSource{events: []connector.ChangeEvent{lineageIntruderEvent("g-1")}}
	r.newConnector = func() connector.SourceConnector { return stub }
	err = r.runSourceOnce(context.Background())
	require.ErrorIs(t, err, errStreamEnded)
	require.Equal(t, 1, stub.ackCnt, "checkpoint advances after hit persistence recovers")
	require.EqualValues(t, 1, journal.LastSequence(), "journal append idempotent on redelivery")
	require.Equal(t, 1, r.spoolCount(), "regenerated hit deduped by spool key")
}

func TestConsumeDurable_NoSpoolQueueFullHoldsCheckpoint(t *testing.T) {
	r := lineageHitFixture(t)
	r.hitSpool = nil // 纯内存队列口径
	for i := 0; i < hitQueueSize; i++ {
		r.hitCh <- gate.Hit{Rule: "filler"}
	}
	stub := &stubSource{events: []connector.ChangeEvent{lineageIntruderEvent("g-1")}}
	r.newConnector = func() connector.SourceConnector { return stub }

	err := r.runSourceOnce(context.Background())
	require.ErrorContains(t, err, "advance checkpoint")
	require.Equal(t, 0, stub.ackCnt, "checkpoint must not advance on queue-full drop")
	require.EqualValues(t, 1, r.Stats().AlertPersistFailed)
}

func TestSweepHitsRouteFailureCountsDropNotHold(t *testing.T) {
	// 销账命中无源事件可重投：路由失败按丢弃计，不阻塞 sweep。
	r := lineageHitFixture(t)
	brokenSpool(r)
	r.recordHits([]gate.Hit{r.pendingHit()}, false)
	require.EqualValues(t, 1, r.Stats().AlertDropped)
	require.EqualValues(t, 0, r.Stats().AlertPersistFailed)
}

func TestConsumeHappyPathStillAcks(t *testing.T) {
	// 白名单内账号（无命中）→ 消费照常确认进度（既有语义不回归）。
	// 事件交付后流关闭返回 errStreamEnded（可重开），检查点已前移。
	r := lineageHitFixture(t)
	stub := &stubSource{events: []connector.ChangeEvent{{
		SourceID: "s1", SourceType: "mysql", Database: "game", Table: "items",
		Operation: connector.OpInsert, Account: "app",
		Position: connector.Position{Kind: "gtid", Value: "g-ok"},
	}}}
	r.newConnector = func() connector.SourceConnector { return stub }
	err := r.runSourceOnce(context.Background())
	require.ErrorIs(t, err, errStreamEnded)
	require.Equal(t, 1, stub.ackCnt)
}
