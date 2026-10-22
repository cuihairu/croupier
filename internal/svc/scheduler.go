package svc

import (
	"context"
	"log/slog"
	"time"

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

// StartScheduler 启动 cron 调度循环（幂等：重复调用无副作用）。
//
// 调度器依赖 TaskScheduleModel 与 Dispatcher，二者就绪后即可启动；
// 触发链路复用异步任务派发，失败只记日志不影响主服务。
// 启动前播种周期报表调度（incident-reports §6，幂等按 name）；注入了
// ReportRunner 时 Kind=incident_report 的调度走 server-local 执行。
func (ctx *ServiceContext) StartScheduler() *scheduler.Manager {
	if ctx == nil || ctx.TaskScheduleModel == nil || ctx.Dispatcher == nil {
		return nil
	}
	if ctx.Scheduler != nil {
		return ctx.Scheduler
	}
	ctx.seedIncidentReportSchedules()
	mgr := scheduler.NewManager(ctx.TaskScheduleModel, dispatcherAdapter{d: ctx.Dispatcher})
	if ctx.ReportRunner != nil {
		mgr.SetLocalRunner(ctx.ReportRunner)
	}
	mgr.Start()
	ctx.Scheduler = mgr
	slog.Default().Info("task scheduler started", "interval", "30s")
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
