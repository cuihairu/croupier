// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package player

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlayerUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新玩家信息
func NewPlayerUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlayerUpdateLogic {
	return &PlayerUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PlayerUpdateLogic) PlayerUpdate(req *types.PlayerUpdateRequest) (resp *types.PlayerUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
