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
	if req.Schema == nil {
		return nil, errorx.NewBadRequest("empty ui payload: schema is required")
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

	if req.Schema != nil {
		if isClearCustomUISchema(req.Schema) {
			// 保留显式清理标记，解决 interface{} 无法区分 null 与未传字段的问题。
			delete(meta, "ui")
		} else {
			if err := validateFormilySchema(req.Schema); err != nil {
				return nil, errorx.NewBadRequest("invalid function ui schema: " + err.Error())
			}
			meta["ui"] = req.Schema
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
	if err := validateFormilySchema(resolved.Schema); err != nil {
		return nil, errorx.NewBadRequest("invalid function ui schema: " + err.Error())
	}

	return &FunctionUIResponse{
		Schema:         resolved.Schema,
		Custom:         resolved.Custom,
		HasDefault:     resolved.HasDefault,
		UISource:       resolved.UISource,
		UISourceDetail: resolved.UISourceDetail,
	}, nil
}
