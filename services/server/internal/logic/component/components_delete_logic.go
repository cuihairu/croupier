// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package component

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComponentsDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除组件
func NewComponentsDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsDeleteLogic {
	return &ComponentsDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsDeleteLogic) ComponentsDelete(req *types.ComponentActionRequest) (resp *types.ComponentsDeleteResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
