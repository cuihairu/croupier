
package function

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionUIUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新函数UI配置
func NewFunctionUIUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionUIUpdateLogic {
	return &FunctionUIUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionUIUpdateLogic) FunctionUIUpdate(req *FunctionUIUpdateRequest) (*FunctionUIResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}
	if req.Schema == nil && req.Layout == nil && req.Components == nil {
		return nil, errorx.NewBadRequest("empty ui payload: schema/layout/components are all missing")
	}

	fn, err := getOrCreateFunctionRecord(l.ctx, l.svcCtx, functionID)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})
	meta := fn.Metadata
	if meta == nil {
		meta = map[string]interface{}{}
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
		if err := persistFunctionUIVersion(l.ctx, l.svcCtx, fn, "update ui config"); err != nil {
			return nil, err
		}
	}

	resolved := resolveFunctionUI(l.svcCtx.Config, fn)

	return &FunctionUIResponse{
		Schema:         resolved.Schema,
		Layout:         resolved.Layout,
		Components:     resolved.Components,
		Custom:         resolved.Custom,
		HasDefault:     resolved.HasDefault,
		UISource:       resolved.UISource,
		UISourceDetail: resolved.UISourceDetail,
	}, nil
}
