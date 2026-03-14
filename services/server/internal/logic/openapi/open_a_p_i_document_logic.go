// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package openapi

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpenAPIDocumentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 导出聚合 OpenAPI 文档
func NewOpenAPIDocumentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenAPIDocumentLogic {
	return &OpenAPIDocumentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenAPIDocumentLogic) OpenAPIDocument(req *types.OpenAPIDocumentRequest) (resp *types.OpenAPIDocumentResponse, err error) {
	spec, err := l.svcCtx.RegistryStore.BuildOpenAPISpec()
	if err != nil {
		return nil, err
	}
	// Convert spec to map[string]interface{} for JSON response
	return &types.OpenAPIDocumentResponse{
		Spec: spec,
	}, nil
}
