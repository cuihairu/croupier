package worker

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// revProgressSeenDefaultTTL bounds the growth of the per-message dedup markers
// used by commitRevenueDurable. A marker only has to outlive the window in
// which the source message can still be redelivered by reclaim (pendingIdleTimeout
// plus a flush/restart cycle), so a week of headroom is enough in practice.
const revProgressSeenDefaultTTL = 7 * 24 * time.Hour

// revProgressTTL is how long the cumulative daily revenue progress is kept in
// Redis after its last advance. A day-scoped key stops advancing once the day
// is over and all its rows are flushed; a generous TTL keeps the cumulative
// total available for late redeliveries or a manual re-flush.
const revProgressTTL = 180 * 24 * time.Hour

// revProgressSeenMarker namespaces the per-message dedup marker keys written
// by commitRevenueDurable. The stream is part of the key so events and
// payments never collide.
func revProgressSeenMarker(stream, id string) string {
	return "analytics:rev:seen:" + stream + ":" + id
}

// commitRevenueScript advances the durable cumulative daily revenue progress
// for a batch of payment messages and marks every advanced revKey dirty, so a
// crash between payment Send+XACK and the daily_revenue insert leaves an
// explicit flush obligation in Redis (DATA-021).
//
// KEYS[1] = rev progress hash, KEYS[2] = dirty aggregate set.
// ARGV[1] = version seed (unix seconds), ARGV[2] = marker TTL ms, ARGV[3] =
// dirty member prefix, ARGV[4] = message count N. Then per message, 5 slots:
// [seen marker key, revKey, revenue delta, refunds delta, failed delta]. Lua
// ARGV indices are 1-based, so message i lives at ARGV[F+1+5i] with F=4; the
// base is derived from F instead of hardcoded to keep that mapping obvious.
// A message whose marker already exists (reclaim redelivery of an
// already-committed message) is skipped entirely. The version is an atomic
// Redis INCR on a per-revKey field, so a later flush always carries a
// strictly greater ReplacingMergeTree version than an earlier one, across
// workers. On the very first advance the base is lifted to the seed + 1:
// rows written before DATA-021 used unix seconds at flush time as their
// version, and a raw INCR (1, 2, ...) would lose the ReplacingMergeTree
// comparison against every one of them.
//
// The script returns a flat [revKey, version, revKey, version, ...] list.
const commitRevenueScript = `
local seed = tonumber(ARGV[1])
local hkey, skey = KEYS[1], KEYS[2]
local ttl = ARGV[2]
local prefix = ARGV[3]
local n = tonumber(ARGV[4])
local F = 4
local touched = {}
for i = 0, n - 1 do
  local b = F + 1 + i * 5
  local seenKey, revKey = ARGV[b], ARGV[b + 1]
  if redis.call('SET', seenKey, '1', 'PX', ttl, 'NX') then
    redis.call('HINCRBY', hkey, revKey .. ':rev', tonumber(ARGV[b + 2]))
    redis.call('HINCRBY', hkey, revKey .. ':ref', tonumber(ARGV[b + 3]))
    redis.call('HINCRBY', hkey, revKey .. ':fail', tonumber(ARGV[b + 4]))
    touched[revKey] = true
  end
end
local out = {}
for revKey, _ in pairs(touched) do
  redis.call('SADD', skey, prefix .. revKey)
  local vf = revKey .. ':ver'
  local v = redis.call('HINCRBY', hkey, vf, 1)
  if v == 1 then
    v = seed + 1
    redis.call('HSET', hkey, vf, v)
  end
  out[#out + 1] = revKey
  out[#out + 1] = v
end
return out
`

// revProgressKey is the Redis hash field for a revKey and a component.
func revProgressKey(revKey, component string) string {
	return revKey + ":" + component
}

