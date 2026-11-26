package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type redisQueue struct {
	cli            *redis.Client
	streamEvents   string
	streamPayments string
	maxLenApprox   bool
	// maxLen is the cleanup ceiling, not a head-eviction quota (DATA-048):
	// above it, the stream is trimmed up to the consumer-progress boundary
	// (see trimToSafeBoundary); undelivered/un-ACKed entries are never
	// evicted and retained may stay above the cap while work is stalled.
	maxLen int64
	// group is the consumer group whose progress defines the backlog
	// (DATA-047); it mirrors the worker's WORKER_GROUP default.
	group string
	// groupsFn seam: XINFO GROUPS lookup, overridable in tests — miniredis's
	// group lag is not faithful to real Redis, so backlog assertions inject
	// their readings here.
	groupsFn func(ctx context.Context, stream string) ([]redis.XInfoGroup, error)
}

// NewRedis builds the Redis streams publisher. An unparseable address is an
// initialization error (DATA-043), not a silent fallback to noop: the ingest
// service exists to forward events, so starting with an empty publisher would
// acknowledge publishes while dropping everything. Empty-write mode is only
// reachable through the explicit ANALYTICS_MQ_TYPE=noop factory branch.
func NewRedis(url, streamEvents, streamPayments string, maxLen int64, approx bool) (Queue, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	cli := redis.NewClient(opt)
	group := os.Getenv("WORKER_GROUP")
	if group == "" {
		group = "analytics-worker"
	}
	return &redisQueue{
		cli: cli, streamEvents: streamEvents, streamPayments: streamPayments,
		maxLen: maxLen, maxLenApprox: approx,
		group:    group,
		groupsFn: defaultGroupsFn(cli),
	}, nil
}

func newRedisFromEnv() (Queue, error) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379/0"
	}
	se := os.Getenv("ANALYTICS_REDIS_STREAM_EVENTS")
	if se == "" {
		se = "analytics:events"
	}
	sp := os.Getenv("ANALYTICS_REDIS_STREAM_PAYMENTS")
	if sp == "" {
		sp = "analytics:payments"
	}
	ml := int64(1000000)
	if v := os.Getenv("ANALYTICS_REDIS_MAXLEN"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			ml = n
		}
	}
	approx := true
	if v := os.Getenv("ANALYTICS_REDIS_MAXLEN_APPROX"); v != "" {
		approx = (v == "1" || v == "true" || v == "yes")
	}
	return NewRedis(url, se, sp, ml, approx)
}

func (q *redisQueue) Close() error { return q.cli.Close() }

func (q *redisQueue) xadd(ctx context.Context, stream string, m map[string]any) error {
	// Store as single field 'data' with JSON body for schema flexibility
	b, _ := json.Marshal(m)
	// DATA-048: the cap never evicts blind from the head. Add first without
	// MaxLen, then trim only up to the boundary every consumer group has
	// fully consumed, so entries that are undelivered or pending ACK survive
	// a worker outage even when the backlog exceeds ANALYTICS_REDIS_MAXLEN.
	if err := q.cli.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: map[string]any{"data": string(b)}}).Err(); err != nil {
		return err
	}
	if q.maxLen > 0 {
		q.trimToSafeBoundary(ctx, stream)
	}
	return nil
}

// trimToSafeBoundary applies the safe-cleanup boundary once the stream is
// above the configured cap. The read is best-effort and fail-safe: any
// progress-read failure retains everything and the next publish re-checks.
func (q *redisQueue) trimToSafeBoundary(ctx context.Context, stream string) {
	retained, err := q.cli.XLen(ctx, stream).Result()
	if err != nil || retained <= q.maxLen {
		return
	}
	boundary, ok := q.safeTrimID(ctx, stream)
	if !ok {
		return
	}
	if q.maxLenApprox {
		_, _ = q.cli.XTrimMinIDApprox(ctx, stream, boundary, 0).Result()
		return
	}
	_, _ = q.cli.XTrimMinID(ctx, stream, boundary).Result()
}

