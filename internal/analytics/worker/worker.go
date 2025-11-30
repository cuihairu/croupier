package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
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
	// batching
	eventBatch       chdriver.Batch
	eventBatchRows   int
	paymentBatch     chdriver.Batch
	paymentBatchRows int
	clickBatchSize   int
	// checkpoints
	checkpointPrefix string
	lastIDs          map[string]string
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
	if len(opts.Addr) == 0 {
		opts.Addr = []string{"localhost:9000"}
	}
	ch, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: %w", err)
	}
	batchSize := 500
	if v := strings.TrimSpace(os.Getenv("ANALYTICS_CLICKHOUSE_BATCH")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			batchSize = n
		}
	}
	return &Worker{
		rdb:              rdb,
		ch:               ch,
		streamEvents:     se,
		streamPayments:   sp,
		group:            grp,
		consumer:         cons,
		touchedMinutes:   map[string]struct{}{},
		touchedDays:      map[string]struct{}{},
		revAgg:           map[string]*revRow{},
		clickBatchSize:   batchSize,
		checkpointPrefix: strings.TrimSuffix(envOrDefault("ANALYTICS_CHECKPOINT_PREFIX", "analytics:checkpoint"), ":"),
		lastIDs:          map[string]string{},
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
	defer func() {
		_ = w.flushBatches(context.Background())
	}()
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
				w.lastIDs[str.Stream] = msg.ID
				w.persistCheckpoint(ctx, str.Stream, msg.ID)
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
	return w.appendEventRow(ctx, ts, game, env, uid, sid, evt, channel, platform, country, appv, eid, string(propsBytes))
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
	region := asString(m, "region")
	city := asString(m, "city")
	product := asString(m, "product_id")
	reason := asString(m, "reason")
	return w.appendPaymentRow(ctx, ts, game, env, uid, oid, uint64(amount), curr, status, channel, platform, country, region, city, product, reason)
}

func (w *Worker) ensureEventBatch(ctx context.Context) (chdriver.Batch, error) {
	if w.eventBatch != nil {
		return w.eventBatch, nil
	}
	batch, err := w.ch.PrepareBatch(ctx, insertEventsSQL)
	if err != nil {
		return nil, err
	}
	w.eventBatch = batch
	w.eventBatchRows = 0
	return batch, nil
}

func (w *Worker) ensurePaymentBatch(ctx context.Context) (chdriver.Batch, error) {
	if w.paymentBatch != nil {
		return w.paymentBatch, nil
	}
	batch, err := w.ch.PrepareBatch(ctx, insertPaymentsSQL)
	if err != nil {
		return nil, err
	}
	w.paymentBatch = batch
	w.paymentBatchRows = 0
	return batch, nil
}

func (w *Worker) appendEventRow(ctx context.Context, args ...any) error {
	batch, err := w.ensureEventBatch(ctx)
	if err != nil {
		return err
	}
	if err := batch.Append(args...); err != nil {
		return err
	}
	w.eventBatchRows++
	if w.clickBatchSize > 0 && w.eventBatchRows >= w.clickBatchSize {
		return w.flushEventBatch(ctx)
	}
	return nil
}

func (w *Worker) appendPaymentRow(ctx context.Context, args ...any) error {
	batch, err := w.ensurePaymentBatch(ctx)
	if err != nil {
		return err
	}
	if err := batch.Append(args...); err != nil {
		return err
	}
	w.paymentBatchRows++
	if w.clickBatchSize > 0 && w.paymentBatchRows >= w.clickBatchSize {
		return w.flushPaymentBatch(ctx)
	}
	return nil
}

func (w *Worker) flushEventBatch(ctx context.Context) error {
	if w.eventBatch == nil || w.eventBatchRows == 0 {
		return nil
	}
	if err := w.eventBatch.Send(); err != nil {
		return err
	}
	w.eventBatch = nil
	w.eventBatchRows = 0
	return nil
}

func (w *Worker) flushPaymentBatch(ctx context.Context) error {
	if w.paymentBatch == nil || w.paymentBatchRows == 0 {
		return nil
	}
	if err := w.paymentBatch.Send(); err != nil {
		return err
	}
	w.paymentBatch = nil
	w.paymentBatchRows = 0
	return nil
}

func (w *Worker) flushBatches(ctx context.Context) error {
	if err := w.flushEventBatch(ctx); err != nil {
		return err
	}
	if err := w.flushPaymentBatch(ctx); err != nil {
		return err
	}
	return nil
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
	if err := w.flushBatches(ctx); err != nil {
		slog.Warn("flush batches", "err", err)
	}
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
		delete(w.touchedDays, dk)
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
		delete(w.revAgg, rk)
	}
	return nil
}

func max0(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}
func (w *Worker) restoreCheckpoints(ctx context.Context, stream string) {
	if w.checkpointPrefix == "" {
		return
	}
	key := fmt.Sprintf("%s:%s", w.checkpointPrefix, stream)
	id, err := w.rdb.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			slog.Warn("checkpoint get", "stream", stream, "err", err)
		}
		return
	}
	if id == "" {
		return
	}
	if err := w.rdb.XGroupSetID(ctx, stream, w.group, id).Err(); err != nil {
		slog.Warn("checkpoint setid", "stream", stream, "err", err)
		return
	}
	w.lastIDs[stream] = id
}

func (w *Worker) persistCheckpoint(ctx context.Context, stream, id string) {
	if w.checkpointPrefix == "" || id == "" {
		return
	}
	key := fmt.Sprintf("%s:%s", w.checkpointPrefix, stream)
	if err := w.rdb.Set(ctx, key, id, 0).Err(); err != nil {
		slog.Warn("checkpoint set", "stream", stream, "err", err)
	}
}
