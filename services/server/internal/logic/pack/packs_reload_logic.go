// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pack

import (
	"context"

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

func (l *PacksReloadLogic) PacksReload(req *types.PacksReloadRequest) (resp *types.PacksReloadResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
