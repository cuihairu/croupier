// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CallPlatformLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 调用第三方平台 API
func NewCallPlatformLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CallPlatformLogic {
	return &CallPlatformLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CallPlatformLogic) CallPlatform(req *types.CallPlatformRequest) (resp *types.CallPlatformResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
