// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package openapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionOpenAPISpecLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数的 OpenAPI spec
func NewFunctionOpenAPISpecLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionOpenAPISpecLogic {
	return &FunctionOpenAPISpecLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionOpenAPISpecLogic) FunctionOpenAPISpec(req *types.OpenAPISpecRequest) (resp *types.OpenAPISpecResponse, err error) {
	// 参数验证
	if req.ID == "" {
		return nil, fmt.Errorf("function ID is required")
	}

	// 从 registry store 获取 OpenAPI operation
	op, err := l.svcCtx.RegistryStore.GetOpenAPI(req.ID)
	if err != nil {
		l.Errorf("failed to get OpenAPI operation '%s': %v", req.ID, err)
		return nil, err
	}

	// 转换为 JSON 格式返回
	opJSON, err := op.MarshalJSON()
	if err != nil {
		l.Errorf("failed to marshal OpenAPI operation: %v", err)
		return nil, err
	}

	// 解析为 interface{} 用于响应
	var spec interface{}
	if err := json.Unmarshal(opJSON, &spec); err != nil {
		l.Errorf("failed to unmarshal OpenAPI operation: %v", err)
		return nil, err
	}

	return &types.OpenAPISpecResponse{
		Spec: spec,
	}, nil
}
