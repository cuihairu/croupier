package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
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

// shutdownTimeout / finalFlushTimeout 是 Run 退出序列的有界等待上限（默认
// 与生产行为一致），作为可注入缝隙供测试缩短以覆盖阻塞刷新与最终 flush
// 的截止分支（DATA-015）。
var (
	shutdownTimeout   = 5 * time.Second
	finalFlushTimeout = 10 * time.Second
)

// pendingReclaimInterval 是回收循环的心跳间隔（默认与生产行为一致），
// 作为可注入缝隙供测试缩短以覆盖 ticker 分支。
var pendingReclaimInterval = 30 * time.Second

// runFlushInterval 是 Run 周期 flush 的心跳间隔（默认与生产行为一致），
// 作为可注入缝隙供测试缩短以覆盖 ticker 分支。
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
	// ParseDSN 恒产出非空 Addr（DSN 缺 host 时直接报 "parse dsn address
	// failed"），空 Addr 兜底分支不可达，已删除。
	// clickhouse-go v2.43.0 的 Open 无条件构造连接对象返回 nil error
	// （懒连接，不做任何可失败的校验），error 分支不可达，已删除。
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
	// workerCtx 派生自 WithoutCancel(ctx)：Run 返回后子协程仍可安全地完成
	// 在途刷新/回收，随后由下面的等待顺序收束；父 ctx 取消只负责让主循环
	// 停止读取新消息，不直接杀死仍在刷新 ClickHouse 的子协程。
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
				// flush 实现恒返回 nil（内部各失败路径均以 slog.Warn
				// 记录后继续），err 分支不可达。
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

	// 有界退出顺序（DATA-015）：先停子协程（stopWorkers），再以
	// shutdownTimeout 上界等它们完成在途刷新/回收，最后以带截止的独立
	// flush 把剩余 pending 批次与聚合状态落盘。不再读 worker/测试配置
	// （runFlushInterval 已固定为当前值），且不在刷新未结束时关资源。
	stopWorkers()
	if !waitGroupBounded(&flushWG, shutdownTimeout) {
		slog.Warn("flush goroutine exceeded shutdown timeout", "timeout", shutdownTimeout)
	}
	if !waitGroupBounded(&reclaimWG, shutdownTimeout) {
		slog.Warn("reclaim goroutine exceeded shutdown timeout", "timeout", shutdownTimeout)
	}

	flushCtx, cancelFlush := context.WithTimeout(context.WithoutCancel(ctx), finalFlushTimeout)
	defer cancelFlush()
	flushDone := make(chan struct{})
	go func() {
		defer close(flushDone)
		w.flush(flushCtx)
	}()
	select {
	case <-flushDone:
	case <-flushCtx.Done():
		slog.Warn("final flush exceeded deadline", "timeout", finalFlushTimeout)
	}
	return ctx.Err()
}

// waitGroupBounded waits up to timeout for wg to finish. It returns true if
// the group completed, false on timeout (the waiter goroutine is abandoned;
// this is the bounded-exit cost when a refresh is genuinely stuck).
func waitGroupBounded(wg *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
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

// normalizeTimestamp coerces common wire formats into the ClickHouse
// datetime layout "2006-01-02 15:04:05" expected by appendEventRow:
// RFC3339 (bridge/E2E producers), unix seconds, and the layout itself.
func normalizeTimestamp(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Now().Format("2006-01-02 15:04:05")
	}
	if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return t.UTC().Format("2006-01-02 15:04:05")
	}
	if _, err := time.Parse("2006-01-02 15:04:05", trimmed); err == nil {
		return trimmed
	}
	if secs, err := strconv.ParseInt(trimmed, 10, 64); err == nil && secs > 0 {
		if secs > 1e12 { // 13+ digits: milliseconds (quick-start producers)
			secs /= 1000
		}
		return time.Unix(secs, 0).UTC().Format("2006-01-02 15:04:05")
	}
	// Unrecognized: fall through with the raw value; ClickHouse Append will
	// surface a parse error (and the message retries into the dead letter).
	return trimmed
}

