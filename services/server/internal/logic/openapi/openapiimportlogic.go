// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package openapi

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenAPIImportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 导入 OpenAPI spec
func NewOpenAPIImportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenAPIImportLogic {
	return &OpenAPIImportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenAPIImportLogic) OpenAPIImport(req *types.OpenAPIImportRequest) (resp *types.OpenAPIImportResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
