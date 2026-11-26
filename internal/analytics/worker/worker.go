package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	redis "github.com/redis/go-redis/v9"
)

func envOrDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

const (
	insertEventsSQL   = "INSERT INTO analytics.events (event_time, game_id, env, user_id, session_id, event, channel, platform, country, app_version, event_id, props_json)"
	insertPaymentsSQL = "INSERT INTO analytics.payments (time, game_id, env, user_id, order_id, amount_cents, currency, status, channel, platform, country, region, city, product_id, reason)"

	// Dead letter streams
	deadEventsStream   = "analytics:events:dead"
	deadPaymentsStream = "analytics:payments:dead"

	// Pending recovery settings
	pendingIdleTimeout = 5 * time.Minute
	maxPendingRetries  = 3
)

// shutdownTimeout / finalFlushTimeout are the bounded wait limits of the Run
// exit sequence (defaults match production behavior); they are injectable
// seams that tests can shorten to cover the blocking-flush and final-flush
// deadline branches (DATA-015).
var (
	shutdownTimeout   = 5 * time.Second
	finalFlushTimeout = 10 * time.Second
)

// pendingReclaimInterval is the heartbeat interval of the reclaim loop
// (default matches production behavior); an injectable seam that tests can
// shorten to cover the ticker branch.
var pendingReclaimInterval = 30 * time.Second

// runFlushInterval is the heartbeat interval of Run's periodic flush
// (default matches production behavior); an injectable seam that tests can
// shorten to cover the ticker branch.
var runFlushInterval = 15 * time.Second

// ackRef tracks a Redis stream message whose row has been appended to an
// in-memory ClickHouse batch but not yet durably Send()ed. The message must
// stay PENDING (un-ACKed) until the batch flushes, so a crash between append
// and Send redelivers it via reclaim instead of silently losing the row
// (ARCH-011: batch Append must not be ACKed before Send).
type ackRef struct {
	stream string
	id     string
	// revKey/rev carry the payment's daily-revenue delta. It is committed to
	// revAgg only after the ClickHouse batch Send succeeds; a dropped failed
	// batch must not leave an additive aggregation delta behind for reclaim.
	revKey string
	rev    *revRow
}

type Worker struct {
	rdb            *redis.Client
	ch             clickhouse.Conn
	streamEvents   string
	streamPayments string
	group          string
	consumer       string
	// groupStart is the position a freshly created consumer group starts at:
	// "0" (consume the retained backlog; the default, so a worker started
	// after the first inbound events does not silently drop them) or "$"
	// (latest only), selected by WORKER_GROUP_START. groupReset marks the
	// explicit WORKER_GROUP_RESET=true operator request, the only sanctioned
	// way to reposition an existing group (DATA-023).
	groupStart string
	groupReset bool
	// aggregation state
	aggMu          sync.Mutex
	touchedMinutes map[string]struct{}
	touchedDays    map[string]struct{}
	revAgg         map[string]*revRow
	// batching
	eventBatch       chdriver.Batch
	eventBatchRows   int
	paymentBatch     chdriver.Batch
	paymentBatchRows int
	clickBatchSize   int
	// batchMu guards the batch state and the deferred-ACK lists. The Run loop
	// (append) and the periodic flush goroutine (Send+ACK) touch these
	// concurrently; prior code raced on eventBatch/eventBatchRows.
	batchMu     sync.Mutex
	eventAcks   []ackRef
	paymentAcks []ackRef
	// lastMu guards lastIDs (written by both the Run goroutine's immediate
	// ACK path and the flush goroutine's batch-ACK path).
	lastMu sync.Mutex
	// checkpoints
	checkpointPrefix string
	lastIDs          map[string]string
	// aggregation durability (DATA-020): aggDirtySet names the Redis set that
	// records which HLL buckets still owe a flushed aggregate row, and aggRdb
	// is the command surface used by the marker/HLL-rebuild paths. aggRdb is
	// nil-safe (falls back to rdb); tests swap it to simulate a HLL write
	// failure without breaking the rest of the worker's Redis use.
	aggDirtySet   string
	aggRebuildSet string
	aggRdb        aggRedis
	// dead letter handling
	deadEventsStream   string
	deadPaymentsStream string
}

