// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package profile

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProfileGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取当前用户资料
func NewProfileGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProfileGetLogic {
	return &ProfileGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProfileGetLogic) ProfileGet(req *types.ProfileGetRequest) (resp *types.ProfileGetResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
