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
	rdb            *redis.Client
	ch             clickhouse.Conn
	streamEvents   string
	streamPayments string
	group          string
	consumer       string
	// aggregation state
	touchedMinutes map[string]struct{}
	touchedDays    map[string]struct{}
	revAgg         map[string]*revRow
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
	// naive DSN parse: clickhouse://host:port/db
	addr := strings.TrimPrefix(strings.TrimPrefix(dsn, "clickhouse://"), "http://")
	host := addr
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	ch, err := clickhouse.Open(&clickhouse.Options{Addr: []string{host}})
	if err != nil {
		return nil, fmt.Errorf("clickhouse: %w", err)
	}
	return &Worker{rdb: rdb, ch: ch, streamEvents: se, streamPayments: sp, group: grp, consumer: cons, touchedMinutes: map[string]struct{}{}, touchedDays: map[string]struct{}{}, revAgg: map[string]*revRow{}}, nil
}

func (w *Worker) ensureGroups(ctx context.Context) {
	_ = w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, "$").Err()
	_ = w.rdb.XGroupCreateMkStream(ctx, w.streamPayments, w.group, "$").Err()
}

func (w *Worker) Run(ctx context.Context) error {
	w.ensureGroups(ctx)
	// periodic flush
	go func() {
		tk := time.NewTicker(15 * time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
				if err := w.flush(ctx); err != nil {
					slog.Warn("flush", "err", err)
				}
			}
		}
	}()
	for {
		sel := []string{w.streamEvents, w.streamPayments}
		res, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{Group: w.group, Consumer: w.consumer, Streams: append([]string{}, append([]string{}, sel...)...), Count: 200, Block: 2 * time.Second}).Result()
		if err != nil && err != redis.Nil {
			slog.Warn("xreadgroup", "err", err)
			continue
		}
		for _, str := range res {
			for _, msg := range str.Messages {
				data := string(fmtAny(msg.Values["data"]))
				if data == "" {
					_ = w.rdb.XAck(ctx, str.Stream, w.group, msg.ID).Err()
					continue
				}
				var m map[string]any
				if err := json.Unmarshal([]byte(data), &m); err != nil {
					_ = w.rdb.XAck(ctx, str.Stream, w.group, msg.ID).Err()
					continue
				}
				if str.Stream == w.streamEvents {
					// Update Redis HLL for minute online, DAU/new_users
					w.touchAgg(ctx, m)
					if err := w.insertEvent(ctx, m); err != nil {
						slog.Warn("insert event", "err", err)
					}
				} else if str.Stream == w.streamPayments {
					w.touchRevenue(ctx, m)
					if err := w.insertPayment(ctx, m); err != nil {
						slog.Warn("insert payment", "err", err)
					}
				}
				_ = w.rdb.XAck(ctx, str.Stream, w.group, msg.ID).Err()
			}
		}
	}
}

func fmtAny(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(v)
	}
}

func asString(m map[string]any, k string) string {
	if v, ok := m[k]; ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}
func asFloat(m map[string]any, k string) float64 {
	if v, ok := m[k]; ok {
		switch t := v.(type) {
		case float64:
			return t
		case int:
			return float64(t)
		}
	}
	return 0
}

func (w *Worker) insertEvent(ctx context.Context, m map[string]any) error {
	// Minimal fields; props_json stays raw
	ts := asString(m, "ts")
	if ts == "" {
		ts = time.Now().Format(time.RFC3339)
	}
	game := asString(m, "game_id")
	env := asString(m, "env")
	uid := asString(m, "user_id")
	sid := asString(m, "session_id")
	evt := asString(m, "event")
	channel := asString(m, "channel")
	platform := asString(m, "platform")
	country := asString(m, "country")
	appv := asString(m, "app_version")
	eid := asString(m, "event_id")
	propsBytes, _ := json.Marshal(m["props"]) // may be nil
	batch, err := w.ch.PrepareBatch(ctx, "INSERT INTO analytics.events (event_time, game_id, env, user_id, session_id, event, channel, platform, country, app_version, event_id, props_json)")
	if err != nil {
		return err
	}
	if err := batch.Append(ts, game, env, uid, sid, evt, channel, platform, country, appv, eid, string(propsBytes)); err != nil {
		return err
	}
	return batch.Send()
}

func (w *Worker) insertPayment(ctx context.Context, m map[string]any) error {
	ts := asString(m, "ts")
	if ts == "" {
		ts = time.Now().Format(time.RFC3339)
	}
	game := asString(m, "game_id")
	env := asString(m, "env")
	uid := asString(m, "user_id")
	oid := asString(m, "order_id")
	amount := asFloat(m, "amount_cents")
	curr := asString(m, "currency")
	status := asString(m, "status")
	channel := asString(m, "channel")
	platform := asString(m, "platform")
	country := asString(m, "country")
	reason := asString(m, "reason")
	batch, err := w.ch.PrepareBatch(ctx, "INSERT INTO analytics.payments (time, game_id, env, user_id, order_id, amount_cents, currency, status, channel, platform, country, reason)")
	if err != nil {
		return err
	}
	if err := batch.Append(ts, game, env, uid, oid, uint64(amount), curr, status, channel, platform, country, reason); err != nil {
		return err
	}
	return batch.Send()
}