func NewWorker() (*Worker, error) {
	// Redis
	rurl := os.Getenv("REDIS_URL")
	if rurl == "" {
		rurl = "redis://localhost:6379/0"
	}
	ropt, err := redis.ParseURL(rurl)
	if err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}
	rdb := redis.NewClient(ropt)
	se := os.Getenv("ANALYTICS_REDIS_STREAM_EVENTS")
	if se == "" {
		se = "analytics:events"
	}
	sp := os.Getenv("ANALYTICS_REDIS_STREAM_PAYMENTS")
	if sp == "" {
		sp = "analytics:payments"
	}
	grp := os.Getenv("WORKER_GROUP")
	if grp == "" {
		grp = "analytics-worker"
	}
	cons := os.Getenv("WORKER_CONSUMER")
	if cons == "" {
		cons = fmt.Sprintf("c-%d", time.Now().UnixNano())
	}
	groupStart := parseGroupStart(os.Getenv("WORKER_GROUP_START"))
	groupReset := strings.EqualFold(strings.TrimSpace(os.Getenv("WORKER_GROUP_RESET")), "true")
	// ClickHouse
	dsn := os.Getenv("CLICKHOUSE_DSN")
	if dsn == "" {
		dsn = "clickhouse://localhost:9000/analytics"
	}
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse clickhouse dsn: %w", err)
	}
	// ParseDSN always yields a non-empty Addr (a DSN missing its host fails
	// outright with "parse dsn address failed"), so the empty-Addr fallback
	// branch is unreachable and has been removed.
	// clickhouse-go v2.43.0's Open unconditionally constructs the connection
	// object and returns a nil error (lazy connect, no fallible validation),
	// so the error branch is unreachable and has been removed.
	ch, _ := clickhouse.Open(opts)
	batchSize := 500
	if v := strings.TrimSpace(os.Getenv("ANALYTICS_CLICKHOUSE_BATCH")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			batchSize = n
		}
	}
	// Dead letter streams (can be overridden via env)
	deadEvents := envOrDefault("ANALYTICS_DEAD_EVENTS_STREAM", deadEventsStream)
	deadPayments := envOrDefault("ANALYTICS_DEAD_PAYMENTS_STREAM", deadPaymentsStream)

	return &Worker{
		rdb:                rdb,
		ch:                 ch,
		streamEvents:       se,
		streamPayments:     sp,
		group:              grp,
		consumer:           cons,
		groupStart:         groupStart,
		groupReset:         groupReset,
		touchedMinutes:     map[string]struct{}{},
		touchedDays:        map[string]struct{}{},
		revAgg:             map[string]*revRow{},
		clickBatchSize:     batchSize,
		checkpointPrefix:   strings.TrimSuffix(envOrDefault("ANALYTICS_CHECKPOINT_PREFIX", "analytics:checkpoint"), ":"),
		lastIDs:            map[string]string{},
		aggDirtySet:        envOrDefault("ANALYTICS_AGG_DIRTY_SET", "analytics:agg:dirty"),
		aggRebuildSet:      envOrDefault("ANALYTICS_AGG_REBUILD_SET", "analytics:agg:rebuild"),
		aggRdb:             rdb,
		deadEventsStream:   deadEvents,
		deadPaymentsStream: deadPayments,
	}, nil
}

// parseGroupStart maps WORKER_GROUP_START onto a stream position: "0"
// consumes the retained backlog (the default), "$" is latest-only. An
// invalid value falls back to the backlog-consuming default with a warning —
// silently skipping a backlog is the one behavior this pipeline must never
// default to (DATA-023).
func parseGroupStart(v string) string {
	switch strings.TrimSpace(v) {
	case "":
		return "0"
	case "0":
		return "0"
	case "$":
		return "$"
	default:
		slog.Warn("invalid WORKER_GROUP_START; using 0", "value", v)
		return "0"
	}
}

