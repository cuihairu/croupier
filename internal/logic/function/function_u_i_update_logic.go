package function

import (
	"bytes"
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
	if len(req.Schema) == 0 {
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

	if len(req.Schema) > 0 {
		if bytes.Equal(bytes.TrimSpace(req.Schema), []byte("null")) {
			delete(meta, "ui")
		} else {
			schemaValue, err := jsonValueFromRaw(req.Schema)
			if err != nil {
				return nil, errorx.NewBadRequest("invalid function ui schema: " + err.Error())
			}
			if err := validateFormilySchema(schemaValue); err != nil {
				return nil, errorx.NewBadRequest("invalid function ui schema: " + err.Error())
			}
			meta["ui"] = schemaValue
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
	schema := rawJSONFromValue(resolved.Schema)
	schemaValue, err := jsonValueFromRaw(schema)
	if err != nil {
		return nil, errorx.NewBadRequest("invalid function ui schema: " + err.Error())
	}
	if err := validateFormilySchema(schemaValue); err != nil {
		return nil, errorx.NewBadRequest("invalid function ui schema: " + err.Error())
	}

	return &FunctionUIResponse{
		Schema:         schema,
		Custom:         resolved.Custom,
		HasDefault:     resolved.HasDefault,
		UISource:       resolved.UISource,
		UISourceDetail: resolved.UISourceDetail,
	}, nil
}
