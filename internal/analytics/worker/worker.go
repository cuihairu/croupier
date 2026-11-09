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
	skipFinalFlush := false
	if !waitGroupBounded(&flushWG, shutdownTimeout) {
		slog.Warn("flush goroutine exceeded shutdown timeout", "timeout", shutdownTimeout)
		// 审核发现 727：周期 flush 仍在执行时，如果此时再启动最终 flush，
		// 两个 flush 会并发执行——aggMu 只保护共享状态转移，ClickHouse I/O
		// 在锁外，同 key 双写会产生重复/竞态。正确做法：最终 flush 只在
		// 周期 flush goroutine 已完全退出后才执行；如果它卡死超时，宁可
		// 放弃本次最终 flush（剩余数据仍留在 Redis stream pending，由重启
		// 后的 worker 继续处理），也不能并发双写。
		skipFinalFlush = true
	}
	if !waitGroupBounded(&reclaimWG, shutdownTimeout) {
		slog.Warn("reclaim goroutine exceeded shutdown timeout", "timeout", shutdownTimeout)
	}
	if skipFinalFlush {
		return ctx.Err()
	}

	flushCtx, cancelFlush := context.WithTimeout(context.WithoutCancel(ctx), finalFlushTimeout)
	// cancelFlush 也作为阻塞 Send() 不吃 ctx 的兜底退出信号：主流程超时
	// 返回后，后台 flush Goroutine仍能在 defer cancelFlush 时解除测试/
	// 生产自持有锁的等待，不再永久泄漏。
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
