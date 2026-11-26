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
	// Always normalize buckets to UTC: the flush side parses the wall-clock strings in the
	// hll key with time.Parse assuming UTC. If we rendered keys here using the event's own
	// offset (or a local-timezone fallback), the two sides would be off by a timezone offset
	// on non-UTC deployments, t.Before(nowMin) would never hold, and minute_online would
	// never be flushed on CST servers (same family as the 993cb65 audit time-window issue:
	// write/parse timezones must match).
	t = t.UTC()
	game := asString(m, "game_id")
	env := asString(m, "env")
	uid := asString(m, "user_id")
	evt := normalizeAggEvent(asString(m, "event"))
	// minute online (heartbeat or session_start)
	if evt == "heartbeat" || evt == "session_start" {
		min := t.Truncate(time.Minute)
		k := fmt.Sprintf("hll:online:%s:%s:%s", game, env, min.Format("200601021504"))
		// DATA-020: the flush marker is durable, so a kill after the raw
		// event was ACKed but before the aggregate row landed still leaves
		// the next worker a flush obligation. A failed HLL write additionally
		// marks the bucket for rebuild from the persisted raw events.
		w.markAggDirty(ctx, aggDirtyMinutePrefix+k)
		if err := w.agg().PFAdd(ctx, k, uid).Err(); err != nil {
			slog.Warn("minute HLL write failed; marked for rebuild", "key", k, "err", err)
			w.markAggRebuild(ctx, aggRebuildMinutePrefix+k)
		} else {
			_ = w.agg().Expire(ctx, k, 48*time.Hour).Err()
		}
		w.touchedMinutes[k] = struct{}{}
	}
	// DAU
	if evt == "login" || evt == "session_start" {
		day := t.Format("2006-01-02")
		k := fmt.Sprintf("hll:dau:%s:%s:%s", game, env, day)
		dk := fmt.Sprintf("%s|%s|%s", day, game, env)
		w.markAggDirty(ctx, aggDirtyDayPrefix+dk)
		if err := w.agg().PFAdd(ctx, k, uid).Err(); err != nil {
			slog.Warn("dau HLL write failed; marked for rebuild", "key", k, "err", err)
			w.markAggRebuild(ctx, aggRebuildDayPrefix+"dau:"+dk)
		} else {
			_ = w.agg().Expire(ctx, k, 30*24*time.Hour).Err()
		}
		w.touchedDays[dk] = struct{}{}
	}
	// new users
	if evt == "register" || evt == "first_active" {
		day := t.Format("2006-01-02")
		k := fmt.Sprintf("hll:new:%s:%s:%s", game, env, day)
		dk := fmt.Sprintf("%s|%s|%s", day, game, env)
		w.markAggDirty(ctx, aggDirtyDayPrefix+dk)
		if err := w.agg().PFAdd(ctx, k, uid).Err(); err != nil {
			slog.Warn("new HLL write failed; marked for rebuild", "key", k, "err", err)
			w.markAggRebuild(ctx, aggRebuildDayPrefix+"new:"+dk)
		} else {
			_ = w.agg().Expire(ctx, k, 30*24*time.Hour).Err()
		}
		w.touchedDays[dk] = struct{}{}
	}
}

