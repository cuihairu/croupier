package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsAgentMetaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsAgentMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentMetaLogic {
	return &OpsAgentMetaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentMetaLogic) OpsAgentMeta(req *types.OpsAgentMetaUpdateRequest) (*types.GenericOkResponse, error) {
	if req == nil || strings.TrimSpace(req.Id) == "" {
		return nil, errors.New("agent id required")
	}
	ok := l.svcCtx.UpdateAgentMeta(req.Id, req.Region, req.Zone, req.Labels)
	if !ok {
		return nil, ErrAgentNotFound
	}
	return &types.GenericOkResponse{Ok: true}, nil
}
