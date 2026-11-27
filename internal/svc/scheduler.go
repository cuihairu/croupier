package svc

import (
	"context"
	"log/slog"
	"time"

	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	scheduler "github.com/cuihairu/croupier/internal/tasks/scheduler"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// SetReportRunner 注入 server-local 报表执行面（cmd/server 启动时布线；
// 须在 StartScheduler 之前调用）。nil 清除。
func (ctx *ServiceContext) SetReportRunner(r scheduler.LocalRunner) {
	if ctx == nil {
		return
	}
	ctx.ReportRunner = r
}

// StartScheduler 装配 cron 调度循环（幂等：重复调用返回同一实例），但
// **不启动**循环——调用方在注入治理钩子（与停机钻机钩子）后自行
// Start()，使钩子在首 tick 前就位（ARCH-017 停机钻机需在循环启动前
// 注入卡死 tick 点）。依赖 TaskScheduleModel 与 Dispatcher，二者就绪后
// 即可装配；触发链路复用异步任务派发，失败只记日志不影响主服务。
// 装配前播种周期报表调度（incident-reports §6，幂等按 name）；注入了
// ReportRunner 时 Kind=incident_report 的调度走 server-local 执行。
func (ctx *ServiceContext) StartScheduler() *scheduler.Manager {
	if ctx == nil || ctx.TaskScheduleModel == nil || ctx.Dispatcher == nil {
		return nil
	}
	if ctx.Scheduler != nil {
		return ctx.Scheduler
	}
	// AUD-087：multiGame 模式下 task_runs 是 game 模型，live 在 schedule
	// 所属 (gameID, env) 的 per-game 库——终态回查按作用域路由到 game 库。
	// Router 为 nil（单库模式，默认 dev/CI）不注入：模型回退自身连接，
	// task_runs 与调度定义同库，行为不变。
	if ctx.Router != nil {
		ctx.TaskScheduleModel.SetScopeResolver(routerScheduleScopes{r: ctx.Router})
	}
	ctx.seedIncidentReportSchedules()
	ctx.repairMissingNextTriggers()
	mgr := scheduler.NewManager(ctx.TaskScheduleModel, dispatcherAdapter{d: ctx.Dispatcher})
	if ctx.ReportRunner != nil {
		mgr.SetLocalRunner(ctx.ReportRunner)
	}
	ctx.Scheduler = mgr
	slog.Default().Info("task scheduler created", "interval", "30s")
	return mgr
}

// seedIncidentReportSchedules 播种周/月报调度（幂等；HasTable 守卫在
// model 侧——multiGame 模式 meta 库无 task_schedules 表时静默跳过，报表
// 调度为单库模式的已知边界，multiGame 部署手动建调度）。
func (ctx *ServiceContext) seedIncidentReportSchedules() {
	n, err := model.SeedIncidentReportSchedules(context.Background(), ctx.TaskScheduleModel,
		func(cronExpr string) (time.Time, bool) {
			spec, err := scheduler.ParseCron(cronExpr)
			if err != nil {
				return time.Time{}, false
			}
			return spec.Next(time.Now()), true
		})
	if err != nil {
		slog.Default().Warn("seed incident report schedules failed", "error", err)
		return
	}
	if n > 0 {
		slog.Default().Info("seeded incident report schedules", "count", n)
	}
}

// repairMissingNextTriggers 盘点收敛存量死行（active 且 next_triggered_at 为
// NULL，ListDue 永远扫不出，AUD-088）。启动时运行一次，幂等；cron 非法的行
// 逐条报出来留人工处置。svc 侧经 scheduler.ParseCron 注入 nextAfter（model
// 不 import scheduler，防环，与 seedIncidentReportSchedules 同型）。
func (ctx *ServiceContext) repairMissingNextTriggers() {
	res, err := ctx.TaskScheduleModel.RepairMissingNextTrigger(context.Background(),
		func(cronExpr string) (time.Time, bool) {
			spec, err := scheduler.ParseCron(cronExpr)
			if err != nil {
				return time.Time{}, false
			}
			return spec.Next(time.Now()), true
		})
	if err != nil {
		slog.Default().Warn("repair missing next_triggered_at failed", "error", err)
		return
	}
	if len(res.Repaired) > 0 {
		slog.Default().Info("repaired schedules missing next_triggered_at", "count", len(res.Repaired))
	}
	for _, row := range res.Unrepairable {
		slog.Default().Warn("schedule has invalid cron, next_triggered_at left NULL", "schedule", row.ID, "cron", row.CronExpr)
	}
}

// StopScheduler 停止调度循环（server 优雅退出时调用；可为 nil）。
func (ctx *ServiceContext) StopScheduler() {
	if ctx == nil || ctx.Scheduler == nil {
		return
	}
	ctx.Scheduler.Stop()
	slog.Default().Info("task scheduler stopped")
}

// dispatcherAdapter 把 *dispatch.Dispatcher 适配为 scheduler.Dispatcher
// （调度触发统一走结构化 InvokeRequest 以携带 metadata）。
type dispatcherAdapter struct {
	d interface {
		StartTaskRequest(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error)
	}
}

func (a dispatcherAdapter) StartTask(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
	return a.d.StartTaskRequest(ctx, req)
}

// routerScheduleScopes 把 per-game 路由器适配为模型的 ScheduleScopeResolver
// （与 cmd/server 的 routerTaskScopes 同型）。解析错误必须透传：终态回查的
// 「查库失败」要与「记录不存在 / 未终态」可区分——静默吞错回退 meta 连接
// 会令 multiGame 下回查恒判「不存在」，正是 AUD-087 修复的缺陷形态。
type routerScheduleScopes struct {
	r *router.Router
}

func (a routerScheduleScopes) ScopeContext(ctx context.Context, gameID, env string) (context.Context, error) {
	scoped, _, err := a.r.Resolve(ctx, gameID, env)
	return scoped, err
}
