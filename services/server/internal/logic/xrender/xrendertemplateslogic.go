// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package xrender

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type XRenderTemplatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取XRender模板
func NewXRenderTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *XRenderTemplatesLogic {
	return &XRenderTemplatesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *XRenderTemplatesLogic) XRenderTemplates(req *types.XRenderTemplatesRequest) (resp *types.XRenderTemplatesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
