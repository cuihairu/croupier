package function

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionAnalyticsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionAnalyticsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionAnalyticsLogic {
	return &FunctionAnalyticsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// FunctionAnalytics 返回函数的真实调用统计，数据源为 execution_logs
// 执行留痕（REST invoke / 页面绑定执行）。零调用时 TotalCalls/SuccessRate
// 为 0——此前实现数的是 function_form 配置版本数且硬编码成功率 100%，
// 属占位伪造数据，已在 function-pipeline-blockers.md 记录并废弃。
// 口径边界：异步任务与 agent→游戏侧直连调用不经 execution_logs，
// 不在统计内（执行链路卡点 E3/E10）。
func (l *FunctionAnalyticsLogic) FunctionAnalytics(req *FunctionAnalyticsRequest) (*FunctionAnalyticsResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}
	if _, err := getOrCreateFunctionRecord(l.ctx, l.svcCtx, functionID); err != nil {
		return nil, err
	}

	resp := &FunctionAnalyticsResponse{}
	if l.svcCtx == nil || l.svcCtx.ExecutionLogModel == nil {
		return resp, nil
	}

	now := time.Now().UTC()
	stats, err := l.svcCtx.ExecutionLogModel.CallStatsByFunction(
		l.ctx, functionID,
		now.Add(-24*time.Hour), now.Add(-7*24*time.Hour), now.Add(-30*24*time.Hour),
	)
	if err != nil {
		return nil, err
	}
	resp.TotalCalls = stats.Total
	resp.CallsToday = stats.Today
	resp.CallsThisWeek = stats.Week
	resp.CallsThisMonth = stats.Month
	if stats.Total > 0 {
		resp.SuccessRate = float64(stats.Ok) * 100 / float64(stats.Total)
		resp.AvgLatency = stats.AvgMs
	}
	return resp, nil
}
