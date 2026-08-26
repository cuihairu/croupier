// ---------------------------------------------------------------------------
// PipelineMonitor: 数据管道健康监控（短期数据监控方案）
//
// 在 server 进程内周期检查 analytics 管道的数据侧健康，异常写入告警中心
// （model.Alert），复用运维中心已有的告警列表/静默 UI：
//
//	clickhouse_unreachable  ClickHouse 不可达（非未启用）      critical
//	event_stream_stalled    某 (game,env) 事件断流（近 5 分钟为 0）  critical
//	event_volume_drop       事件量骤降（低于前 60 分钟均值 20%）   warning
//	dead_letter_backlog     死信流积压超阈值                  warning
//	mq_backlog              主 stream 积压（worker 停摆/消费不及） critical
//
// 判定全部基于 ClickHouse/Redis 的简单窗口对比，不引入外部依赖；恢复时
// 自动把 firing 告警置为 resolved。同 key 天然去抖（AlertID 唯一索引 +
// 先查后写）。未配置 CLICKHOUSE_DSN 时监控器自动空转（disabled）。
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

// 告警类型与来源常量（与告警中心 Type/Source 过滤器对齐）。
const (
	PipelineAlertSource         = "analytics-monitor"
	AlertTypeDataPipeline       = "data_pipeline"
	AlertTypeDataQuality        = "data_quality"
	PipelineAlertStatusFiring   = "firing"
	PipelineAlertStatusResolved = "resolved"
)

// PipelineMonitorConfig 控制 check 行为；零值字段取默认。全部可经 env 覆盖。
type PipelineMonitorConfig struct {
	Interval        time.Duration // 检查周期，默认 60s
	StallWindow     time.Duration // 断流窗口，默认 5m
	BaselineWindow  time.Duration // 基线窗口，默认 60m
	BaselineMinRows int64         // 基线最低事件量（低于不判定），默认 100
	DropRatio       float64       // 骤降阈值比例，默认 0.2
	DeadThreshold   int64         // 死信积压阈值，默认 100
	MQBacklogLimit  int64         // 主 stream 积压阈值，默认 10000
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

// PipelineAlertSink 是监控器写告警的最小接口（model.AlertModel 满足）。
type PipelineAlertSink interface {
	Create(ctx context.Context, alert *model.Alert) error
	FindByAlertID(ctx context.Context, alertID string) (*model.Alert, error)
	UpdateStatus(ctx context.Context, id uint, status string) error
}

// DeadLetterCounter 查询死信/主 stream 长度（*redis.Client 满足）。
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
	// states 记录上一轮每类 key 是否触发，用于恢复判定。
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
			if err := m.Check(ctx); err != nil {
				slog.Warn("pipeline monitor check", "err", err)
			}
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
		// 部署未启用分析仓库：不是故障，静默跳过全部 CH 检查。
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
		return // 未启用或不可达：reachability check 已单独处理
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
		m.fireAlert(ctx, "events_query_failed", AlertTypeDataPipeline, "warning",
			"管道监控查询 analytics.events 失败", map[string]any{"error": err.Error()})
		return
	}
	defer rows.Close()

	scoped := map[string]pipelineScopeRow{}
	for rows.Next() {
		var r pipelineScopeRow
		if err := rows.Scan(&r.Game, &r.Env, &r.Recent, &r.Baseline); err != nil {
			slog.Warn("pipeline monitor scan", "err", err)
			continue
		}
		if strings.TrimSpace(r.Game) == "" && strings.TrimSpace(r.Env) == "" {
			continue
		}
		scoped[scopeKey(r.Game, r.Env)] = r
	}

	// 断流 / 骤降
	fired := map[string]bool{}
	for k, r := range scoped {
		if r.Baseline < m.cfg.BaselineMinRows {
			continue // 低流量 scope 无法区分断流与正常低谷
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
		case float64(r.Recent)/m.cfg.StallWindow.Minutes() < (float64(r.Baseline)/m.cfg.BaselineWindow.Minutes())*m.cfg.DropRatio:
			fired[dropKey] = true
			m.fireAlert(ctx, dropKey, AlertTypeDataQuality, "warning",
				fmt.Sprintf("事件量骤降：%s/%s 近 %s 仅 %d 条，低于前 %s 均值的 %.0f%%",
					r.Game, r.Env, m.cfg.StallWindow, r.Recent, m.cfg.BaselineWindow, m.cfg.DropRatio*100),
				map[string]any{"gameId": r.Game, "env": r.Env, "recent": r.Recent, "baseline": r.Baseline})
		}
	}
	// 恢复判定：上一轮 firing 且本轮未再触发 → resolved
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, kind := range []string{"event_stream_stalled", "event_volume_drop"} {
		for k := range m.states {
			if !strings.HasPrefix(k, kind+":") {
				continue
			}
			if fired[k] {
				continue
			}
			// scope 可能已从结果集中消失（极低流量），同样视为恢复
			m.resolveAsync(ctx, k)
		}
	}
	// 记录本轮触发状态
	for k := range m.states {
		if strings.HasPrefix(k, "event_") {
			delete(m.states, k)
		}
	}
	for k := range fired {
		m.states[k] = true
	}
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
			continue // Redis 不可达由 MQ backlog/运维中心兜底，不重复告警
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
		// resolved → 重新触发
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
