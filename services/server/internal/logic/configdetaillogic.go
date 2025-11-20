package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConfigDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigDetailLogic {
	return &ConfigDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigDetailLogic) ConfigDetail(req *types.ConfigDetailRequest) (*types.ConfigDetailResponse, error) {
	if req == nil {
		return nil, ErrInvalidRequest
	}
	entry, latest, err := l.svcCtx.ConfigDetail(req.Id, req.GameId, req.Env)
	if err != nil {
		return nil, err
	}
	content := ""
	version := entry.Latest
	if latest != nil {
		content = latest.Content
		version = latest.Version
	}
	return &types.ConfigDetailResponse{
		Id:      entry.ID,
		GameId:  entry.GameID,
		Env:     entry.Env,
		Format:  entry.Format,
		Version: version,
		Content: content,
	}, nil
}
