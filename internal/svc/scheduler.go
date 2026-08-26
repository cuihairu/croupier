package svc

import (
	"context"
	"log/slog"

	scheduler "github.com/cuihairu/croupier/internal/tasks/scheduler"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// StartScheduler 启动 cron 调度循环（幂等：重复调用无副作用）。
//
// 调度器依赖 TaskScheduleModel 与 Dispatcher，二者就绪后即可启动；
// 触发链路复用异步任务派发，失败只记日志不影响主服务。
func (ctx *ServiceContext) StartScheduler() *scheduler.Manager {
	if ctx == nil || ctx.TaskScheduleModel == nil || ctx.Dispatcher == nil {
		return nil
	}
	if ctx.Scheduler != nil {
		return ctx.Scheduler
	}
	mgr := scheduler.NewManager(ctx.TaskScheduleModel, dispatcherAdapter{d: ctx.Dispatcher})
	mgr.Start()
	ctx.Scheduler = mgr
	slog.Default().Info("task scheduler started", "interval", "30s")
	return mgr
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
