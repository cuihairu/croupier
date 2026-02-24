// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionUIUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新函数UI配置
func NewFunctionUIUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionUIUpdateLogic {
	return &FunctionUIUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionUIUpdateLogic) FunctionUIUpdate(req *types.FunctionUIUpdateRequest) (*types.FunctionUIResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})
	meta := fn.Metadata
	if meta == nil {
		meta = datatypes.JSONMap{}
	}

	// 更新 Schema（支持三种来源）
	if req.Schema != nil {
		if m, ok := req.Schema.(map[string]interface{}); ok {
			// 保留一个显式清理标记，解决 interface{} 无法区分 null 与未传字段的问题。
			if clearFlag, ok := m["__clear_custom_ui"].(bool); ok && clearFlag {
				delete(meta, "ui")
			} else {
				meta["ui"] = req.Schema
			}
		} else {
			// 存储到 Metadata["ui"] 作为自定义 UI
			meta["ui"] = req.Schema
		}
		updates["metadata"] = meta
		fn.Metadata = meta
	}

	// 更新 Layout 和 Components
	if req.Layout != nil || req.Components != nil {
		if req.Layout != nil {
			meta["layout"] = req.Layout
		}
		if req.Components != nil {
			meta["components"] = req.Components
		}
		updates["metadata"] = meta
		fn.Metadata = meta
	}

	if len(updates) > 0 {
		if err := l.svcCtx.FunctionModel.Update(l.ctx, fn.ID, updates); err != nil {
			return nil, err
		}
	}

	// 读取并返回合并后的 UI
	var customUI, defaultUI, legacyUI interface{}
	customUI = fn.Metadata["ui"]

	if fn.OpenAPISpec != nil {
		defaultUI = fn.OpenAPISpec["x-ui"]
	}
	legacyUI = fn.Schema
	hasCustom := customUI != nil
	hasDefault := defaultUI != nil || legacyUI != nil

	resultUI := customUI
	if resultUI == nil {
		resultUI = defaultUI
	}
	if resultUI == nil {
		resultUI = legacyUI
	}

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
