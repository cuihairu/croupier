package function

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionUILogicV2 struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数UI配置（增强版：支持优先级合并）
func NewFunctionUILogicV2(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionUILogicV2 {
	return &FunctionUILogicV2{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionUILogicV2) FunctionUI(req *FunctionUIRequest) (*FunctionUIResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := getOrCreateFunctionRecord(l.ctx, l.svcCtx, functionID)
	if err != nil {
		return nil, err
	}
	resolved := resolveFunctionUI(l.svcCtx.Config, fn)
	schema := rawJSONFromValue(resolved.Schema)
	schemaValue, err := jsonValueFromRaw(schema)
	if err != nil {
		return nil, errorx.NewBadRequest("invalid function ui schema: " + err.Error())
	}
	if schemaValue == nil {
		schemaValue = map[string]interface{}{}
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
