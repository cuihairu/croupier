package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type redisQueue struct {
	cli            *redis.Client
	streamEvents   string
	streamPayments string
	maxLenApprox   bool
	maxLen         int64
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
	args := &redis.XAddArgs{Stream: stream, Values: map[string]any{"data": string(b)}}
	if q.maxLen > 0 {
		args.MaxLen = q.maxLen
		args.Approx = q.maxLenApprox
	}
	return q.cli.XAdd(ctx, args).Err()
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