// --- Aggregation helpers ---

type revRow struct {
	revenue uint64
	refunds uint64
	failed  uint64
}

func (w *Worker) touchAgg(ctx context.Context, m map[string]any) {
	ts := asString(m, "ts")
	t, _ := time.Parse(time.RFC3339, ts)
	if ts == "" || t.IsZero() {
		t = time.Now()
	}
	game := asString(m, "game_id")
	env := asString(m, "env")
	uid := asString(m, "user_id")
	evt := strings.ToLower(asString(m, "event"))
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

func (w *Worker) touchRevenue(ctx context.Context, m map[string]any) {
	ts := asString(m, "ts")
	t, _ := time.Parse(time.RFC3339, ts)
	if ts == "" || t.IsZero() {
		t = time.Now()
	}
	day := t.Format("2006-01-02")
	game := asString(m, "game_id")
	env := asString(m, "env")
	status := strings.ToLower(asString(m, "status"))
	amt := uint64(asFloat(m, "amount_cents"))
	key := fmt.Sprintf("%s|%s|%s", day, game, env)
	rv := w.revAgg[key]
	if rv == nil {
		rv = &revRow{}
		w.revAgg[key] = rv
	}
	if status == "success" {
		rv.revenue += amt
	}
	if status == "refunded" {
		rv.refunds += amt
	}
	if status == "failed" {
		rv.failed += 1
	}
}

func (w *Worker) flush(ctx context.Context) error {
	nowMin := time.Now().Truncate(time.Minute)
	// flush minute_online for minutes earlier than current minute
	for k := range w.touchedMinutes {
		parts := strings.Split(k, ":") // hll:online:game:env:YYYYMMDDHHmm
		if len(parts) < 5 {
			delete(w.touchedMinutes, k)
			continue
		}
		ts := parts[len(parts)-1]
		t, err := time.Parse("200601021504", ts)
		if err != nil || !t.Before(nowMin) {
			continue
		}
		game := parts[2]
		env := parts[3]
		n, err := w.rdb.PFCount(ctx, k).Result()
		if err != nil {
			slog.Warn("pfcount", "key", k, "err", err)
			continue
		}
		if n < 0 {
			n = 0
		}
		batch, err := w.ch.PrepareBatch(ctx, "INSERT INTO analytics.minute_online (m, game_id, env, online)")
		if err != nil {
			slog.Warn("ch batch", "err", err)
			continue
		}
		if err := batch.Append(t, game, env, uint32(n)); err != nil {
			slog.Warn("batch append", "err", err)
			continue
		}
		if err := batch.Send(); err != nil {
			slog.Warn("batch send", "err", err)
			continue
		}
		delete(w.touchedMinutes, k)
	}
	// flush daily_users
	for dk := range w.touchedDays {
		sp := strings.Split(dk, "|")
		if len(sp) != 3 {
			delete(w.touchedDays, dk)
			continue
		}
		day, game, env := sp[0], sp[1], sp[2]
		kdau := fmt.Sprintf("hll:dau:%s:%s:%s", game, env, day)
		knew := fmt.Sprintf("hll:new:%s:%s:%s", game, env, day)
		dau, _ := w.rdb.PFCount(ctx, kdau).Result()
		neu, _ := w.rdb.PFCount(ctx, knew).Result()
		d, _ := time.Parse("2006-01-02", day)
		ver := uint64(time.Now().Unix())
		batch, err := w.ch.PrepareBatch(ctx, "INSERT INTO analytics.daily_users (d, game_id, env, dau, new_users, version)")
		if err != nil {
			slog.Warn("ch daily_users", "err", err)
			continue
		}
		if err := batch.Append(d, game, env, uint64(max0(dau)), uint64(max0(neu)), ver); err != nil {
			slog.Warn("daily_users append", "err", err)
			continue
		}
		if err := batch.Send(); err != nil {
			slog.Warn("daily_users send", "err", err)
			continue
		}
	}
	// flush daily_revenue
	for rk, rv := range w.revAgg {
		sp := strings.Split(rk, "|")
		if len(sp) != 3 {
			continue
		}
		day, game, env := sp[0], sp[1], sp[2]
		d, _ := time.Parse("2006-01-02", day)
		ver := uint64(time.Now().Unix())
		batch, err := w.ch.PrepareBatch(ctx, "INSERT INTO analytics.daily_revenue (d, game_id, env, revenue_cents, refunds_cents, failed, version)")
		if err != nil {
			slog.Warn("ch daily_revenue", "err", err)
			continue
		}
		if err := batch.Append(d, game, env, rv.revenue, rv.refunds, rv.failed, ver); err != nil {
			slog.Warn("daily_revenue append", "err", err)
			continue
		}
		if err := batch.Send(); err != nil {
			slog.Warn("daily_revenue send", "err", err)
			continue
		}
	}
	return nil
}

func max0(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}
