// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package openapi

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type FunctionOpenAPISpecLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数的 OpenAPI spec
func NewFunctionOpenAPISpecLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionOpenAPISpecLogic {
	return &FunctionOpenAPISpecLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionOpenAPISpecLogic) FunctionOpenAPISpec(req *types.OpenAPISpecRequest) (resp *types.OpenAPISpecResponse, err error) {
	spec, err := l.svcCtx.RegistryStore.GetOpenAPI(req.ID)
	if err != nil {
		return nil, err
	}
	return &types.OpenAPISpecResponse{
		Spec: spec,
	}, nil
}
