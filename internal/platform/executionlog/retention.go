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
	// ResolveDays 可选的动态保留期来源（L3 log.retentionDays 覆盖，OPEN-ISSUES
	// #54）：每轮 Sweep 前调用一次，返回 (executionLogDays, taskLogDays)；
	// 返回值 >0 时覆盖静态字段，<=0 时沿用静态字段。nil 时仅用静态配置。
	ResolveDays func() (execDays, taskDays int)
	// Router 为 multiGame 模式的 per-game 库路由；nil 时仅清理 meta 库
	// （单库模式）。多游戏模式下 execution_logs/task_runs 分散在各 game 库，
	// 需逐库清理。
	Router     DBRouter
	GameScopes func(ctx context.Context) ([]GameScopeRef, error)
}

// GameScopeRef 一个 (gameID, env) 物理库引用。
type GameScopeRef struct {
	GameID string
	Env    string
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

// effectiveDays 每轮清理前的保留期合成：ResolveDays（L3 覆盖）>0 时压过静态配置。
func (r *Retention) effectiveDays() (execDays, taskDays int) {
	execDays, taskDays = r.cfg.ExecutionLogDays, r.cfg.TaskLogDays
	if r.cfg.ResolveDays == nil {
		return execDays, taskDays
	}
	d1, d2 := r.cfg.ResolveDays()
	if d1 > 0 {
		execDays = d1
	}
	if d2 > 0 {
		taskDays = d2
	}
	return execDays, taskDays
}

// Sweep 执行单轮清理，返回摘要。
func (r *Retention) Sweep(ctx context.Context) RetentionSummary {
	var summary RetentionSummary
	now := time.Now().UTC()
	execDays, taskDays := r.effectiveDays()
	if execDays > 0 {
		cutoff := now.AddDate(0, 0, -execDays)
		deleted, err := r.sweepExecutionLogs(ctx, cutoff)
		if err != nil {
			slog.WarnContext(ctx, "execution_logs retention sweep failed", "error", err)
		}
		summary.ExecutionLogsDeleted = deleted
		summary.ExecutionLogCutoff = cutoff
	}
	if taskDays > 0 {
		cutoff := now.AddDate(0, 0, -taskDays)
		runDeleted, eventDeleted, err := r.sweepTaskLogs(ctx, cutoff)
		if err != nil {
			slog.WarnContext(ctx, "task log retention sweep failed", "error", err)
		}
		summary.TaskRunsDeleted = runDeleted
		summary.TaskEventsDeleted = eventDeleted
		summary.TaskLogCutoff = cutoff
	}
	return summary
}

// PurgeBefore 手动清理（OPEN-ISSUES #54）：按任意 cutoff 清理留痕表，
// scope 取 "execution"（仅 execution_logs）/"task"（task_runs+task_events）/
// "all"。multiGame 逐 game 库 fanout 与 Sweep 同路径；audit_records 不参与。
func (r *Retention) PurgeBefore(ctx context.Context, cutoff time.Time, scope string) (RetentionSummary, error) {
	var summary RetentionSummary
	if scope == "execution" || scope == "all" {
		deleted, err := r.sweepExecutionLogs(ctx, cutoff)
		if err != nil {
			return summary, err
		}
		summary.ExecutionLogsDeleted = deleted
		summary.ExecutionLogCutoff = cutoff
	}
	if scope == "task" || scope == "all" {
		runDeleted, eventDeleted, err := r.sweepTaskLogs(ctx, cutoff)
		if err != nil {
			summary.TaskRunsDeleted = runDeleted
			return summary, err
		}
		summary.TaskRunsDeleted = runDeleted
		summary.TaskEventsDeleted = eventDeleted
		summary.TaskLogCutoff = cutoff
	}
	return summary, nil
}

// sweepExecutionLogs 清理 execution_logs：单库清 meta；multiGame 清全部
// game 库（逐 scope），meta 兜底（含历史误写残留）。
func (r *Retention) sweepExecutionLogs(ctx context.Context, cutoff time.Time) (int64, error) {
	logModel := model.NewExecutionLogModel(r.db)
	var total int64
	deleted, err := logModel.DeleteBefore(ctx, cutoff, 1000)
	if err != nil {
		return 0, err
	}
	total += deleted
	if r.cfg.Router == nil || r.cfg.GameScopes == nil {
		return total, nil
	}
	scopes, err := r.cfg.GameScopes(ctx)
	if err != nil {
		return total, err
	}
	for _, scope := range scopes {
		gctx, _, err := r.cfg.Router.Resolve(ctx, scope.GameID, scope.Env)
		if err != nil {
			continue
		}
		deleted, err := logModel.DeleteBefore(gctx, cutoff, 1000)
		if err != nil {
			slog.WarnContext(ctx, "game execution_logs sweep failed",
				"gameId", scope.GameID, "env", scope.Env, "error", err)
			continue
		}
		total += deleted
	}
	return total, nil
}

// sweepTaskLogs 清理 task_runs/task_events（单库 meta；multiGame 的任务
// 留痕当前落 meta 库，game 库暂无这两表）。
func (r *Retention) sweepTaskLogs(ctx context.Context, cutoff time.Time) (int64, int64, error) {
	runDeleted, err := model.NewTaskRunModel(r.db).DeleteBefore(ctx, cutoff, 1000)
	if err != nil {
		return 0, 0, err
	}
	eventDeleted, err := model.NewTaskEventModel(r.db).DeleteBefore(ctx, cutoff, 1000)
	if err != nil {
		return runDeleted, 0, err
	}
	return runDeleted, eventDeleted, nil
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
