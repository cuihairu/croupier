package mq

import (
    "context"
    "encoding/json"
    "log"
    "os"
    "strconv"
    "time"

    redis "github.com/redis/go-redis/v9"
)

type redisQueue struct {
    cli *redis.Client
    streamEvents   string
    streamPayments string
    maxLenApprox   bool
    maxLen         int64
}

func NewRedis(url, streamEvents, streamPayments string, maxLen int64, approx bool) Queue {
    opt, err := redis.ParseURL(url)
    if err != nil { log.Printf("[analytics-mq] redis parse url: %v", err); return NewNoop() }
    cli := redis.NewClient(opt)
    return &redisQueue{cli: cli, streamEvents: streamEvents, streamPayments: streamPayments, maxLen: maxLen, maxLenApprox: approx}
}

func newRedisFromEnv() (Queue, error) {
    url := os.Getenv("REDIS_URL")
    if url == "" { url = "redis://localhost:6379/0" }
    se := os.Getenv("ANALYTICS_REDIS_STREAM_EVENTS"); if se == "" { se = "analytics:events" }
    sp := os.Getenv("ANALYTICS_REDIS_STREAM_PAYMENTS"); if sp == "" { sp = "analytics:payments" }
    ml := int64(1000000)
    if v := os.Getenv("ANALYTICS_REDIS_MAXLEN"); v != "" { if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 { ml = n } }
    approx := true
    if v := os.Getenv("ANALYTICS_REDIS_MAXLEN_APPROX"); v != "" { approx = (v == "1" || v == "true" || v == "yes") }
    return NewRedis(url, se, sp, ml, approx), nil
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
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second); defer cancel()
    return q.xadd(ctx, q.streamEvents, evt)
}

func (q *redisQueue) PublishPayment(pay map[string]any) error {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second); defer cancel()
    return q.xadd(ctx, q.streamPayments, pay)
}
