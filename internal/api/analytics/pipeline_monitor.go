// ---------------------------------------------------------------------------
// PipelineMonitor: data pipeline health monitoring (short-term data monitoring plan)
//
// Periodically checks the data-side health of the analytics pipeline inside the server
// process and writes anomalies to the alert center (model.Alert), reusing the alert
// list / silence UI that the ops center already has:
//
//	clickhouse_unreachable  ClickHouse unreachable (not merely disabled)         critical
//	event_stream_stalled    events stalled for some (game,env) (0 in the last 5 min) critical
//	event_volume_drop       event volume dropped (below the average of the prior 60 min by 20%) warning
//	dead_letter_backlog     dead letter stream backlog past the threshold     warning
//	mq_backlog              main stream backlog (worker stalled / cannot keep up) critical
//
// All decisions are simple window comparisons against ClickHouse/Redis and pull in no
// external dependency; on recovery the firing alert is automatically set to resolved.
// Recovery requires positive evidence (DATA-022): an alert is closed only by a scope
// row that was actually read and satisfies the recovery rule; a scope whose data is
// missing from the result set, or a round with a failed/partial query, is unknown and
// keeps the existing alert firing. The same key is naturally debounced (AlertID unique
// index + read-then-write). When CLICKHOUSE_DSN is not configured the monitor idles on
// its own (disabled).
// ---------------------------------------------------------------------------
package analytics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	redis "github.com/redis/go-redis/v9"
)

// Alert type and source constants (aligned with the alert center Type/Source filters).
const (
	PipelineAlertSource         = "analytics-monitor"
	AlertTypeDataPipeline       = "data_pipeline"
	AlertTypeDataQuality        = "data_quality"
	PipelineAlertStatusFiring   = "firing"
	PipelineAlertStatusResolved = "resolved"
)

// PipelineMonitorConfig controls check behavior; zero-value fields fall back to defaults.
// Every field can be overridden through env.
type PipelineMonitorConfig struct {
	Interval        time.Duration // check interval, default 60s
	StallWindow     time.Duration // stall window, default 5m
	BaselineWindow  time.Duration // baseline window, default 60m
	BaselineMinRows int64         // minimum event volume for a baseline (no verdict below it), default 100
	DropRatio       float64       // volume-drop threshold ratio, default 0.2
	DeadThreshold   int64         // dead letter backlog threshold, default 100
	MQBacklogLimit  int64         // main stream backlog threshold, default 10000
}

func pipelineConfigFromEnv() PipelineMonitorConfig {
	cfg := PipelineMonitorConfig{
		Interval:        60 * time.Second,
		StallWindow:     5 * time.Minute,
		BaselineWindow:  60 * time.Minute,
		BaselineMinRows: 100,
		DropRatio:       0.2,
		DeadThreshold:   100,
		MQBacklogLimit:  10000,
	}
	if v := os.Getenv("PIPELINE_MONITOR_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.Interval = d
		}
	}
	if v := os.Getenv("PIPELINE_MONITOR_DEAD_THRESHOLD"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			cfg.DeadThreshold = n
		}
	}
	if v := os.Getenv("PIPELINE_MONITOR_MQ_BACKLOG"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			cfg.MQBacklogLimit = n
		}
	}
	return cfg
}

// PipelineAlertSink is the minimal interface the monitor uses to write alerts
// (satisfied by model.AlertModel).
type PipelineAlertSink interface {
	Create(ctx context.Context, alert *model.Alert) error
	FindByAlertID(ctx context.Context, alertID string) (*model.Alert, error)
	UpdateStatus(ctx context.Context, id uint, status string) error
}

// DeadLetterCounter queries dead letter / main stream lengths (satisfied by *redis.Client).
type DeadLetterCounter interface {
	XLen(ctx context.Context, stream string) *redis.IntCmd
}

type PipelineMonitor struct {
	cfg    PipelineMonitorConfig
	conn   func() (warehouseConn, error)
	sink   PipelineAlertSink
	dead   DeadLetterCounter
	stream struct {
		events       string
		payments     string
		deadEvents   string
		deadPayments string
	}
	// states records whether each key kind fired on the previous round, for recovery detection.
	states map[string]bool
	mu     sync.Mutex
}

