package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// Marker prefixes tag the durable aggregate bookkeeping set members so a
// worker restart can tell which Redis HLL buckets still owe a flushed
// aggregate row, and which of those need their HLL re-derived from the
// persisted raw events (DATA-020). The revenue prefix (DATA-021) marks a
// revKey whose cumulative progress still owes a daily_revenue flush.
const (
	aggDirtyMinutePrefix   = "m:"
	aggDirtyDayPrefix      = "d:"
	aggRebuildMinutePrefix = "rm:"
	aggRebuildDayPrefix    = "rd:"
	aggDirtyRevenuePrefix  = "r:"
)

// aggRedis is the Redis command surface used by the aggregation durability
// paths (dirty/rebuild markers + HLL rebuild). *redis.Client satisfies it;
// tests swap it to simulate an HLL write failure without breaking the rest
// of the worker's Redis use. w.agg() falls back to w.rdb when aggRdb is nil.
type aggRedis interface {
	PFAdd(ctx context.Context, key string, els ...interface{}) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SMembers(ctx context.Context, key string) *redis.StringSliceCmd
	SIsMember(ctx context.Context, key string, member interface{}) *redis.BoolCmd
}

// agg returns the Redis surface for the aggregation durability paths.
func (w *Worker) agg() aggRedis {
	if w.aggRdb != nil {
		return w.aggRdb
	}
	return w.rdb
}

// markAggDirty durably records that a bucket still owes a flushed aggregate
// row. It survives a worker crash, so a kill after the raw event was ACKed
// but before the aggregate row landed still leaves a flush obligation for
// the next worker.
func (w *Worker) markAggDirty(ctx context.Context, marker string) {
	if w.aggDirtySet == "" {
		return
	}
	if err := w.agg().SAdd(ctx, w.aggDirtySet, marker).Err(); err != nil {
		slog.Warn("agg dirty marker", "marker", marker, "err", err)
	}
}

// markAggRebuild durably records that a bucket's HLL needs re-derivation from
// the persisted raw events (its best-effort HLL write failed). Distinct from
// the flush marker: the flush marker says "row owed", the rebuild marker
// says "the HLL in Redis is not trustworthy".
func (w *Worker) markAggRebuild(ctx context.Context, marker string) {
	if w.aggRebuildSet == "" {
		return
	}
	if err := w.agg().SAdd(ctx, w.aggRebuildSet, marker).Err(); err != nil {
		slog.Warn("agg rebuild marker", "marker", marker, "err", err)
	}
}

// isAggRebuild reports whether a bucket's HLL must be rebuilt from raw
// events before its count can be read.
func (w *Worker) isAggRebuild(ctx context.Context, marker string) bool {
	if w.aggRebuildSet == "" {
		return false
	}
	return w.agg().SIsMember(ctx, w.aggRebuildSet, marker).Val()
}

// clearAggRebuild clears a bucket's rebuild marker once its HLL has been
// rebuilt from raw events.
func (w *Worker) clearAggRebuild(ctx context.Context, marker string) {
	if w.aggRebuildSet == "" {
		return
	}
	if err := w.agg().SRem(ctx, w.aggRebuildSet, marker).Err(); err != nil {
		slog.Warn("agg rebuild clear", "marker", marker, "err", err)
	}
}

// clearAggDirty clears a bucket's flush marker after its aggregate row has
// been persisted.
func (w *Worker) clearAggDirty(ctx context.Context, marker string) {
	if w.aggDirtySet == "" {
		return
	}
	if err := w.agg().SRem(ctx, w.aggDirtySet, marker).Err(); err != nil {
		slog.Warn("agg dirty clear", "marker", marker, "err", err)
	}
}

// restoreAggDirty reloads the durable markers into the in-memory touched
// maps at worker start, so aggregate state that was marked before a crash is
// still flushed after the restart (the in-memory maps alone are empty on
// start). Rebuild markers are not loaded into memory: the flush path checks
// them against Redis so a worker that crashes mid-rebuild still rebuilds.
func (w *Worker) restoreAggDirty(ctx context.Context) {
	if w.aggDirtySet == "" {
		return
	}
	members, err := w.agg().SMembers(ctx, w.aggDirtySet).Result()
	if err != nil {
		if err != redis.Nil {
			slog.Warn("agg dirty restore", "err", err)
		}
		return
	}
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	for _, m := range members {
		switch {
		case strings.HasPrefix(m, aggDirtyMinutePrefix):
			w.touchedMinutes[strings.TrimPrefix(m, aggDirtyMinutePrefix)] = struct{}{}
		case strings.HasPrefix(m, aggDirtyDayPrefix):
			w.touchedDays[strings.TrimPrefix(m, aggDirtyDayPrefix)] = struct{}{}
		}
	}
}

