// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package component

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComponentsPatchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新组件配置
func NewComponentsPatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsPatchLogic {
	return &ComponentsPatchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsPatchLogic) ComponentsPatch(req *types.ComponentPatchRequest) (resp *types.ComponentsPatchResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
