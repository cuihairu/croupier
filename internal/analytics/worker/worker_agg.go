package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type revRow struct {
	revenue uint64
	refunds uint64
	failed  uint64
}

// otelEventAliases maps the dotted OTel semantic event names emitted by the
// telemetry bridge (session.start, user.login, ...) onto the underscore
// names the aggregation matcher understands. Bridge-side naming follows
// OTel conventions; the worker stays the contract owner for aggregation.
var otelEventAliases = map[string]string{
	"session.start": "session_start",
	"session.end":   "session_end",
	"user.login":    "login",
	"user.register": "register",
}

func normalizeAggEvent(raw string) string {
	evt := strings.ToLower(strings.TrimSpace(raw))
	if canonical, ok := otelEventAliases[evt]; ok {
		return canonical
	}
	return evt
}

func (w *Worker) touchAgg(ctx context.Context, m map[string]any) {
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	ts := asString(m, "ts")
	t, _ := time.Parse(time.RFC3339, ts)
	if ts == "" || t.IsZero() {
		t = time.Now()
	}
	// 分桶一律归一 UTC：flush 侧 time.Parse 按 UTC 解析 hll key 里的墙钟串，
	// 这里若按事件自带 offset（或本地时区 fallback）渲染，非 UTC 部署上双方
	// 错开一个时区偏移，minute_online 在 CST 服务器上永不满足 t.Before(nowMin)
	// 而漏flush（993cb65 审计时间窗同族：写入/解析时区必须一致）。
	t = t.UTC()
	game := asString(m, "game_id")
	env := asString(m, "env")
	uid := asString(m, "user_id")
	evt := normalizeAggEvent(asString(m, "event"))
	// minute online (heartbeat or session_start)
	if evt == "heartbeat" || evt == "session_start" {
		min := t.Truncate(time.Minute)
		k := fmt.Sprintf("hll:online:%s:%s:%s", game, env, min.Format("200601021504"))
		_ = w.rdb.PFAdd(ctx, k, uid).Err()
		_ = w.rdb.Expire(ctx, k, 48*time.Hour).Err()
		w.touchedMinutes[k] = struct{}{}
	}
	// DAU
	if evt == "login" || evt == "session_start" {
		day := t.Format("2006-01-02")
		k := fmt.Sprintf("hll:dau:%s:%s:%s", game, env, day)
		_ = w.rdb.PFAdd(ctx, k, uid).Err()
		_ = w.rdb.Expire(ctx, k, 30*24*time.Hour).Err()
		w.touchedDays[fmt.Sprintf("%s|%s|%s", day, game, env)] = struct{}{}
	}
	// new users
	if evt == "register" || evt == "first_active" {
		day := t.Format("2006-01-02")
		k := fmt.Sprintf("hll:new:%s:%s:%s", game, env, day)
		_ = w.rdb.PFAdd(ctx, k, uid).Err()
		_ = w.rdb.Expire(ctx, k, 30*24*time.Hour).Err()
		w.touchedDays[fmt.Sprintf("%s|%s|%s", day, game, env)] = struct{}{}
	}
}