// aggEventVariants maps a bucket kind to the raw event name spellings whose
// users count toward it: touchAgg counts an event under its canonical name
// (login/session_start/...), but the raw events table stores whatever the
// producer (or the OTel bridge, dotted names) emitted. The rebuild query
// must match both or it undercounts bridged traffic.
var aggEventVariants = map[string][]string{
	"online": {"heartbeat", "session_start", "session.start"},
	"dau":    {"login", "session_start", "user.login", "session.start"},
	"new":    {"register", "first_active", "user.register"},
}

// aggHLLBucket describes one HLL bucket: its key, and the raw-event window
// that rebuilds it from ClickHouse. kind selects the aggregate that consumes
// the count and the TTL applied to a rebuilt HLL.
type aggHLLBucket struct {
	kind string // "online", "dau" or "new"
	key  string
	game string
	env  string
	from time.Time
	to   time.Time
}

// parseAggHLLKey parses the hll:<kind>:<game>:<env>:<id> key written by
// touchAgg into its rebuild parameters. The bucket id is always the last
// ':'-separated segment; game/env are parts[2]/parts[3]. game/env values
// containing ':' are not supported by the key format itself (touchAgg
// renders them with ':' separators), so such buckets are simply not
// rebuildable — the rebuild is skipped and the flush marker is still cleared
// after a normal flush.
func parseAggHLLKey(key string) (aggHLLBucket, bool) {
	parts := strings.Split(key, ":")
	if len(parts) != 5 || parts[0] != "hll" {
		return aggHLLBucket{}, false
	}
	bucket := aggHLLBucket{kind: parts[1], key: key, game: parts[2], env: parts[3]}
	id := parts[4]
	switch parts[1] {
	case "online":
		t, err := time.Parse("200601021504", id)
		if err != nil {
			return aggHLLBucket{}, false
		}
		bucket.from = t.UTC()
		bucket.to = bucket.from.Add(time.Minute)
		return bucket, true
	case "dau", "new":
		t, err := time.Parse("2006-01-02", id)
		if err != nil {
			return aggHLLBucket{}, false
		}
		bucket.from = t.UTC()
		bucket.to = bucket.from.Add(24 * time.Hour)
		return bucket, true
	}
	return aggHLLBucket{}, false
}

// rebuildHLL re-derives one HLL bucket from the persisted raw events when
// the Redis HLL is not trustworthy (its write failed). PFADD is idempotent
// on an HLL (union by user id), so rebuilding onto an existing key cannot
// double count even if some of those events already made it into the HLL
// before the failure. A rebuild with no matching events is a no-op: flush
// then writes a truthful 0 (PFCount of a missing key) and clears the
// rebuild marker.
func (w *Worker) rebuildHLL(ctx context.Context, b aggHLLBucket, ttl time.Duration) error {
	uids, err := w.queryDistinctUsers(ctx, b.game, b.env, b.from, b.to, aggEventVariants[b.kind])
	if err != nil {
		return err
	}
	if len(uids) == 0 {
		return nil
	}
	members := make([]interface{}, len(uids))
	for i, u := range uids {
		members[i] = u
	}
	if err := w.agg().PFAdd(ctx, b.key, members...).Err(); err != nil {
		return err
	}
	return w.agg().Expire(ctx, b.key, ttl).Err()
}

// queryDistinctUsers returns the distinct user_ids in the raw events table
// for a game/env window and event set. The IN list is built from
// aggEventVariants (fixed strings), never from the key, so no user input
// reaches the query text.
func (w *Worker) queryDistinctUsers(ctx context.Context, game, env string, from, to time.Time, events []string) ([]string, error) {
	in := strings.TrimSuffix(strings.Repeat("?,", len(events)), ",")
	q := fmt.Sprintf("SELECT DISTINCT user_id FROM analytics.events WHERE game_id = ? AND env = ? AND event_time >= ? AND event_time < ? AND event IN (%s)", in)
	args := []any{game, env, from, to}
	for _, e := range events {
		args = append(args, e)
	}
	var uids []string
	if err := w.ch.Select(ctx, &uids, q, args...); err != nil {
		return nil, err
	}
	return uids, nil
}

// ensureHLL rebuilds a bucket's HLL from the persisted raw events when its
// rebuild marker is set (the best-effort HLL write failed). Returns an error
// only when the bucket must be retried (Redis or ClickHouse unavailable);
// callers keep the bucket dirty in that case.
func (w *Worker) ensureHLL(ctx context.Context, key, marker string, ttl time.Duration) error {
	if !w.isAggRebuild(ctx, marker) {
		return nil
	}
	bucket, ok := parseAggHLLKey(key)
	if !ok {
		// Unparseable key: cannot rebuild. Drop the rebuild marker so the
		// flush is not retried forever on a malformed bucket.
		w.clearAggRebuild(ctx, marker)
		return nil
	}
	if err := w.rebuildHLL(ctx, bucket, ttl); err != nil {
		return err
	}
	w.clearAggRebuild(ctx, marker)
	return nil
}
