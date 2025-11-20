// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EntityDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEntityDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EntityDeleteLogic {
	return &EntityDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EntityDeleteLogic) EntityDelete(req *types.EntityDeleteRequest) (resp *types.EntityDeleteResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