// commitRevenueDurable folds a batch of payment refs' revenue deltas into the
// durable cumulative progress (a Redis hash) and marks every advanced revKey
// dirty, replacing the in-memory-only commitRevenue for payment batches. The
// advance and the per-message dedup happen inside one Lua script, so it is
// atomic against concurrent workers. aggMu is held across the script and the
// revAgg refresh so a concurrent flush cannot clear a marker while the advance
// it covers is still in flight.
//
// On Redis failure the function returns an error and revAgg is untouched; the
// caller leaves the source messages pending and retries the whole advance —
// the seen-marker dedup keeps already-advanced messages out of the retry's
// cumulative add, so the total is never double counted.
func (w *Worker) commitRevenueDurable(ctx context.Context, acks []ackRef) error {
	if w.revProgressSet == "" {
		// No durable progress configured (a hand-built Worker in tests): fall
		// back to the in-memory-only fold, matching the pre-DATA-021 behavior.
		w.commitRevenue(acks)
		return nil
	}
	type item struct {
		markerKey string
		revKey    string
		rev       uint64
		ref       uint64
		fail      uint64
	}
	items := make([]item, 0, len(acks))
	seen := make(map[string]struct{}, len(acks))
	for _, ack := range acks {
		if ack.rev == nil {
			continue
		}
		key := holdKey(ack.stream, ack.id)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, item{
			markerKey: revProgressSeenMarker(ack.stream, ack.id),
			revKey:    ack.revKey,
			rev:       ack.rev.revenue,
			ref:       ack.rev.refunds,
			fail:      ack.rev.failed,
		})
	}
	if len(items) == 0 {
		return nil
	}

	w.aggMu.Lock()
	defer w.aggMu.Unlock()

	argv := make([]any, 0, 4+len(items)*5)
	argv = append(argv,
		time.Now().Unix(), // version seed: floor above legacy wall-clock versions
		w.revProgressSeenTTL.Milliseconds(),
		aggDirtyRevenuePrefix,
		len(items),
	)
	for _, it := range items {
		argv = append(argv, it.markerKey, it.revKey,
			strconv.FormatUint(it.rev, 10), strconv.FormatUint(it.ref, 10), strconv.FormatUint(it.fail, 10))
	}
	res, err := w.rdb.Eval(ctx, commitRevenueScript, []string{w.revProgressSet, w.aggDirtySet}, argv...).Result()
	if err != nil {
		return err
	}
	// The script returns a flat [revKey, version, revKey, version, ...] list.
	list, ok := res.([]any)
	if !ok {
		return nil
	}
	advanced := make(map[string]uint64, len(list)/2)
	for i := 0; i+1 < len(list); i += 2 {
		revKey, _ := list[i].(string)
		version, ok := list[i+1].(int64)
		if !ok || revKey == "" {
			continue
		}
		advanced[revKey] = uint64(version)
	}
	if len(advanced) == 0 {
		return nil
	}
	progress, err := w.rdb.HGetAll(ctx, w.revProgressSet).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	for revKey, version := range advanced {
		rv := revRowFromProgress(progress, revKey)
		rv.version = version
		w.revAgg[revKey] = rv
	}
	_ = w.rdb.Expire(ctx, w.revProgressSet, revProgressTTL).Err()
	return nil
}

// revRowFromProgress builds a revRow from the durable hash for one revKey.
func revRowFromProgress(progress map[string]string, revKey string) *revRow {
	rv := &revRow{}
	for _, comp := range []string{"rev", "ref", "fail"} {
		v, ok := progress[revProgressKey(revKey, comp)]
		if !ok || strings.TrimSpace(v) == "" {
			continue
		}
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			slog.Warn("revenue progress field not numeric", "key", revKey, "comp", comp, "val", v)
			continue
		}
		switch comp {
		case "rev":
			rv.revenue = n
		case "ref":
			rv.refunds = n
		case "fail":
			rv.failed = n
		}
	}
	return rv
}

// restoreAggRevenue reloads the durable cumulative revenue progress for every
// revKey marked dirty, so the next flush writes the owed total instead of a
// stale snapshot or a false zero. Called from ensureGroups at worker start
// alongside restoreAggDirty (DATA-021).
func (w *Worker) restoreAggRevenue(ctx context.Context) {
	if w.aggDirtySet == "" || w.revProgressSet == "" {
		return
	}
	members, err := w.agg().SMembers(ctx, w.aggDirtySet).Result()
	if err != nil {
		if err != redis.Nil {
			slog.Warn("agg revenue restore: dirty read", "err", err)
		}
		return
	}
	revKeys := make([]string, 0, len(members))
	for _, m := range members {
		if !strings.HasPrefix(m, aggDirtyRevenuePrefix) {
			continue
		}
		revKeys = append(revKeys, strings.TrimPrefix(m, aggDirtyRevenuePrefix))
	}
	if len(revKeys) == 0 {
		return
	}
	progress, err := w.rdb.HGetAll(ctx, w.revProgressSet).Result()
	if err != nil && err != redis.Nil {
		slog.Warn("agg revenue restore: progress read", "err", err)
		return
	}
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	for _, revKey := range revKeys {
		if _, exists := w.revAgg[revKey]; exists {
			continue
		}
		// A hash with no component fields cannot be restored: writing the
		// resulting zeros would overwrite the real ClickHouse rows (a fresh
		// wall-clock version wins the ReplacingMergeTree comparison). Leave
		// the dirty marker in place and surface the state instead.
		if _, ok := progress[revProgressKey(revKey, "rev")]; !ok {
			if _, ok := progress[revProgressKey(revKey, "ref")]; !ok {
				if _, ok := progress[revProgressKey(revKey, "fail")]; !ok {
					slog.Warn("agg revenue restore: progress fields missing", "key", revKey)
					continue
				}
			}
		}
		rv := revRowFromProgress(progress, revKey)
		if v, err := strconv.ParseUint(progress[revProgressKey(revKey, "ver")], 10, 64); err == nil {
			rv.version = v
		}
		w.revAgg[revKey] = rv
	}
}

// clearRevenueProgress clears a revKey's dirty marker once its cumulative row
// has been persisted (the daily_revenue insert in flush()).
func (w *Worker) clearRevenueProgress(ctx context.Context, revKey string) {
	w.clearAggDirty(ctx, aggDirtyRevenuePrefix+revKey)
}
