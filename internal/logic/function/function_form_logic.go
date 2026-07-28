package function

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionFormLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数表单配置。
func NewFunctionFormLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionFormLogic {
	return &FunctionFormLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionFormLogic) FunctionForm(req *FunctionFormRequest) (*FunctionFormResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := getOrCreateFunctionRecord(l.ctx, l.svcCtx, functionID)
	if err != nil {
		return nil, err
	}
	resolved := resolveFunctionForm(l.svcCtx.Config, fn)
	schema := rawJSONFromValue(resolved.Schema)
	schemaValue, err := jsonValueFromRaw(schema)
	if err != nil {
		return nil, errorx.NewBadRequest("invalid function form schema: " + err.Error())
	}
	if schemaValue == nil {
		schemaValue = map[string]interface{}{}
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