func (w *Worker) flush(ctx context.Context) error {
	if err := w.flushBatches(ctx); err != nil {
		slog.Warn("flush batches", "err", err)
	}
	// Snapshot/sync ownership (DATA-014): inside aggMu, move the shared state wholesale
	// into a local snapshot and clear the shared maps, then release the lock and do the
	// ClickHouse I/O against the snapshot. Writers (touchAgg/commitRevenue) are no longer
	// blocked by a long-held lock; on I/O failure the item is merged back into the shared
	// map (accumulated rather than overwritten if the writer already rebuilt the same key),
	// so failed retries lose no increments. After the snapshot transfer, new writer values
	// land in the shared map and cannot be wrongly deleted by this flush.
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

	// The wall-clock string inside the key is already normalized to UTC (touchAgg); the
	// comparison baseline is UTC too.
	nowMin := time.Now().UTC().Truncate(time.Minute)
	// flush minute_online for minutes earlier than current minute
	for k := range minSnap {
		parts := strings.Split(k, ":") // hll:online:game:env:YYYYMMDDHHmm
		if len(parts) < 5 {
			// An unparseable key is garbage data: drop it (the shared map is already
			// cleared, so it is not written back) to avoid failing retries on every flush.
			// Only I/O failures remerge to preserve the key.
			continue
		}
		ts := parts[len(parts)-1]
		t, err := time.Parse("200601021504", ts)
		if err != nil {
			// Bad time string: not flushable, drop it (original semantics: delete).
			continue
		}
		if !t.Before(nowMin) {
			// Current/future minutes: kept until they become the past, then flushed (original
			// semantics).
			w.remergeTouchedMinute(k)
			continue
		}
		game := parts[2]
		env := parts[3]
		// DATA-020: a bucket whose HLL write failed is rebuilt from the
		// persisted raw events here, before the count is read. On rebuild
		// failure, keep the bucket dirty for the next flush rather than
		// writing a false zero.
		if err := w.ensureHLL(ctx, k, aggRebuildMinutePrefix+k, 48*time.Hour); err != nil {
			slog.Warn("rebuild minute HLL", "key", k, "err", err)
			w.remergeTouchedMinute(k)
			continue
		}
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
		// The aggregate row is durable: drop the rebuild obligation. A new
		// event may have re-marked this bucket while the row was being
		// written; re-add it so a crash before the next flush still leaves
		// the obligation durable.
		w.clearAggDirty(ctx, aggDirtyMinutePrefix+k)
		w.aggMu.Lock()
		_, stillTouched := w.touchedMinutes[k]
		w.aggMu.Unlock()
		if stillTouched {
			w.markAggDirty(ctx, aggDirtyMinutePrefix+k)
		}
	}
	// flush daily_users
	for dk := range daySnap {
		sp := strings.Split(dk, "|")
		if len(sp) != 3 {
			// Bad key discarded (original semantics).
			continue
		}
		day, game, env := sp[0], sp[1], sp[2]
		kdau := fmt.Sprintf("hll:dau:%s:%s:%s", game, env, day)
		knew := fmt.Sprintf("hll:new:%s:%s:%s", game, env, day)
		// DATA-020: both daily HLLs are rebuildable from the raw events. If
		// either rebuild fails, keep the day dirty for the next flush instead
		// of persisting a partial/zero daily row.
		if err := w.ensureHLL(ctx, kdau, aggRebuildDayPrefix+"dau:"+dk, 30*24*time.Hour); err != nil {
			slog.Warn("rebuild dau HLL", "key", kdau, "err", err)
			w.remergeTouchedDay(dk)
			continue
		}
		if err := w.ensureHLL(ctx, knew, aggRebuildDayPrefix+"new:"+dk, 30*24*time.Hour); err != nil {
			slog.Warn("rebuild new HLL", "key", knew, "err", err)
			w.remergeTouchedDay(dk)
			continue
		}
		dau, err := w.rdb.PFCount(ctx, kdau).Result()
		if err != nil {
			// A PFCount failure must not overwrite the day's real DAU with 0 (ReplacingMergeTree
			// replaces by version); merge back into the shared map for the next retry (same
			// semantics as minute_online).
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
		// The daily_users row is durable: drop the rebuild obligation (both
		// the dau and new HLLs are covered by this one day marker). Re-add if
		// a new event re-marked the day during the flush, same as minutes.
		w.clearAggDirty(ctx, aggDirtyDayPrefix+dk)
		w.aggMu.Lock()
		_, stillTouched := w.touchedDays[dk]
		w.aggMu.Unlock()
		if stillTouched {
			w.markAggDirty(ctx, aggDirtyDayPrefix+dk)
		}
	}
	// flush daily_revenue
	for rk, rv := range revSnap {
		sp := strings.Split(rk, "|")
		if len(sp) != 3 {
			// Bad key kept (original semantics: continue without delete, so a retry can fix
			// it).
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

// remergeTouchedMinute merges a minute_online key whose flush failed back into the
// shared map; if the writer already rebuilt the same key, it is kept (the writer keeps
// accumulating the new value, the snapshot item is stale, so it does not overwrite).
func (w *Worker) remergeTouchedMinute(k string) {
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	if _, ok := w.touchedMinutes[k]; ok {
		return
	}
	w.touchedMinutes[k] = struct{}{}
}

// remergeTouchedDay merges a daily_users key whose flush failed back into the shared
// map (same semantics as remergeTouchedMinute).
func (w *Worker) remergeTouchedDay(k string) {
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	if _, ok := w.touchedDays[k]; ok {
		return
	}
	w.touchedDays[k] = struct{}{}
}

// remergeRev merges a daily_revenue increment whose flush failed back into the shared
// map; if the writer already rebuilt the same key, the values are accumulated (not
// overwritten), so failed retries lose no increments.
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
