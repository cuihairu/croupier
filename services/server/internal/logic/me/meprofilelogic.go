// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package me

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MeProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取我的资料
func NewMeProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MeProfileLogic {
	return &MeProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MeProfileLogic) MeProfile(req *types.MeProfileRequest) (resp *types.MeProfileResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