// ensureGroups implements the DATA-023 group-initialization policy:
//
//   - A group that does not exist yet is created at w.groupStart ("0" by
//     default so a worker started after the first inbound events consumes the
//     retained backlog; WORKER_GROUP_START=$ opts into latest-only). When a
//     persisted checkpoint exists the group is created AT it instead — the
//     resume-after-Redis-loss path; the checkpoint is only written after a
//     message was ACKed, so this never skips undelivered data.
//   - An existing group is never repositioned at startup: the Redis-native
//     cursor is the source of truth, so restarts keep the consumption
//     position and a stale application checkpoint cannot replay ACKed data.
//     Repositioning an existing group happens only through the explicit
//     WORKER_GROUP_RESET=true operator action.
//   - Initialization failures are logged, never silently discarded; a
//     BUSYGROUP race against another worker is expected and only traced.
func (w *Worker) ensureGroups(ctx context.Context) {
	for _, stream := range []string{w.streamEvents, w.streamPayments} {
		// Adopt the persisted checkpoint as the in-memory monotonic floor on
		// every path, so advanceCheckpoint cannot regress the stored key when
		// a late reclaim ACKs an old pending id after restart.
		w.adoptCheckpoint(ctx, stream)
		if w.groupExists(ctx, stream) {
			if w.groupReset {
				w.resetGroup(ctx, stream)
			}
			continue
		}
		pos := w.startPos()
		if id := w.checkpointFor(ctx, stream); id != "" {
			pos = id
		}
		if err := w.rdb.XGroupCreateMkStream(ctx, stream, w.group, pos).Err(); err != nil {
			if isBusyGroupErr(err) {
				// Another worker created the group first; its cursor is
				// authoritative, skip checkpoint adoption.
				slog.Info("stream group already exists", "stream", stream, "group", w.group)
				continue
			}
			slog.Warn("stream group create failed", "stream", stream, "group", w.group, "start", pos, "err", err)
			continue
		}
		slog.Info("stream group created", "stream", stream, "group", w.group, "start", pos)
	}
	w.restoreAggDirty(ctx)
}

// groupExists reports whether the consumer group already exists on the
// stream (a missing stream means it does not).
func (w *Worker) groupExists(ctx context.Context, stream string) bool {
	groups, err := w.rdb.XInfoGroups(ctx, stream).Result()
	if err != nil {
		return false
	}
	for _, g := range groups {
		if g.Name == w.group {
			return true
		}
	}
	return false
}

// resetGroup repositions an existing group to the persisted checkpoint
// (falling back to the declared start position when none exists). It runs
// only under WORKER_GROUP_RESET=true — the single sanctioned path for moving
// an existing group's cursor, because it deliberately replays data.
func (w *Worker) resetGroup(ctx context.Context, stream string) {
	id := w.checkpointFor(ctx, stream)
	if id == "" {
		id = w.startPos()
	}
	if err := w.rdb.XGroupSetID(ctx, stream, w.group, id).Err(); err != nil {
		slog.Warn("stream group reset failed", "stream", stream, "group", w.group, "id", id, "err", err)
		return
	}
	slog.Warn("stream group cursor reset by operator request", "stream", stream, "group", w.group, "id", id)
}

// startPos returns the declared group start position, normalizing an unset
// value (hand-constructed Worker in tests) to the backlog-consuming default.
func (w *Worker) startPos() string {
	if w.groupStart == "" {
		return "0"
	}
	return w.groupStart
}

func isBusyGroupErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "BUSYGROUP")
}

