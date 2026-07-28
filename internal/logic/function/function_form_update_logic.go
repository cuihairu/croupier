package function

import (
	"bytes"
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionFormUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新函数表单配置
func NewFunctionFormUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionFormUpdateLogic {
	return &FunctionFormUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionFormUpdateLogic) FunctionFormUpdate(req *FunctionFormUpdateRequest) (*FunctionFormResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}
	if len(req.Schema) == 0 {
		return nil, errorx.NewBadRequest("empty form payload: schema is required")
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
			delete(meta, "form")
			delete(meta, "ui")
		} else {
			schemaValue, err := jsonValueFromRaw(req.Schema)
			if err != nil {
				return nil, errorx.NewBadRequest("invalid function form schema: " + err.Error())
			}
			if err := validateFormilySchema(schemaValue); err != nil {
				return nil, errorx.NewBadRequest("invalid function form schema: " + err.Error())
			}
			meta["form"] = schemaValue
			delete(meta, "ui")
		}
		delete(meta, "layout")
		delete(meta, "components")
		updates["metadata"] = meta
		fn.Metadata = meta
	}

	if len(updates) > 0 {
		if err := l.svcCtx.FunctionModel.Update(l.ctx, fn.ID, updates); err != nil {
			return nil, err
		}
		if err := persistFunctionFormVersion(l.ctx, l.svcCtx, fn, "update form config"); err != nil {
			return nil, err
		}
	}

	resolved := resolveFunctionForm(l.svcCtx.Config, fn)
	schema := rawJSONFromValue(resolved.Schema)
	schemaValue, err := jsonValueFromRaw(schema)
	if err != nil {
		return nil, errorx.NewBadRequest("invalid function form schema: " + err.Error())
	}
	if err := validateFormilySchema(schemaValue); err != nil {
		return nil, errorx.NewBadRequest("invalid function form schema: " + err.Error())
	}

	return &FunctionFormResponse{
		Schema:           schema,
		Custom:           resolved.Custom,
		HasDefault:       resolved.HasDefault,
		FormSource:       resolved.FormSource,
		FormSourceDetail: resolved.FormSourceDetail,
	}, nil
}
