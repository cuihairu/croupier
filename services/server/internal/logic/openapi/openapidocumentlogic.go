// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package openapi

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenAPIDocumentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 导出聚合 OpenAPI 文档
func NewOpenAPIDocumentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenAPIDocumentLogic {
	return &OpenAPIDocumentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenAPIDocumentLogic) OpenAPIDocument(req *types.OpenAPIDocumentRequest) (resp *types.OpenAPIDocumentResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