// NewPipelineMonitor builds a monitor wired to the shared warehouse
// ClickHouse connection and the alert model. redisClient may be nil (dead
// letter / MQ backlog checks are then skipped).
func NewPipelineMonitor(sink PipelineAlertSink, redisClient DeadLetterCounter) *PipelineMonitor {
	m := &PipelineMonitor{
		cfg:  pipelineConfigFromEnv(),
		conn: warehouseConnect,
		sink: sink,
		dead: redisClient,
	}
	m.stream.events = envOr("ANALYTICS_REDIS_STREAM_EVENTS", "analytics:events")
	m.stream.payments = envOr("ANALYTICS_REDIS_STREAM_PAYMENTS", "analytics:payments")
	m.stream.deadEvents = envOr("ANALYTICS_DEAD_EVENTS_STREAM", "analytics:events:dead")
	m.stream.deadPayments = envOr("ANALYTICS_DEAD_PAYMENTS_STREAM", "analytics:payments:dead")
	m.states = map[string]bool{}
	return m
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// Run blocks until ctx is done, executing checks every Interval.
func (m *PipelineMonitor) Run(ctx context.Context) {
	interval := m.cfg.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	tk := time.NewTicker(interval)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			// The Check implementation always returns nil (each sub-check records its own
			// failures as alert events), so the err branch is unreachable and has been removed.
			_ = m.Check(ctx)
		}
	}
}

// Check executes one monitoring pass.
func (m *PipelineMonitor) Check(ctx context.Context) error {
	if m == nil || m.sink == nil {
		return nil
	}
	m.checkClickHouseReachable(ctx)
	m.checkEventStreams(ctx)
	m.checkDeadLetters(ctx)
	m.checkMQBacklog(ctx)
	return nil
}

// ---- ClickHouse reachability ----

func (m *PipelineMonitor) checkClickHouseReachable(ctx context.Context) {
	const key = "clickhouse_unreachable"
	_, err := m.conn()
	if err == nil {
		m.resolveIfFiring(ctx, key, "ClickHouse 已恢复可达")
		return
	}
	if errors.Is(err, errWarehouseDisabled) {
		// The deployment has not enabled the analytics warehouse: not a fault, silently skip all CH checks.
		return
	}
	m.fireAlert(ctx, key, AlertTypeDataPipeline, "critical",
		"ClickHouse 不可达：分析数据无法写入/查询",
		map[string]any{"error": err.Error()})
}

// ---- Event stream stall / volume drop ----

type pipelineScopeRow struct {
	Game     string
	Env      string
	Recent   int64
	Baseline int64
}

