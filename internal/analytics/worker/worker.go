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
		touchedMinutes:     map[string]struct{}{},
		touchedDays:        map[string]struct{}{},
		revAgg:             map[string]*revRow{},
		clickBatchSize:     batchSize,
		checkpointPrefix:   strings.TrimSuffix(envOrDefault("ANALYTICS_CHECKPOINT_PREFIX", "analytics:checkpoint"), ":"),
		lastIDs:            map[string]string{},
		deadEventsStream:   deadEvents,
		deadPaymentsStream: deadPayments,
	}, nil
}

func (w *Worker) ensureGroups(ctx context.Context) {
	_ = w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$").Err()
	_ = w.rdb.XGroupCreateMkStream(ctx, w.streamPayments, w.group, "$").Err()
	w.restoreCheckpoints(ctx, w.streamEvents)
	w.restoreCheckpoints(ctx, w.streamPayments)
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
