package worker

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "os"
    "strings"
    "time"

    clickhouse "github.com/ClickHouse/clickhouse-go/v2"
    redis "github.com/redis/go-redis/v9"
)

type Worker struct {
    rdb *redis.Client
    ch  clickhouse.Conn
    streamEvents   string
    streamPayments string
    group          string
    consumer       string
}

func NewWorker() (*Worker, error) {
    // Redis
    rurl := os.Getenv("REDIS_URL")
    if rurl == "" { rurl = "redis://localhost:6379/0" }
    ropt, err := redis.ParseURL(rurl)
    if err != nil { return nil, fmt.Errorf("redis: %w", err) }
    rdb := redis.NewClient(ropt)
    se := os.Getenv("ANALYTICS_REDIS_STREAM_EVENTS"); if se == "" { se = "analytics:events" }
    sp := os.Getenv("ANALYTICS_REDIS_STREAM_PAYMENTS"); if sp == "" { sp = "analytics:payments" }
    grp := os.Getenv("WORKER_GROUP"); if grp == "" { grp = "analytics-worker" }
    cons := os.Getenv("WORKER_CONSUMER"); if cons == "" { cons = fmt.Sprintf("c-%d", time.Now().UnixNano()) }
    // ClickHouse
    dsn := os.Getenv("CLICKHOUSE_DSN"); if dsn == "" { dsn = "clickhouse://localhost:9000/analytics" }
    ch, err := clickhouse.Open(&clickhouse.Options{ Addr: []string{ strings.TrimPrefix(strings.TrimPrefix(dsn, "clickhouse://"), "http://") } })
    if err != nil { return nil, fmt.Errorf("clickhouse: %w", err) }
    return &Worker{ rdb: rdb, ch: ch, streamEvents: se, streamPayments: sp, group: grp, consumer: cons }, nil
}

func (w *Worker) ensureGroups(ctx context.Context) {
    _ = w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$" ).Err()
    _ = w.rdb.XGroupCreateMkStream(ctx, w.streamPayments, w.group, "$" ).Err()
}

func (w *Worker) Run(ctx context.Context) error {
    w.ensureGroups(ctx)
    for {
        sel := []string{ w.streamEvents, w.streamPayments }
        res, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{ Group: w.group, Consumer: w.consumer, Streams: append([]string{}, append([]string{}, sel...)...), Count: 200, Block: 2*time.Second }).Result()
        if err != nil && err != redis.Nil { slog.Warn("xreadgroup", "err", err); continue }
        for _, str := range res {
            for _, msg := range str.Messages {
                data := string(fmtAny(msg.Values["data"]))
                if data == "" { _ = w.rdb.XAck(ctx, str.Stream, w.group, msg.ID).Err(); continue }
                var m map[string]any
                if err := json.Unmarshal([]byte(data), &m); err != nil { _ = w.rdb.XAck(ctx, str.Stream, w.group, msg.ID).Err(); continue }
                if str.Stream == w.streamEvents {
                    if err := w.insertEvent(ctx, m); err != nil { slog.Warn("insert event", "err", err) }
                } else if str.Stream == w.streamPayments {
                    if err := w.insertPayment(ctx, m); err != nil { slog.Warn("insert payment", "err", err) }
                }
                _ = w.rdb.XAck(ctx, str.Stream, w.group, msg.ID).Err()
            }
        }
    }
}

func fmtAny(v any) string { if v==nil { return "" }; switch t:=v.(type){ case string: return t; case []byte: return string(t); default: return fmt.Sprint(v) } }

func asString(m map[string]any, k string) string { if v, ok := m[k]; ok { if s,ok2 := v.(string); ok2 { return s } }; return "" }
func asFloat(m map[string]any, k string) float64 { if v, ok := m[k]; ok { switch t:=v.(type){ case float64: return t; case int: return float64(t) } }; return 0 }

func (w *Worker) insertEvent(ctx context.Context, m map[string]any) error {
    // Minimal fields; props_json stays raw
    ts := asString(m, "ts")
    if ts == "" { ts = time.Now().Format(time.RFC3339) }
    game := asString(m, "game_id"); env := asString(m, "env")
    uid := asString(m, "user_id"); sid := asString(m, "session_id"); evt := asString(m, "event")
    channel := asString(m, "channel"); platform := asString(m, "platform"); country := asString(m, "country"); appv := asString(m, "app_version")
    eid := asString(m, "event_id")
    propsBytes, _ := json.Marshal(m["props"]) // may be nil
    batch, err := w.ch.PrepareBatch(ctx, "INSERT INTO analytics.events (event_time, game_id, env, user_id, session_id, event, channel, platform, country, app_version, event_id, props_json)")
    if err != nil { return err }
    if err := batch.Append(ts, game, env, uid, sid, evt, channel, platform, country, appv, eid, string(propsBytes)); err != nil { return err }
    return batch.Send()
}

func (w *Worker) insertPayment(ctx context.Context, m map[string]any) error {
    ts := asString(m, "ts"); if ts=="" { ts = time.Now().Format(time.RFC3339) }
    game := asString(m, "game_id"); env := asString(m, "env")
    uid := asString(m, "user_id"); oid := asString(m, "order_id")
    amount := asFloat(m, "amount_cents")
    curr := asString(m, "currency"); status := asString(m, "status"); channel := asString(m, "channel"); platform := asString(m, "platform"); country := asString(m, "country"); reason := asString(m, "reason")
    batch, err := w.ch.PrepareBatch(ctx, "INSERT INTO analytics.payments (time, game_id, env, user_id, order_id, amount_cents, currency, status, channel, platform, country, reason)")
    if err != nil { return err }
    if err := batch.Append(ts, game, env, uid, oid, uint64(amount), curr, status, channel, platform, country, reason); err != nil { return err }
    return batch.Send()
}

