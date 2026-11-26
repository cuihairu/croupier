package executionlog

import (
	"context"
	"log/slog"
	"sync"
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
	// wg 守 Run 起的清理循环；Wait 供装配方 Close 收尾（cancel 后等待在飞
	// sweep 结束，杜绝 teardown 与懒开库的竞态）。
	wg sync.WaitGroup
}

func NewRetention(db *gorm.DB, cfg RetentionConfig) *Retention {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultSweepInterval
	}
	return &Retention{db: db, cfg: cfg}
}

// Run 周期性清理，直到 ctx 取消。goroutine 经 Wait 等待退出。
func (r *Retention) Run(ctx context.Context) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
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

// Wait 阻塞至 Run 的清理 goroutine 退出；未 Run 过立即返回。
func (r *Retention) Wait() {
	if r == nil {
		return
	}
	r.wg.Wait()
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
		deleted, err := r.sweepExecutionLogs(ctx, cutoff, &summary)
		if err != nil {
			slog.WarnContext(ctx, "execution_logs retention sweep failed", "error", err)
		}
		summary.ExecutionLogsDeleted = deleted
		summary.ExecutionLogCutoff = cutoff
	}
	// AUD-057：meta 缺表不再短路 multiGame——任务留痕在各 game 物理库，
	// Router 已装配时仍要进入逐库清理（库内缺表由逐库守卫处理）。
	if taskDays > 0 && (r.taskLogTablesPresent() || r.cfg.Router != nil) {
		cutoff := now.AddDate(0, 0, -taskDays)
		runDeleted, eventDeleted, err := r.sweepTaskLogs(ctx, cutoff, &summary)
		if err != nil {
			slog.WarnContext(ctx, "task log retention sweep failed", "error", err)
		}
		summary.TaskRunsDeleted = runDeleted
		summary.TaskEventsDeleted = eventDeleted
		summary.TaskLogCutoff = cutoff
	}
	return summary
}

// taskLogTablesPresent 周期清理的缺表守卫：task_runs/task_events 是 game 域
// 模型，裁剪部署或测试的 meta 库可能没建——单库模式下缺表即无数据可清，
// 周期 Sweep 直接跳过（避免每轮刷 no such table 告警噪声）。multiGame 模式
// （Router 已装配）不以此为准：任务留痕在各 game 物理库（AUD-057），meta
// 缺表不代表 game 库缺表，逐库守卫在 sweepTaskLogs 内。手动 PurgeBefore 不
// 经此守卫，缺表仍如实报错（契约见 TestRetentionPurgeBefore_ErrorWings）。
func (r *Retention) taskLogTablesPresent() bool {
	return r.db.Migrator().HasTable("task_runs") && r.db.Migrator().HasTable("task_events")
}

// PurgeBefore 手动清理（OPEN-ISSUES #54）：按任意 cutoff 清理留痕表，
// scope 取 "execution"（仅 execution_logs）/"task"（task_runs+task_events）/
// "all"。multiGame 逐 game 库 fanout 与 Sweep 同路径；audit_records 不参与。
func (r *Retention) PurgeBefore(ctx context.Context, cutoff time.Time, scope string) (RetentionSummary, error) {
	var summary RetentionSummary
	if scope == "execution" || scope == "all" {
		deleted, err := r.sweepExecutionLogs(ctx, cutoff, &summary)
		if err != nil {
			return summary, err
		}
		summary.ExecutionLogsDeleted = deleted
		summary.ExecutionLogCutoff = cutoff
	}
	if scope == "task" || scope == "all" {
		runDeleted, eventDeleted, err := r.sweepTaskLogs(ctx, cutoff, &summary)
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
// game 库（逐 scope），meta 兜底（含历史误写残留）。解析/删除失败的 scope
// 记入 summary.FailedScopes（汇报失败范围），不中断其余库。
func (r *Retention) sweepExecutionLogs(ctx context.Context, cutoff time.Time, summary *RetentionSummary) (int64, error) {
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
			summary.FailedScopes = append(summary.FailedScopes, scopeRefKey(scope))
			continue
		}
		deleted, err := logModel.DeleteBefore(gctx, cutoff, 1000)
		if err != nil {
			slog.WarnContext(ctx, "game execution_logs sweep failed",
				"gameId", scope.GameID, "env", scope.Env, "error", err)
			summary.FailedScopes = append(summary.FailedScopes, scopeRefKey(scope))
			continue
		}
		total += deleted
	}
	return total, nil
}

