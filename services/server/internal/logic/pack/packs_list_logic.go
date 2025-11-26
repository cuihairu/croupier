// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pack

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PacksListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取功能包列表
func NewPacksListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PacksListLogic {
	return &PacksListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PacksListLogic) PacksList(req *types.PacksListRequest) (resp *types.PacksListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
