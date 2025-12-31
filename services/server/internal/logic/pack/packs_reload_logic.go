// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pack

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PacksReloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重新加载功能包
func NewPacksReloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PacksReloadLogic {
	return &PacksReloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PacksReloadLogic) PacksReload(_ *types.PacksReloadRequest) (*types.PacksReloadResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权重新加载功能包", "admin:all", "packs:reload"); err != nil {
		return nil, err
	}

	packsDir := resolvePacksDir(l.svcCtx.Config)
	_, err := loadPackSummaries(packsDir)
	if err != nil {
		return nil, err
	}

	return &types.PacksReloadResponse{
		OK:        true,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}
