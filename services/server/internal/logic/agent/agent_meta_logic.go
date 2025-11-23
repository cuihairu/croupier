package agent

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AgentMetaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentMetaLogic {
	return &AgentMetaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentMetaLogic) AgentMeta(req *types.AgentMetaReportRequest) (resp *types.AgentMetaResponse, err error) {
	// TODO: add your logic here and delete this line

	return
}