func (w *Worker) flush(ctx context.Context) error {
	if err := w.flushBatches(ctx); err != nil {
		slog.Warn("flush batches", "err", err)
	}
	// 快照/同步所有权（DATA-014）：在 aggMu 内把共享状态整段转移给本地
	// 快照并清空共享 map，随即释放锁，再对快照做 ClickHouse I/O。写方
	// （touchAgg/commitRevenue）不再被长持锁阻塞；I/O 失败时把该项合并
	// 回共享 map（若写方已重建同 key，则累加而非覆盖），失败重试不丢
	// 增量。快照转移后写方新值落在共享 map，不会被本次 flush 误删。
	w.aggMu.Lock()
	minSnap := make(map[string]struct{}, len(w.touchedMinutes))
	for k := range w.touchedMinutes {
		minSnap[k] = struct{}{}
	}
	w.touchedMinutes = map[string]struct{}{}
	daySnap := make(map[string]struct{}, len(w.touchedDays))
	for k := range w.touchedDays {
		daySnap[k] = struct{}{}
	}
	w.touchedDays = map[string]struct{}{}
	revSnap := make(map[string]*revRow, len(w.revAgg))
	for k, rv := range w.revAgg {
		revSnap[k] = rv
	}
	w.revAgg = map[string]*revRow{}
	w.aggMu.Unlock()

	// key 内墙钟串已归一 UTC（touchAgg），比较基准同步 UTC。
	nowMin := time.Now().UTC().Truncate(time.Minute)
	// flush minute_online for minutes earlier than current minute
	for k := range minSnap {
		parts := strings.Split(k, ":") // hll:online:game:env:YYYYMMDDHHmm
		if len(parts) < 5 {
			// 无法解析的 key 属垃圾数据：丢弃（共享 map 已清空，不重写回），
			// 避免每次 flush 都失败重试。仅 I/O 失败才 remerge 保留。
			continue
		}
		ts := parts[len(parts)-1]
		t, err := time.Parse("200601021504", ts)
		if err != nil {
			// 坏时间串：不属可 flush 范围，丢弃（原语义 delete）。
			continue
		}
		if !t.Before(nowMin) {
			// 当前/未来分钟：保留，等它成为过去后再 flush（原语义）。
			w.remergeTouchedMinute(k)
			continue
		}
		game := parts[2]
		env := parts[3]
		n, err := w.rdb.PFCount(ctx, k).Result()
		if err != nil {
			slog.Warn("pfcount", "key", k, "err", err)
			w.remergeTouchedMinute(k)
			continue
		}
		if n < 0 {
			n = 0
		}
		ver := uint64(time.Now().Unix())
		batch, err := w.ch.PrepareBatch(ctx, "INSERT INTO analytics.minute_online (m, game_id, env, online, version)")
		if err != nil {
			slog.Warn("ch batch", "err", err)
			w.remergeTouchedMinute(k)
			continue
		}
		if err := batch.Append(t, game, env, uint32(n), ver); err != nil {
			slog.Warn("batch append", "err", err)
			w.remergeTouchedMinute(k)
			continue
		}
		if err := batch.Send(); err != nil {
			slog.Warn("batch send", "err", err)
			w.remergeTouchedMinute(k)
			continue
		}
	}
	// flush daily_users
	for dk := range daySnap {
		sp := strings.Split(dk, "|")
		if len(sp) != 3 {
			// 坏 key 丢弃（原语义）。
			continue
		}
		day, game, env := sp[0], sp[1], sp[2]
		kdau := fmt.Sprintf("hll:dau:%s:%s:%s", game, env, day)
		knew := fmt.Sprintf("hll:new:%s:%s:%s", game, env, day)
		dau, err := w.rdb.PFCount(ctx, kdau).Result()
		if err != nil {
			// PFCount 失败不得以 0 覆写当日真实 DAU（ReplacingMergeTree 按
			// version 取代）；合并回共享 map 下次重试（语义同 minute_online）。
			slog.Warn("pfcount dau", "key", kdau, "err", err)
			w.remergeTouchedDay(dk)
			continue
		}
		neu, err := w.rdb.PFCount(ctx, knew).Result()
		if err != nil {
			slog.Warn("pfcount new", "key", knew, "err", err)
			w.remergeTouchedDay(dk)
			continue
		}
		d, _ := time.Parse("2006-01-02", day)
		ver := uint64(time.Now().Unix())
		batch, err := w.ch.PrepareBatch(ctx, "INSERT INTO analytics.daily_users (d, game_id, env, dau, new_users, version)")
		if err != nil {
			slog.Warn("ch daily_users", "err", err)
			w.remergeTouchedDay(dk)
			continue
		}
		if err := batch.Append(d, game, env, uint64(max0(dau)), uint64(max0(neu)), ver); err != nil {
			slog.Warn("daily_users append", "err", err)
			w.remergeTouchedDay(dk)
			continue
		}
		if err := batch.Send(); err != nil {
			slog.Warn("daily_users send", "err", err)
			w.remergeTouchedDay(dk)
			continue
		}
	}
	// flush daily_revenue
	for rk, rv := range revSnap {
		sp := strings.Split(rk, "|")
		if len(sp) != 3 {
			// 坏 key 保留（原语义：continue 不 delete，重试可修复）。
			w.remergeRev(rk, rv)
			continue
		}
		day, game, env := sp[0], sp[1], sp[2]
		d, _ := time.Parse("2006-01-02", day)
		ver := uint64(time.Now().Unix())
		batch, err := w.ch.PrepareBatch(ctx, "INSERT INTO analytics.daily_revenue (d, game_id, env, revenue_cents, refunds_cents, failed, version)")
		if err != nil {
			slog.Warn("ch daily_revenue", "err", err)
			w.remergeRev(rk, rv)
			continue
		}
		if err := batch.Append(d, game, env, rv.revenue, rv.refunds, rv.failed, ver); err != nil {
			slog.Warn("daily_revenue append", "err", err)
			w.remergeRev(rk, rv)
			continue
		}
		if err := batch.Send(); err != nil {
			slog.Warn("daily_revenue send", "err", err)
			w.remergeRev(rk, rv)
			continue
		}
	}
	return nil
}

// remergeTouchedMinute 把 flush 失败的 minute_online key 合并回共享 map；
// 若写方已重建同 key，保留（新值由写方继续累积，快照项为旧值，不覆盖）。
func (w *Worker) remergeTouchedMinute(k string) {
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	if _, ok := w.touchedMinutes[k]; ok {
		return
	}
	w.touchedMinutes[k] = struct{}{}
}

// remergeTouchedDay 把 flush 失败的 daily_users key 合并回共享 map（语义同
// remergeTouchedMinute）。
func (w *Worker) remergeTouchedDay(k string) {
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	if _, ok := w.touchedDays[k]; ok {
		return
	}
	w.touchedDays[k] = struct{}{}
}

// remergeRev 把 flush 失败的 daily_revenue 增量合并回共享 map；若写方已
// 重建同 key，则累加（不覆盖），保证失败重试不丢增量。
func (w *Worker) remergeRev(k string, rv *revRow) {
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	cur := w.revAgg[k]
	if cur == nil {
		w.revAgg[k] = rv
		return
	}
	cur.revenue += rv.revenue
	cur.refunds += rv.refunds
	cur.failed += rv.failed
}

func max0(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}
