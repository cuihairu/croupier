// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package component

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComponentsDisableLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 禁用组件
func NewComponentsDisableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsDisableLogic {
	return &ComponentsDisableLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsDisableLogic) ComponentsDisable(req *types.ComponentActionRequest) (resp *types.ComponentsDisableResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
