// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EntityCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEntityCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EntityCreateLogic {
	return &EntityCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EntityCreateLogic) EntityCreate(req *types.EntityCreateRequest) (resp *types.EntityCreateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