// safeTrimID returns the exclusive trim boundary: XTRIM MINID at this id
// removes exactly the entries every consumer group on the stream has both
// delivered and acknowledged. It reports false when nothing is safely
// removable — no group exists yet (DATA-023 creates the group from the
// stream head on first worker start, so every retained entry is undelivered)
// — or when any group's progress cannot be read.
func (q *redisQueue) safeTrimID(ctx context.Context, stream string) (string, bool) {
	gs, err := q.groupsFn(ctx, stream)
	if err != nil || len(gs) == 0 {
		return "", false
	}
	boundary := ""
	for i := range gs {
		g := &gs[i]
		// Oldest entry this group still needs; XTRIM MINID keeps it and
		// everything after.
		need := g.LastDeliveredID
		if g.Pending > 0 {
			pe, err := q.cli.XPendingExt(ctx, &redis.XPendingExtArgs{
				Stream: stream, Group: g.Name, Start: "-", End: "+", Count: 1,
			}).Result()
			if err != nil || len(pe) == 0 {
				return "", false
			}
			need = pe[0].ID
		} else {
			// Fully acknowledged through last-delivered: keep only entries
			// strictly after it.
			ms, seq, err := splitStreamID(g.LastDeliveredID)
			if err != nil {
				return "", false
			}
			need = fmt.Sprintf("%d-%d", ms, seq+1)
		}
		// Pin on the slowest group: another consumer may still need entries
		// this group has finished with.
		if boundary == "" || streamIDBefore(need, boundary) {
			boundary = need
		}
	}
	return boundary, true
}

func splitStreamID(id string) (int64, int64, error) {
	msStr, seqStr, _ := strings.Cut(id, "-")
	ms, err := strconv.ParseInt(msStr, 10, 64)
	if err != nil {
		return 0, 0, err
	}
	seq := int64(0)
	if seqStr != "" {
		if seq, err = strconv.ParseInt(seqStr, 10, 64); err != nil {
			return 0, 0, err
		}
	}
	return ms, seq, nil
}

// streamIDBefore reports whether Redis stream id a sorts strictly before b.
func streamIDBefore(a, b string) bool {
	aMs, aSeq, errA := splitStreamID(a)
	bMs, bSeq, errB := splitStreamID(b)
	if errA != nil || errB != nil {
		return false
	}
	if aMs != bMs {
		return aMs < bMs
	}
	return aSeq < bSeq
}

func (q *redisQueue) PublishEvent(evt map[string]any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return q.xadd(ctx, q.streamEvents, evt)
}

func (q *redisQueue) PublishPayment(pay map[string]any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return q.xadd(ctx, q.streamPayments, pay)
}

func defaultGroupsFn(cli *redis.Client) func(ctx context.Context, stream string) ([]redis.XInfoGroup, error) {
	return func(ctx context.Context, stream string) ([]redis.XInfoGroup, error) {
		return cli.XInfoGroups(ctx, stream).Result()
	}
}

// Backlog reports the DATA-047 split readings for both streams: retained
// physical length (XLEN), undelivered lag and pending-ack counts from the
// worker group's progress. An XLEN failure fails the probe; a group-progress
// failure degrades that stream's undelivered/pendingAck to unknown instead of
// failing the whole reading.
func (q *redisQueue) Backlog(ctx context.Context) (QueueBacklog, error) {
	ev, err := q.backlogFor(ctx, q.streamEvents)
	if err != nil {
		return QueueBacklog{}, err
	}
	pay, err := q.backlogFor(ctx, q.streamPayments)
	if err != nil {
		return QueueBacklog{Events: ev}, err
	}
	return QueueBacklog{Events: ev, Payments: pay}, nil
}

func (q *redisQueue) backlogFor(ctx context.Context, stream string) (StreamBacklog, error) {
	retained, err := q.cli.XLen(ctx, stream).Result()
	if err != nil {
		return StreamBacklog{}, err
	}
	b := StreamBacklog{Retained: retained}
	gs, err := q.groupsFn(ctx, stream)
	if err != nil {
		// Retained is valid, group progress is not: report unknown rather
		// than a fake zero (DATA-047).
		return b, nil
	}
	var g *redis.XInfoGroup
	for i := range gs {
		if gs[i].Name == q.group {
			g = &gs[i]
			break
		}
	}
	if g == nil {
		// The worker group does not exist yet: nothing has been delivered
		// (the group is created on first worker start, DATA-023), so every
		// retained entry is undelivered and none are pending ACK.
		undelivered := retained
		zero := int64(0)
		b.Undelivered = &undelivered
		b.PendingAck = &zero
		return b, nil
	}
	// go-redis reports lag as -1 when the broker cannot determine it
	// (Redis < 7 omits the field): keep it unknown instead of a fake zero.
	if g.Lag >= 0 {
		lag := g.Lag
		b.Undelivered = &lag
	}
	pending := g.Pending
	b.PendingAck = &pending
	return b, nil
}