// sweepTaskLogs 清理 task_runs/task_events。AUD-057 勘误：二者是 game 域
// 模型，multiGame 模式下任务留痕落在各 game 物理库（meta/croupier_meta 不
// 建这两表）——单库清 meta；multiGame 逐 scope 清对应 game 库，meta 仅兜底
// 历史残留（multiGame 下 meta 缺表跳过，不算失败）。逐库双表皆缺即无数据
// 可清（裁剪库容忍跳过）；解析/删除失败的 scope 记入 summary.FailedScopes。
func (r *Retention) sweepTaskLogs(ctx context.Context, cutoff time.Time, summary *RetentionSummary) (int64, int64, error) {
	var runDeleted, eventDeleted int64
	if r.taskLogTablesPresent() || r.cfg.Router == nil {
		// meta 段：单库模式直接清（缺表由驱动如实报错，PurgeBefore 缺表
		// 契约）；multiGame 模式下 meta 兜底历史残留。
		rd, err := model.NewTaskRunModel(r.db).DeleteBefore(ctx, cutoff, 1000)
		if err != nil {
			return 0, 0, err
		}
		ed, err := model.NewTaskEventModel(r.db).DeleteBefore(ctx, cutoff, 1000)
		if err != nil {
			return rd, 0, err
		}
		runDeleted, eventDeleted = rd, ed
	}
	if r.cfg.Router == nil || r.cfg.GameScopes == nil {
		return runDeleted, eventDeleted, nil
	}
	scopes, err := r.cfg.GameScopes(ctx)
	if err != nil {
		return runDeleted, eventDeleted, err
	}
	runModel := model.NewTaskRunModel(r.db)
	eventModel := model.NewTaskEventModel(r.db)
	for _, scope := range scopes {
		gctx, gdb, err := r.cfg.Router.Resolve(ctx, scope.GameID, scope.Env)
		if err != nil {
			summary.FailedScopes = append(summary.FailedScopes, scopeRefKey(scope))
			continue
		}
		// 裁剪 game 库双表皆缺 = 无数据可清，静默跳过（部分缺表仍尝试，
		// 让可清的表清掉、缺失的表报失败范围）
		migrator := gdb.Migrator()
		if !migrator.HasTable("task_runs") && !migrator.HasTable("task_events") {
			continue
		}
		rd, err := runModel.DeleteBefore(gctx, cutoff, 1000)
		if err != nil {
			slog.WarnContext(ctx, "game task_runs sweep failed",
				"gameId", scope.GameID, "env", scope.Env, "error", err)
			summary.FailedScopes = append(summary.FailedScopes, scopeRefKey(scope))
			continue
		}
		runDeleted += rd
		ed, err := eventModel.DeleteBefore(gctx, cutoff, 1000)
		if err != nil {
			slog.WarnContext(ctx, "game task_events sweep failed",
				"gameId", scope.GameID, "env", scope.Env, "error", err)
			summary.FailedScopes = append(summary.FailedScopes, scopeRefKey(scope))
			continue
		}
		eventDeleted += ed
	}
	return runDeleted, eventDeleted, nil
}

func scopeRefKey(scope GameScopeRef) string {
	return scope.GameID + "/" + scope.Env
}

type RetentionSummary struct {
	ExecutionLogsDeleted int64
	TaskRunsDeleted      int64
	TaskEventsDeleted    int64
	ExecutionLogCutoff   time.Time
	TaskLogCutoff        time.Time
	// FailedScopes multiGame fanout 中解析或清理失败的 (gameId, env) 范围
	//（"gameId/env"）；成功范围由删除计数与 cutoff 表达。nil=无失败。
	FailedScopes []string
}

func (r *Retention) sweepLogged(ctx context.Context) {
	summary := r.Sweep(ctx)
	if summary.ExecutionLogsDeleted+summary.TaskRunsDeleted+summary.TaskEventsDeleted > 0 {
		args := []interface{}{
			"executionLogs", summary.ExecutionLogsDeleted,
			"taskRuns", summary.TaskRunsDeleted,
			"taskEvents", summary.TaskEventsDeleted,
		}
		if len(summary.FailedScopes) > 0 {
			args = append(args, "failedScopes", summary.FailedScopes)
		}
		slog.InfoContext(ctx, "retention sweep completed", args...)
	}
	if len(summary.FailedScopes) > 0 {
		// 全部失败时删除计数为 0，失败范围仍要汇报（AUD-057）
		slog.WarnContext(ctx, "retention sweep had failed scopes",
			"failedScopes", summary.FailedScopes)
	}
}
