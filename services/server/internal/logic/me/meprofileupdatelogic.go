// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package me

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MeProfileUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新我的资料
func NewMeProfileUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MeProfileUpdateLogic {
	return &MeProfileUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MeProfileUpdateLogic) MeProfileUpdate(req *types.MeProfileUpdateRequest) (resp *types.MeProfileUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
