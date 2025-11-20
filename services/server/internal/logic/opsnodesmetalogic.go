package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNodeMetaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsNodeMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeMetaLogic {
	return &OpsNodeMetaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeMetaLogic) OpsNodeMeta(req *types.OpsNodeMetaRequest) (*types.GenericOkResponse, error) {
	if req == nil || strings.TrimSpace(req.Id) == "" {
		return nil, ErrInvalidRequest
	}
	if strings.EqualFold(strings.TrimSpace(req.Type), "edge") {
		l.svcCtx.RecordEdgeNode(req.Id, req.Addr, req.HttpAddr, req.Version, req.Region, req.Zone)
	}
	return &types.GenericOkResponse{Ok: true}, nil
}
