// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EntityValidateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEntityValidateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EntityValidateLogic {
	return &EntityValidateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EntityValidateLogic) EntityValidate(req *types.EntityValidateRequest) (resp *types.EntityValidateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
