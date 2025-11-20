package logic

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigVersionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConfigVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigVersionsLogic {
	return &ConfigVersionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigVersionsLogic) ConfigVersions(req *types.ConfigVersionsRequest) (*types.ConfigVersionsResponse, error) {
	if req == nil {
		return nil, ErrInvalidRequest
	}
	versions, err := l.svcCtx.ConfigVersions(req.Id, req.GameId, req.Env)
	if err != nil {
		return nil, err
	}
	items := make([]types.ConfigVersionItem, 0, len(versions))
	for _, ver := range versions {
		items = append(items, types.ConfigVersionItem{
			Version:   ver.Version,
			Message:   ver.Message,
			Editor:    ver.Editor,
			CreatedAt: ver.CreatedAt.Format(time.RFC3339),
			Size:      ver.Size,
			Etag:      ver.ETag,
		})
	}
	return &types.ConfigVersionsResponse{Versions: items}, nil
}
