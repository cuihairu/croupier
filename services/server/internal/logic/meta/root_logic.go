// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package meta

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RootLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 根路径 - API 信息和版本
func NewRootLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RootLogic {
	return &RootLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RootLogic) Root(req *types.RootRequest) (resp *types.RootResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
