// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EntityUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEntityUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EntityUpdateLogic {
	return &EntityUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EntityUpdateLogic) EntityUpdate(req *types.EntityUpdateRequest) (resp *types.EntityUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