func (m *PipelineMonitor) checkEventStreams(ctx context.Context) {
	conn, err := m.conn()
	if err != nil {
		return // disabled or unreachable: the reachability check handles that separately
	}
	rows, err := conn.Query(ctx, fmt.Sprintf(`
SELECT game_id, env,
  countIf(event_time >= now() - INTERVAL %d MINUTE) AS recent,
  countIf(event_time >= now() - INTERVAL %d MINUTE
      AND event_time < now() - INTERVAL %d MINUTE) AS baseline
FROM analytics.events
WHERE event_time >= now() - INTERVAL %d MINUTE
GROUP BY game_id, env`,
		int(m.cfg.StallWindow.Minutes()),
		int(m.cfg.BaselineWindow.Minutes()),
		int(m.cfg.StallWindow.Minutes()),
		int(m.cfg.BaselineWindow.Minutes())))
	if err != nil {
		// Unknown, not recovered (DATA-022): nothing is known about any scope
		// this round. Existing alerts stay firing; the failure is marked
		// separately instead of being read as recovery.
		m.fireAlert(ctx, "events_query_failed", AlertTypeDataPipeline, "warning",
			"管道监控查询 analytics.events 失败", map[string]any{"error": err.Error()})
		return
	}
	defer func() { _ = rows.Close() }()

	scoped := map[string]pipelineScopeRow{}
	partial := false
	var scanErr error
	for rows.Next() {
		var r pipelineScopeRow
		if err := rows.Scan(&r.Game, &r.Env, &r.Recent, &r.Baseline); err != nil {
			slog.Warn("pipeline monitor scan", "err", err)
			partial = true
			scanErr = err
			continue
		}
		if strings.TrimSpace(r.Game) == "" && strings.TrimSpace(r.Env) == "" {
			continue
		}
		scoped[scopeKey(r.Game, r.Env)] = r
	}
	if partial {
		// Partial data: the scopes behind the unread rows are unknown, not
		// healthy. Mark the round incomplete; only positively evaluated rows
		// may close an alert below.
		m.fireAlert(ctx, "events_query_failed", AlertTypeDataPipeline, "warning",
			"管道监控仅读到部分作用域数据，未读数按未知处理",
			map[string]any{"partial": true, "error": scanErr.Error()})
	} else {
		m.resolveIfFiring(ctx, "events_query_failed", "管道监控查询已恢复")
	}

	// Stall / volume drop. scoped holds every row actually read this round;
	// only those scopes carry evidence for closing an alert.
	fired := map[string]bool{}
	for k, r := range scoped {
		if r.Baseline < m.cfg.BaselineMinRows {
			continue // a low-traffic scope cannot be distinguished between a stall and a normal trough
		}
		stallKey := "event_stream_stalled:" + k
		dropKey := "event_volume_drop:" + k
		switch {
		case r.Recent == 0:
			fired[stallKey] = true
			m.fireAlert(ctx, stallKey, AlertTypeDataPipeline, "critical",
				fmt.Sprintf("事件断流：%s/%s 近 %s 无任何事件（前 %s 基线 %d 条）",
					r.Game, r.Env, m.cfg.StallWindow, m.cfg.BaselineWindow, r.Baseline),
				map[string]any{"gameId": r.Game, "env": r.Env, "recent": r.Recent, "baseline": r.Baseline})
		case !m.rateHealthy(r):
			fired[dropKey] = true
			m.fireAlert(ctx, dropKey, AlertTypeDataQuality, "warning",
				fmt.Sprintf("事件量骤降：%s/%s 近 %s 仅 %d 条，低于前 %s 均值的 %.0f%%",
					r.Game, r.Env, m.cfg.StallWindow, r.Recent, m.cfg.BaselineWindow, m.cfg.DropRatio*100),
				map[string]any{"gameId": r.Game, "env": r.Env, "recent": r.Recent, "baseline": r.Baseline})
		}
	}
	// Recovery detection (DATA-022): only positive evidence closes an alert.
	// A scope that vanished from the result set (fully silent, i.e. exactly the
	// stall being alerted) or whose row was unreadable this round is unknown and
	// must keep its alert. Resolution is driven by the evaluated rows rather
	// than by in-memory firing state, so a monitor restart cannot strand an
	// alert whose scope has already recovered.
	m.mu.Lock()
	for k, r := range scoped {
		if stallKey := "event_stream_stalled:" + k; !fired[stallKey] && r.Recent > 0 {
			m.resolveAsync(ctx, stallKey)
		}
		// A volume-drop alert is cleared only by a healthy rate reading. Below
		// the baseline floor the rate is unknowable: keep the alert firing.
		dropKey := "event_volume_drop:" + k
		if !fired[dropKey] && r.Recent > 0 && r.Baseline >= m.cfg.BaselineMinRows && m.rateHealthy(r) {
			m.resolveAsync(ctx, dropKey)
		}
	}
	// Record the firing state for this round
	for k := range m.states {
		if strings.HasPrefix(k, "event_") {
			delete(m.states, k)
		}
	}
	for k := range fired {
		m.states[k] = true
	}
	m.mu.Unlock()
}

// rateHealthy reports whether the recent event rate matches the baseline rate
// closely enough to clear a volume-drop alert. Each rate is computed over its
// own actual window (DATA-022): recent over StallWindow, baseline over
// BaselineWindow, so a non-default window pair does not skew the comparison.
func (m *PipelineMonitor) rateHealthy(r pipelineScopeRow) bool {
	recentRate := float64(r.Recent) / m.cfg.StallWindow.Minutes()
	baselineRate := float64(r.Baseline) / m.cfg.BaselineWindow.Minutes()
	return recentRate >= baselineRate*m.cfg.DropRatio
}

// ---- Dead letter backlog ----

