// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionUILogicV2 struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数UI配置（增强版：支持优先级合并）
func NewFunctionUILogicV2(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionUILogicV2 {
	return &FunctionUILogicV2{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionUILogicV2) FunctionUI(req *types.FunctionUIRequest) (*types.FunctionUIResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID)
	if err != nil {
		return nil, err
	}

	// ========== 优先级合并逻辑 ==========
	// 优先级 1: 用户自定义的 UI（Metadata["ui"]）
	var customUI, defaultUI, legacyUI interface{}
	if fn.Metadata != nil {
		customUI = fn.Metadata["ui"]
	}

	// 优先级 2: OpenAPI Spec 中的 x-ui 扩展
	if fn.OpenAPISpec != nil {
		defaultUI = fn.OpenAPISpec["x-ui"]
	}

	// 优先级 3: 旧格式的 Schema（兼容性）
	legacyUI = fn.Schema
	hasCustom := customUI != nil
	hasDefault := defaultUI != nil || legacyUI != nil

	// 合并 UI（自定义覆盖默认）
	resultUI := customUI
	if resultUI == nil {
		resultUI = defaultUI
	}
	if resultUI == nil {
		resultUI = legacyUI
	}

	// Layout 和 Components 仍从 Metadata 读取
	var layout interface{}
	var components interface{}
	if fn.Metadata != nil {
		layout = fn.Metadata["layout"]
		components = fn.Metadata["components"]
	}

	return &types.FunctionUIResponse{
		Schema:     resultUI,
		Layout:     layout,
		Components: components,
		Custom:     hasCustom,
		HasDefault: hasDefault,
	}, nil
}