func (w *Worker) insertEvent(ctx context.Context, m map[string]any, msgID string) error {
	// Minimal fields; props_json stays raw
	ts := normalizeTimestamp(asString(m, "ts"))
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
	if _, err := uuid.Parse(eid); err != nil {
		// ClickHouse UUID column rejects empty/invalid values; producers that
		// do not assign event ids still get a dedup-key-grade identifier.
		eid = uuid.NewString()
	}
	propsBytes, _ := json.Marshal(m["props"]) // may be nil
	return w.appendEventRow(ctx, msgID, ts, game, env, uid, sid, evt, channel, platform, country, appv, eid, string(propsBytes))
}

func (w *Worker) insertPayment(ctx context.Context, m map[string]any, msgID string) error {
	ts := normalizeTimestamp(asString(m, "ts"))
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
	return w.appendPaymentRow(ctx, msgID, ts, game, env, uid, oid, uint64(amount), curr, status, channel, platform, country, region, city, product, reason)
}

func (w *Worker) ensureEventBatch(ctx context.Context) (chdriver.Batch, error) {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	return w.ensureEventBatchLocked(ctx)
}

func (w *Worker) ensureEventBatchLocked(ctx context.Context) (chdriver.Batch, error) {
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
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	return w.ensurePaymentBatchLocked(ctx)
}

