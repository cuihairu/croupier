// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AgentMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentMetricsLogic {
	return &AgentMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentMetricsLogic) AgentMetrics(req *types.AgentMetricsRequest) (resp *types.AgentMetricsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
