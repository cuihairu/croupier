package logic

import (
	"context"
	"errors"
	"strings"

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

func (l *AgentMetaLogic) AgentMeta(req *types.AgentMetaReportRequest) (*types.GenericOkResponse, error) {
	if req == nil || strings.TrimSpace(req.AgentId) == "" {
		return nil, errors.New("agent_id required")
	}
	ok := l.svcCtx.UpdateAgentMeta(req.AgentId, req.Region, req.Zone, req.Labels)
	if !ok {
		return nil, ErrAgentNotFound
	}
	return &types.GenericOkResponse{Ok: true}, nil
}