func (w *Worker) ensurePaymentBatchLocked(ctx context.Context) (chdriver.Batch, error) {
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

// appendEventRow appends one row to the event batch and records the source
// message in eventAcks so it is ACKed only when the batch durably Send()s
// (ARCH-011). The msgID is the Redis stream message the row came from.
func (w *Worker) appendEventRow(ctx context.Context, msgID string, args ...any) error {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	batch, err := w.ensureEventBatchLocked(ctx)
	if err != nil {
		return err
	}
	if err := batch.Append(args...); err != nil {
		return err
	}
	w.eventBatchRows++
	w.eventAcks = append(w.eventAcks, ackRef{stream: w.streamEvents, id: msgID})
	if w.clickBatchSize > 0 && w.eventBatchRows >= w.clickBatchSize {
		return w.flushEventBatchLocked(ctx)
	}
	return nil
}

// appendPaymentRow appends one row to the payment batch and records the
// source message in paymentAcks so it is ACKed only when the batch durably
// Send()s (ARCH-011).
func (w *Worker) appendPaymentRow(ctx context.Context, msgID string, args ...any) error {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	batch, err := w.ensurePaymentBatchLocked(ctx)
	if err != nil {
		return err
	}
	if err := batch.Append(args...); err != nil {
		return err
	}
	w.paymentBatchRows++
	ref := ackRef{stream: w.streamPayments, id: msgID}
	if key, delta, ok := revenueDeltaFromPaymentArgs(args); ok {
		ref.revKey = key
		ref.rev = delta
	}
	w.paymentAcks = append(w.paymentAcks, ref)
	if w.clickBatchSize > 0 && w.paymentBatchRows >= w.clickBatchSize {
		return w.flushPaymentBatchLocked(ctx)
	}
	return nil
}

func (w *Worker) flushEventBatch(ctx context.Context) error {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	return w.flushEventBatchLocked(ctx)
}

// flushEventBatchLocked Send()s the event batch, then ACKs its deferred
// message refs. On Send failure the batch and its refs are DROPPED (not
// retried): the messages stay PENDING in Redis so reclaim redelivers them,
// and the triggering message takes the processMessage retry path — avoiding
// double-writing a row that was appended to a batch whose Send failed.
func (w *Worker) flushEventBatchLocked(ctx context.Context) error {
	if w.eventBatch == nil || w.eventBatchRows == 0 {
		w.eventAcks = nil
		return nil
	}
	if err := w.eventBatch.Send(); err != nil {
		w.eventBatch = nil
		w.eventBatchRows = 0
		w.eventAcks = nil
		return err
	}
	w.eventBatch = nil
	w.eventBatchRows = 0
	acks := w.eventAcks
	w.eventAcks = nil
	w.ackSent(ctx, acks)
	return nil
}

func (w *Worker) flushPaymentBatch(ctx context.Context) error {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	return w.flushPaymentBatchLocked(ctx)
}

func (w *Worker) flushPaymentBatchLocked(ctx context.Context) error {
	if w.paymentBatch == nil || w.paymentBatchRows == 0 {
		w.paymentAcks = nil
		return nil
	}
	if err := w.paymentBatch.Send(); err != nil {
		w.paymentBatch = nil
		w.paymentBatchRows = 0
		w.paymentAcks = nil
		return err
	}
	w.paymentBatch = nil
	w.paymentBatchRows = 0
	acks := w.paymentAcks
	w.paymentAcks = nil
	w.commitRevenue(acks)
	w.ackSent(ctx, acks)
	return nil
}

func revenueDeltaFromPaymentArgs(args []any) (string, *revRow, bool) {
	if len(args) < 9 {
		return "", nil, false
	}
	ts, ok := args[0].(string)
	if !ok || len(ts) < 10 {
		return "", nil, false
	}
	game, ok := args[1].(string)
	if !ok {
		return "", nil, false
	}
	env, ok := args[2].(string)
	if !ok {
		return "", nil, false
	}
	amount, ok := args[5].(uint64)
	if !ok {
		return "", nil, false
	}
	status, ok := args[7].(string)
	if !ok {
		return "", nil, false
	}
	delta := &revRow{}
	switch strings.ToLower(status) {
	case "success":
		delta.revenue = amount
	case "refunded":
		delta.refunds = amount
	case "failed":
		delta.failed = 1
	default:
		return "", nil, false
	}
	return fmt.Sprintf("%s|%s|%s", ts[:10], game, env), delta, true
}

func (w *Worker) commitRevenue(acks []ackRef) {
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	for _, ack := range acks {
		if ack.rev == nil {
			continue
		}
		current := w.revAgg[ack.revKey]
		if current == nil {
			current = &revRow{}
			w.revAgg[ack.revKey] = current
		}
		current.revenue += ack.rev.revenue
		current.refunds += ack.rev.refunds
		current.failed += ack.rev.failed
	}
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

// ackSent ACKs message refs whose rows have been durably Send()ed to
// ClickHouse, then advances the per-stream checkpoint. A XAck failure is
// logged but not fatal: the message stays pending and reclaim redelivers it,
// and the ClickHouse natural key (event_id / order_id) deduplicates the
// re-insert.
func (w *Worker) ackSent(ctx context.Context, acks []ackRef) {
	for _, a := range acks {
		if err := w.rdb.XAck(ctx, a.stream, w.group, a.id).Err(); err != nil {
			slog.Warn("batch ack failed", "stream", a.stream, "id", a.id, "err", err)
			continue
		}
		w.advanceCheckpoint(ctx, a.stream, a.id)
	}
}

// ackImmediate ACKs a message resolved without a pending batch row (empty
// payload, dead-lettered, or re-queued) and advances the checkpoint.
func (w *Worker) ackImmediate(ctx context.Context, stream string, msg redis.XMessage) error {
	if err := w.rdb.XAck(ctx, stream, w.group, msg.ID).Err(); err != nil {
		slog.Warn("ack failed", "stream", stream, "id", msg.ID, "err", err)
		return err
	}
	w.advanceCheckpoint(ctx, stream, msg.ID)
	return nil
}

// advanceCheckpoint moves the per-stream high-water mark forward. Stream IDs
// are "<ms>-<seq>"; a later message may be ACKed immediately (dead-letter /
// re-queue) while an earlier one still sits in an unsent batch, so the mark
// must be monotonic rather than blindly overwritten — otherwise a checkpoint
// regression would re-read already-resolved messages after a restart.
func (w *Worker) advanceCheckpoint(ctx context.Context, stream, id string) {
	w.lastMu.Lock()
	cur := w.lastIDs[stream]
	if streamIDLess(id, cur) {
		w.lastMu.Unlock()
		return
	}
	w.lastIDs[stream] = id
	w.lastMu.Unlock()
	w.persistCheckpoint(ctx, stream, id)
}

// parseStreamID splits a "<ms>-<seq>" stream ID; malformed IDs parse as 0.
func parseStreamID(s string) (ms, seq uint64) {
	i := strings.IndexByte(s, '-')
	if i < 0 {
		ms, _ = strconv.ParseUint(s, 10, 64)
		return
	}
	ms, _ = strconv.ParseUint(s[:i], 10, 64)
	seq, _ = strconv.ParseUint(s[i+1:], 10, 64)
	return
}

// streamIDLess reports whether stream ID a sorts before b.
func streamIDLess(a, b string) bool {
	am, as := parseStreamID(a)
	bm, bs := parseStreamID(b)
	if am != bm {
		return am < bm
	}
	return as < bs
}

// --- Aggregation helpers ---

type revRow struct {
	revenue uint64
	refunds uint64
	failed  uint64
}

// otelEventAliases maps the dotted OTel semantic event names emitted by the
// telemetry bridge (session.start, user.login, ...) onto the underscore
// names the aggregation matcher understands. Bridge-side naming follows
// OTel conventions; the worker stays the contract owner for aggregation.
var otelEventAliases = map[string]string{
	"session.start": "session_start",
	"session.end":   "session_end",
	"user.login":    "login",
	"user.register": "register",
}

func normalizeAggEvent(raw string) string {
	evt := strings.ToLower(strings.TrimSpace(raw))
	if canonical, ok := otelEventAliases[evt]; ok {
		return canonical
	}
	return evt
}

func (w *Worker) touchAgg(ctx context.Context, m map[string]any) {
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	ts := asString(m, "ts")
	t, _ := time.Parse(time.RFC3339, ts)
	if ts == "" || t.IsZero() {
		t = time.Now()
	}
	// 分桶一律归一 UTC：flush 侧 time.Parse 按 UTC 解析 hll key 里的墙钟串，
	// 这里若按事件自带 offset（或本地时区 fallback）渲染，非 UTC 部署上双方
	// 错开一个时区偏移，minute_online 在 CST 服务器上永不满足 t.Before(nowMin)
	// 而漏flush（993cb65 审计时间窗同族：写入/解析时区必须一致）。
	t = t.UTC()
	game := asString(m, "game_id")
	env := asString(m, "env")
	uid := asString(m, "user_id")
	evt := normalizeAggEvent(asString(m, "event"))
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

func (w *Worker) flush(ctx context.Context) error {
	if err := w.flushBatches(ctx); err != nil {
		slog.Warn("flush batches", "err", err)
	}
	// 快照/同步所有权（DATA-014）：在 aggMu 内把共享状态整段转移给本地
	// 快照并清空共享 map，随即释放锁，再对快照做 ClickHouse I/O。写方
	// （touchAgg/commitRevenue）不再被长持锁阻塞；I/O 失败时把该项合并
	// 回共享 map（若写方已重建同 key，则累加而非覆盖），失败重试不丢
	// 增量。快照转移后写方新值落在共享 map，不会被本次 flush 误删。
	w.aggMu.Lock()
	minSnap := make(map[string]struct{}, len(w.touchedMinutes))
	for k := range w.touchedMinutes {
		minSnap[k] = struct{}{}
	}
	w.touchedMinutes = map[string]struct{}{}
	daySnap := make(map[string]struct{}, len(w.touchedDays))
	for k := range w.touchedDays {
		daySnap[k] = struct{}{}
	}
	w.touchedDays = map[string]struct{}{}
	revSnap := make(map[string]*revRow, len(w.revAgg))
	for k, rv := range w.revAgg {
		revSnap[k] = rv
	}
	w.revAgg = map[string]*revRow{}
	w.aggMu.Unlock()

	// key 内墙钟串已归一 UTC（touchAgg），比较基准同步 UTC。
	nowMin := time.Now().UTC().Truncate(time.Minute)
	// flush minute_online for minutes earlier than current minute
	for k := range minSnap {
		parts := strings.Split(k, ":") // hll:online:game:env:YYYYMMDDHHmm
		if len(parts) < 5 {
			// 无法解析的 key 属垃圾数据：丢弃（共享 map 已清空，不重写回），
			// 避免每次 flush 都失败重试。仅 I/O 失败才 remerge 保留。
			continue
		}
		ts := parts[len(parts)-1]
		t, err := time.Parse("200601021504", ts)
		if err != nil {
			// 坏时间串：不属可 flush 范围，丢弃（原语义 delete）。
			continue
		}
		if !t.Before(nowMin) {
			// 当前/未来分钟：保留，等它成为过去后再 flush（原语义）。
			w.remergeTouchedMinute(k)
			continue
		}
		game := parts[2]
		env := parts[3]
		n, err := w.rdb.PFCount(ctx, k).Result()
		if err != nil {
			slog.Warn("pfcount", "key", k, "err", err)
			w.remergeTouchedMinute(k)
			continue
		}
		if n < 0 {
			n = 0
		}
		batch, err := w.ch.PrepareBatch(ctx, "INSERT INTO analytics.minute_online (m, game_id, env, online)")
		if err != nil {
			slog.Warn("ch batch", "err", err)
			w.remergeTouchedMinute(k)
			continue
		}
		if err := batch.Append(t, game, env, uint32(n)); err != nil {
			slog.Warn("batch append", "err", err)
			w.remergeTouchedMinute(k)
			continue
		}
		if err := batch.Send(); err != nil {
			slog.Warn("batch send", "err", err)
			w.remergeTouchedMinute(k)
			continue
		}
	}
	// flush daily_users
	for dk := range daySnap {
		sp := strings.Split(dk, "|")
		if len(sp) != 3 {
			// 坏 key 丢弃（原语义）。
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
			w.remergeTouchedDay(dk)
			continue
		}
		if err := batch.Append(d, game, env, uint64(max0(dau)), uint64(max0(neu)), ver); err != nil {
			slog.Warn("daily_users append", "err", err)
			w.remergeTouchedDay(dk)
			continue
		}
		if err := batch.Send(); err != nil {
			slog.Warn("daily_users send", "err", err)
			w.remergeTouchedDay(dk)
			continue
		}
	}
	// flush daily_revenue
	for rk, rv := range revSnap {
		sp := strings.Split(rk, "|")
		if len(sp) != 3 {
			// 坏 key 保留（原语义：continue 不 delete，重试可修复）。
			w.remergeRev(rk, rv)
			continue
		}
		day, game, env := sp[0], sp[1], sp[2]
		d, _ := time.Parse("2006-01-02", day)
		ver := uint64(time.Now().Unix())
		batch, err := w.ch.PrepareBatch(ctx, "INSERT INTO analytics.daily_revenue (d, game_id, env, revenue_cents, refunds_cents, failed, version)")
		if err != nil {
			slog.Warn("ch daily_revenue", "err", err)
			w.remergeRev(rk, rv)
			continue
		}
		if err := batch.Append(d, game, env, rv.revenue, rv.refunds, rv.failed, ver); err != nil {
			slog.Warn("daily_revenue append", "err", err)
			w.remergeRev(rk, rv)
			continue
		}
		if err := batch.Send(); err != nil {
			slog.Warn("daily_revenue send", "err", err)
			w.remergeRev(rk, rv)
			continue
		}
	}
	return nil
}

// remergeTouchedMinute 把 flush 失败的 minute_online key 合并回共享 map；
// 若写方已重建同 key，保留（新值由写方继续累积，快照项为旧值，不覆盖）。
func (w *Worker) remergeTouchedMinute(k string) {
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	if _, ok := w.touchedMinutes[k]; ok {
		return
	}
	w.touchedMinutes[k] = struct{}{}
}

// remergeTouchedDay 把 flush 失败的 daily_users key 合并回共享 map（语义同
// remergeTouchedMinute）。
func (w *Worker) remergeTouchedDay(k string) {
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	if _, ok := w.touchedDays[k]; ok {
		return
	}
	w.touchedDays[k] = struct{}{}
}

// remergeRev 把 flush 失败的 daily_revenue 增量合并回共享 map；若写方已
// 重建同 key，则累加（不覆盖），保证失败重试不丢增量。
func (w *Worker) remergeRev(k string, rv *revRow) {
	w.aggMu.Lock()
	defer w.aggMu.Unlock()
	cur := w.revAgg[k]
	if cur == nil {
		w.revAgg[k] = rv
		return
	}
	cur.revenue += rv.revenue
	cur.refunds += rv.refunds
	cur.failed += rv.failed
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
	w.lastMu.Lock()
	w.lastIDs[stream] = id
	w.lastMu.Unlock()
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

// processMessage handles a single message with error handling and dead letter
// queue.
//
// Durability contract (ARCH-011): a message whose row is appended to an
// in-memory ClickHouse batch is NOT ACKed here — XACK is deferred until the
// batch durably Send()s (see flushEventBatch/flushPaymentBatch). A crash
// between append and Send therefore redelivers the message via the PEL
// reclaim path instead of silently dropping the row. Duplicates can occur if
// a crash lands between Send and XACK; the ClickHouse natural key
// (event_id for events, order_id for payments) is the dedup point, and the
// raw tables are plain MergeTree so consumers must treat those keys as
// unique. A message that is re-queued or dead-lettered is ACKed only after
// the re-queue / dead-letter XAdd succeeds — otherwise it stays pending.
func (w *Worker) processMessage(ctx context.Context, stream string, msg redis.XMessage) error {
	data := string(fmtAny(msg.Values["data"]))
	if data == "" {
		// Empty data, just acknowledge
		return w.ackImmediate(ctx, stream, msg)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		// Invalid JSON, move to dead letter
		if derr := w.sendToDeadLetter(ctx, stream, msg, "invalid_json", err.Error()); derr != nil {
			return derr // do not ack: reclaim retries the dead-letter
		}
		return w.ackImmediate(ctx, stream, msg)
	}

	// Add retry count if not present
	if m["retry_count"] == nil {
		m["retry_count"] = float64(0) // JSON numbers are float64
	}

	var err error
	switch stream {
	case w.streamEvents:
		// Update Redis HLL for minute online, DAU/new_users
		w.touchAgg(ctx, m)
		err = w.insertEvent(ctx, m, msg.ID)
	case w.streamPayments:
		err = w.insertPayment(ctx, m, msg.ID)
	}

	if err != nil {
		retries := int(m["retry_count"].(float64))
		if retries >= maxPendingRetries {
			// Max retries exceeded, move to dead letter
			if derr := w.sendToDeadLetter(ctx, stream, msg, "max_retries_exceeded", err.Error()); derr != nil {
				return derr // do not ack: reclaim retries the dead-letter
			}
			return w.ackImmediate(ctx, stream, msg)
		}
		// Increment retry count and re-queue
		m["retry_count"] = retries + 1
		retryData, _ := json.Marshal(m)
		if rerr := w.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: stream,
			MaxLen: 1000, // Keep retry queue bounded
			Approx: true,
			ID:     "*",
			Values: map[string]interface{}{"data": string(retryData)},
		}).Err(); rerr != nil {
			// Re-queue failed: do NOT ack. The message stays pending so the
			// reclaim path retries it; acking here would lose it.
			slog.Warn("message requeue failed; leaving pending for reclaim", "stream", stream, "id", msg.ID, "retries", retries+1, "err", rerr)
			return rerr
		}
		slog.Warn("message processing failed, re-queued", "stream", stream, "id", msg.ID, "retries", retries+1, "err", err)
		return w.ackImmediate(ctx, stream, msg)
	}

	// Success: the row is in an unsent ClickHouse batch. XACK is deferred
	// until the batch Send()s; do not ack or advance the checkpoint here.
	return nil
}

// sendToDeadLetter sends a message to the dead letter stream. It returns the
// XAdd error so the caller can avoid ACKing a message whose dead-letter entry
// was not enqueued (ARCH-011: unchecked re-queue/dead-letter error would
// otherwise lose the message).
func (w *Worker) sendToDeadLetter(ctx context.Context, stream string, msg redis.XMessage, reason, details string) error {
	deadStream := w.deadEventsStream
	if stream == w.streamPayments {
		deadStream = w.deadPaymentsStream
	}

	deadEntry := map[string]interface{}{
		"original_stream": stream,
		"original_id":     msg.ID,
		"reason":          reason,
		"details":         details,
		"failed_at":       time.Now().Unix(),
		"retry_count":     msg.Values["retry_count"],
		"original_data":   msg.Values["data"],
	}

	if err := w.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: deadStream,
		MaxLen: 10000, // Keep dead letter queue bounded
		Approx: true,
		ID:     "*",
		Values: deadEntry,
	}).Err(); err != nil {
		slog.Error("failed to send to dead letter", "stream", deadStream, "original_id", msg.ID, "err", err)
		return err
	}
	slog.Warn("message sent to dead letter", "stream", deadStream, "original_stream", stream, "original_id", msg.ID, "reason", reason)
	return nil
}