func (m *PipelineMonitor) checkDeadLetters(ctx context.Context) {
	if m.dead == nil {
		return
	}
	for _, s := range []struct{ stream, key string }{
		{m.stream.deadEvents, "dead_letter_backlog:events"},
		{m.stream.deadPayments, "dead_letter_backlog:payments"},
	} {
		n, err := m.dead.XLen(ctx, s.stream).Result()
		if err != nil {
			continue // Redis being unreachable is covered by the MQ backlog / ops center, no duplicate alert
		}
		key := s.key
		if n >= m.cfg.DeadThreshold {
			m.fireAlert(ctx, key, AlertTypeDataQuality, "warning",
				fmt.Sprintf("死信积压：%s 达 %d 条（阈值 %d），存在被丢弃的数据",
					s.stream, n, m.cfg.DeadThreshold),
				map[string]any{"stream": s.stream, "length": n, "threshold": m.cfg.DeadThreshold})
		} else {
			m.resolveIfFiring(ctx, key, fmt.Sprintf("死信已清理：%s 当前 %d 条", s.stream, n))
		}
	}
}

// ---- MQ backlog (worker stalled / ingest surge) ----

func (m *PipelineMonitor) checkMQBacklog(ctx context.Context) {
	if m.dead == nil {
		return
	}
	for _, s := range []struct{ stream, key string }{
		{m.stream.events, "mq_backlog:events"},
		{m.stream.payments, "mq_backlog:payments"},
	} {
		n, err := m.dead.XLen(ctx, s.stream).Result()
		if err != nil {
			continue
		}
		key := s.key
		if n >= m.cfg.MQBacklogLimit {
			m.fireAlert(ctx, key, AlertTypeDataPipeline, "critical",
				fmt.Sprintf("消息积压：%s 达 %d 条（阈值 %d），worker 可能停摆或消费不及",
					s.stream, n, m.cfg.MQBacklogLimit),
				map[string]any{"stream": s.stream, "length": n, "threshold": m.cfg.MQBacklogLimit})
		} else {
			m.resolveIfFiring(ctx, key, fmt.Sprintf("消息积压恢复：%s 当前 %d 条", s.stream, n))
		}
	}
}

// ---- alert helpers ----

func scopeKey(game, env string) string {
	return game + "/" + env
}

func alertIDFor(key string) string {
	return "pipe:" + key
}

// fireAlert upserts a firing alert; existing firing alerts are left as-is
// (dedup via AlertID unique index + find-first).
func (m *PipelineMonitor) fireAlert(ctx context.Context, key, alertType, level, message string, details map[string]any) {
	if m.sink == nil {
		return
	}
	alertID := alertIDFor(key)
	existing, err := m.sink.FindByAlertID(ctx, alertID)
	if err == nil && existing != nil {
		if existing.Status == PipelineAlertStatusFiring {
			return
		}
		// resolved -> re-fire
		_ = m.sink.UpdateStatus(ctx, existing.ID, PipelineAlertStatusFiring)
		slog.Warn("pipeline alert re-fired", "key", key, "level", level)
		return
	}
	if err != nil && !isNotFound(err) {
		slog.Warn("pipeline alert lookup", "key", key, "err", err)
		return
	}
	alert := &model.Alert{
		AlertID: alertID,
		Type:    alertType,
		Level:   level,
		Message: message,
		Source:  PipelineAlertSource,
		Status:  PipelineAlertStatusFiring,
		Details: details,
	}
	if err := m.sink.Create(ctx, alert); err != nil {
		slog.Warn("pipeline alert create", "key", key, "err", err)
		return
	}
	slog.Warn("pipeline alert fired", "key", key, "level", level, "message", message)
}

// resolveIfFiring marks an existing firing alert resolved.
func (m *PipelineMonitor) resolveIfFiring(ctx context.Context, key, message string) {
	if m.sink == nil {
		return
	}
	alertID := alertIDFor(key)
	existing, err := m.sink.FindByAlertID(ctx, alertID)
	if err != nil || existing == nil {
		return
	}
	if existing.Status != PipelineAlertStatusFiring {
		return
	}
	if err := m.sink.UpdateStatus(ctx, existing.ID, PipelineAlertStatusResolved); err != nil {
		slog.Warn("pipeline alert resolve", "key", key, "err", err)
		return
	}
	slog.Info("pipeline alert resolved", "key", key, "message", message)
}

// resolveAsync is the deferred variant used while holding the state lock.
func (m *PipelineMonitor) resolveAsync(ctx context.Context, key string) {
	delete(m.states, key)
	go m.resolveIfFiring(ctx, key, "事件流恢复")
}

func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}
