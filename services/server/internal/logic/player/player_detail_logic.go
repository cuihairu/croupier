// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package player

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlayerDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取玩家详情
func NewPlayerDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlayerDetailLogic {
	return &PlayerDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PlayerDetailLogic) PlayerDetail(req *types.PlayerDetailRequest) (resp *types.PlayerDetailResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
