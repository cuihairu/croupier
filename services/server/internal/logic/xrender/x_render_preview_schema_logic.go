// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package xrender

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type XRenderPreviewSchemaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 预览XRender模式
func NewXRenderPreviewSchemaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *XRenderPreviewSchemaLogic {
	return &XRenderPreviewSchemaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *XRenderPreviewSchemaLogic) XRenderPreviewSchema(req *types.XRenderPreviewRequest) (resp *types.XRenderPreviewSchemaResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
