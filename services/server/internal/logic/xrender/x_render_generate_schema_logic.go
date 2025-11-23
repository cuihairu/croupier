// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package xrender

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type XRenderGenerateSchemaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 生成XRender模式
func NewXRenderGenerateSchemaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *XRenderGenerateSchemaLogic {
	return &XRenderGenerateSchemaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *XRenderGenerateSchemaLogic) XRenderGenerateSchema(req *types.XRenderGenerateRequest) (resp *types.XRenderGenerateSchemaResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
