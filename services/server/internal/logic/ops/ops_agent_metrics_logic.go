// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsAgentMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Agent 指标
func NewOpsAgentMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentMetricsLogic {
	return &OpsAgentMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentMetricsLogic) OpsAgentMetrics(req *types.OpsAgentMetricsRequest) (resp *types.OpsAgentMetricsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
