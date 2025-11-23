// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AgentRegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentRegisterLogic {
	return &AgentRegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentRegisterLogic) AgentRegister(req *types.AgentRegisterRequest) (resp *types.AgentRegisterResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