// reclaimPendingMessages periodically reclaims pending messages that have been idle too long
func (w *Worker) reclaimPendingMessages(ctx context.Context, wg *sync.WaitGroup) {
	ticker := time.NewTicker(pendingReclaimInterval)
	defer wg.Done()
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.reclaimPendingFromStream(ctx, w.streamEvents)
			w.reclaimPendingFromStream(ctx, w.streamPayments)
		}
	}
}

// reclaimPendingFromStream reclaims pending messages from a specific stream
func (w *Worker) reclaimPendingFromStream(ctx context.Context, stream string) {
	// Get list of consumers in the group
	consumers, err := w.rdb.XInfoConsumers(ctx, stream, w.group).Result()
	if err != nil {
		slog.Warn("failed to get consumers", "stream", stream, "err", err)
		return
	}

	// Check each consumer for pending messages
	for _, consumer := range consumers {
		// Get pending messages for this consumer
		pending, err := w.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
			Stream:   stream,
			Group:    w.group,
			Consumer: consumer.Name,
			Start:    "-",
			End:      "+",
			Count:    100,
		}).Result()
		if err != nil {
			slog.Warn("failed to get pending", "stream", stream, "consumer", consumer.Name, "err", err)
			continue
		}

		for _, p := range pending {
			// Check if message has been idle too long
			// p.Idle is time.Duration representing how long the message has been idle
			if p.Idle >= pendingIdleTimeout {
				// Try to claim the message
				cmd := w.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
					Stream:   stream,
					Group:    w.group,
					Consumer: w.consumer,
					MinIdle:  pendingIdleTimeout,
					Start:    p.ID,
					Count:    100,
				})
				claimed, _, err := cmd.Result()
				if err != nil && err != redis.Nil {
					slog.Warn("failed to auto-claim", "stream", stream, "id", p.ID, "err", err)
					continue
				}

				// Process claimed messages - claimed is []redis.XMessage
				for _, msg := range claimed {
					slog.Info("reclaimed pending message", "stream", stream, "id", msg.ID, "idle_time", p.Idle)
					if err := w.processMessage(ctx, stream, msg); err != nil {
						slog.Warn("failed to process reclaimed message", "stream", stream, "id", msg.ID, "err", err)
					}
				}
			}
		}
	}

	// Also check for any orphaned messages (no consumer)
	cmd := w.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    w.group,
		Consumer: w.consumer,
		MinIdle:  pendingIdleTimeout,
		Start:    "0-0",
		Count:    100,
	})
	orphaned, _, err := cmd.Result()
	if err != nil && err != redis.Nil {
		slog.Warn("failed to claim orphaned", "stream", stream, "err", err)
		return
	}

	for _, msg := range orphaned {
		slog.Info("reclaimed orphaned message", "stream", stream, "id", msg.ID)
		if err := w.processMessage(ctx, stream, msg); err != nil {
			slog.Warn("failed to process orphaned message", "stream", stream, "id", msg.ID, "err", err)
		}
	}
}
