// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AgentHealthLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentHealthLogic {
	return &AgentHealthLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentHealthLogic) AgentHealth(req *types.AgentHealthRequest) (resp *types.AgentHealthResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