func (w *Worker) Run(ctx context.Context) error {
	w.ensureGroups(ctx)
	// workerCtx derives from WithoutCancel(ctx): after Run returns the child
	// goroutines can still safely finish in-flight flush/reclaim work, which
	// the wait sequence below then joins; canceling the parent ctx only stops
	// the main loop from reading new messages, without killing a goroutine
	// that is still flushing ClickHouse.
	workerCtx, stopWorkers := context.WithCancel(context.WithoutCancel(ctx))
	defer stopWorkers()

	var flushWG sync.WaitGroup
	flushWG.Add(1)
	go func() {
		defer flushWG.Done()
		ticker := time.NewTicker(runFlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				// The flush implementation always returns nil (every internal
				// failure path is logged with slog.Warn and continues), so the
				// err branch is unreachable.
				w.flush(workerCtx)
			}
		}
	}()

	var reclaimWG sync.WaitGroup
	reclaimWG.Add(1)
	go w.reclaimPendingMessages(workerCtx, &reclaimWG)

	for {
		if ctx.Err() != nil {
			break
		}
		streams := []string{w.streamEvents, w.streamPayments}
		streamIDs := make([]string, len(streams))
		for i := range streamIDs {
			streamIDs[i] = ">" // only new (unacknowledged) messages
		}
		res, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    w.group,
			Consumer: w.consumer,
			Streams:  append(streams, streamIDs...),
			Count:    200,
			Block:    2 * time.Second,
		}).Result()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			if err != redis.Nil {
				slog.Warn("xreadgroup", "err", err)
			}
			continue
		}
		for _, str := range res {
			slog.Info("stream", "name", str.Stream, "msgs", len(str.Messages))
			for _, msg := range str.Messages {
				if err := w.processMessage(ctx, str.Stream, msg); err != nil {
					slog.Warn("process message", "stream", str.Stream, "id", msg.ID, "err", err)
				}
			}
		}
	}

	// Bounded exit ordering (DATA-015): first stop the child goroutines
	// (stopWorkers), then wait for them to finish in-flight flush/reclaim work
	// with shutdownTimeout as the upper bound, and finally persist the remaining
	// pending batches and aggregate state with a separate deadline-bounded
	// flush. Worker/test configuration is no longer consulted (runFlushInterval
	// is pinned to its current value), and resources are not closed while a
	// flush is still running.
	stopWorkers()
	skipFinalFlush := false
	if !waitGroupBounded(&flushWG, shutdownTimeout) {
		slog.Warn("flush goroutine exceeded shutdown timeout", "timeout", shutdownTimeout)
		// Review finding 727: if the periodic flush is still running and the
		// final flush is started now, the two flushes execute concurrently --
		// aggMu only protects the shared state transition, while ClickHouse I/O
		// happens outside the lock, so double-writing the same key produces
		// duplicates/races. Correct approach: run the final flush only after the
		// periodic flush goroutine has fully exited; if it is stuck and times
		// out, prefer dropping this final flush (the remaining data still sits in
		// the Redis stream as pending for the restarted worker to pick up) over
		// concurrent double-writing.
		skipFinalFlush = true
	}
	if !waitGroupBounded(&reclaimWG, shutdownTimeout) {
		slog.Warn("reclaim goroutine exceeded shutdown timeout", "timeout", shutdownTimeout)
	}
	if skipFinalFlush {
		return ctx.Err()
	}

	flushCtx, cancelFlush := context.WithTimeout(context.WithoutCancel(ctx), finalFlushTimeout)
	// cancelFlush also serves as the fallback exit signal for a blocking
	// Send() that does not honor ctx: after the main flow returns on timeout,
	// the background flush goroutine can still release a test/production
	// self-held lock wait when defer cancelFlush fires, so it no longer leaks
	// permanently.
	defer cancelFlush()
	flushDone := make(chan struct{})
	flushDoneWG := &sync.WaitGroup{}
	flushDoneWG.Add(1)
	go func() {
		defer flushDoneWG.Done()
		defer close(flushDone)
		w.flush(flushCtx)
	}()
	select {
	case <-flushDone:
	case <-flushCtx.Done():
		slog.Warn("final flush exceeded deadline", "timeout", finalFlushTimeout)
	}
	if !waitGroupBounded(flushDoneWG, finalFlushTimeout) {
		slog.Warn("final flush goroutine exceeded deadline", "timeout", finalFlushTimeout)
	}
	cancelFlush()
	return ctx.Err()
}
