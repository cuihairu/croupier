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
	if req.Schema != nil {
		updates["schema"] = req.Schema
		fn.Schema = datatypes.JSONMap{}
		if schemaMap, ok := req.Schema.(map[string]interface{}); ok {
			fn.Schema = datatypes.JSONMap(schemaMap)
		}
	}

	if req.Layout != nil || req.Components != nil {
		meta := fn.Metadata
		if meta == nil {
			meta = datatypes.JSONMap{}
		}
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

	var layout interface{}
	var components interface{}
	if fn.Metadata != nil {
		layout = fn.Metadata["layout"]
		components = fn.Metadata["components"]
	}

	return &types.FunctionUIResponse{
		Schema:     fn.Schema,
		Layout:     layout,
		Components: components,
	}, nil
}
