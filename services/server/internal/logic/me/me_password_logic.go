// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package me

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MePasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 修改我的密码
func NewMePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MePasswordLogic {
	return &MePasswordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MePasswordLogic) MePassword(req *types.MePasswordRequest) (resp *types.MePasswordResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
