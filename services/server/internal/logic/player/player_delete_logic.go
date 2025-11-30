// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package player

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlayerDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除玩家
func NewPlayerDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlayerDeleteLogic {
	return &PlayerDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PlayerDeleteLogic) PlayerDelete(req *types.PlayerDeleteRequest) error {
	id, err := utils.ParseUintID(req.ID, "玩家ID")
	if err != nil {
		return err
	}
	return l.svcCtx.PlayerModel.Delete(l.ctx, id)
}
