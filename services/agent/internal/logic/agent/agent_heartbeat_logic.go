// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AgentHeartbeatLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentHeartbeatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentHeartbeatLogic {
	return &AgentHeartbeatLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentHeartbeatLogic) AgentHeartbeat(req *types.AgentHeartbeatRequest) (resp *types.AgentHeartbeatResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
