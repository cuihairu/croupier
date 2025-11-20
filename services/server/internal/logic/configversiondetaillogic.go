package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigVersionDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConfigVersionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigVersionDetailLogic {
	return &ConfigVersionDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigVersionDetailLogic) ConfigVersionDetail(req *types.ConfigVersionDetailRequest) (*types.ConfigVersionDetailResponse, error) {
	if req == nil {
		return nil, ErrInvalidRequest
	}
	ver, err := l.svcCtx.ConfigVersionDetail(req.Id, req.GameId, req.Env, req.Ver)
	if err != nil {
		return nil, err
	}
	return &types.ConfigVersionDetailResponse{
		Version: ver.Version,
		Content: ver.Content,
	}, nil
}
