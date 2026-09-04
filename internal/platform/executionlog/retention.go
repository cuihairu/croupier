package executionlog

import (
	"context"
	"log/slog"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

const defaultSweepInterval = time.Hour

// RetentionConfig 保留期配置（R3）：0=永久保留，未配置由调用方给默认 7。
type RetentionConfig struct {
	ExecutionLogDays int
	TaskLogDays      int
	Interval         time.Duration
}

// Retention 定期清理 payload 级留痕数据。
//
// 明确边界：audit_records（哈希链审计）不参与清理——删链记录会破坏审计
// 链完整性，且审计元数据体量小、合规上通常要求更长期保存。
type Retention struct {
	db  *gorm.DB
	cfg RetentionConfig
}

func NewRetention(db *gorm.DB, cfg RetentionConfig) *Retention {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultSweepInterval
	}
	return &Retention{db: db, cfg: cfg}
}

// Run 周期性清理，直到 ctx 取消。
func (r *Retention) Run(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(r.cfg.Interval)
		defer ticker.Stop()
		// 启动即先清一次积压
		r.sweepLogged(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.sweepLogged(ctx)
			}
		}
	}()
}

// Sweep 执行单轮清理，返回摘要。
func (r *Retention) Sweep(ctx context.Context) RetentionSummary {
	var summary RetentionSummary
	now := time.Now().UTC()
	if r.cfg.ExecutionLogDays > 0 {
		cutoff := now.AddDate(0, 0, -r.cfg.ExecutionLogDays)
		deleted, err := model.NewExecutionLogModel(r.db).DeleteBefore(ctx, cutoff, 1000)
		if err != nil {
			slog.WarnContext(ctx, "execution_logs retention sweep failed", "error", err)
		}
		summary.ExecutionLogsDeleted = deleted
		summary.ExecutionLogCutoff = cutoff
	}
	if r.cfg.TaskLogDays > 0 {
		cutoff := now.AddDate(0, 0, -r.cfg.TaskLogDays)
		runDeleted, err := model.NewTaskRunModel(r.db).DeleteBefore(ctx, cutoff, 1000)
		if err != nil {
			slog.WarnContext(ctx, "task_runs retention sweep failed", "error", err)
		}
		summary.TaskRunsDeleted = runDeleted
		deleted, err := model.NewTaskEventModel(r.db).DeleteBefore(ctx, cutoff, 1000)
		if err != nil {
			slog.WarnContext(ctx, "task_events retention sweep failed", "error", err)
		}
		summary.TaskEventsDeleted = deleted
		summary.TaskLogCutoff = cutoff
	}
	return summary
}

type RetentionSummary struct {
	ExecutionLogsDeleted int64
	TaskRunsDeleted      int64
	TaskEventsDeleted    int64
	ExecutionLogCutoff   time.Time
	TaskLogCutoff        time.Time
}

func (r *Retention) sweepLogged(ctx context.Context) {
	summary := r.Sweep(ctx)
	if summary.ExecutionLogsDeleted+summary.TaskRunsDeleted+summary.TaskEventsDeleted > 0 {
		slog.InfoContext(ctx, "retention sweep completed",
			"executionLogs", summary.ExecutionLogsDeleted,
			"taskRuns", summary.TaskRunsDeleted,
			"taskEvents", summary.TaskEventsDeleted)
	}
}
