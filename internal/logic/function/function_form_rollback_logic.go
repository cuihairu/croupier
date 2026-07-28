package function

import (
	"context"
	"encoding/json"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionFormRollbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionFormRollbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionFormRollbackLogic {
	return &FunctionFormRollbackLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionFormRollbackLogic) FunctionFormRollback(req *FunctionFormRollbackRequest) (*FunctionFormRollbackResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}
	if req.Version <= 0 {
		return nil, errorx.NewBadRequest("version must be greater than 0")
	}
	if l == nil || l.svcCtx == nil || l.svcCtx.ConfigVersionModel == nil {
		return &FunctionFormRollbackResponse{AppliedVersion: req.Version}, nil
	}

	record, err := l.svcCtx.ConfigVersionModel.Find(l.ctx, functionFormHistoryKey(functionID), req.Version)
	if err != nil {
		return nil, err
	}

	fn, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID)
	if err != nil {
		return nil, err
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(record.Value), &cfg); err != nil {
		return nil, err
	}
	if schema, ok := cfg["schema"]; ok && schema != nil {
		if err := validateFormilySchema(schema); err != nil {
			return nil, errorx.NewBadRequest("invalid function form schema: " + err.Error())
		}
	}

	meta := applyFormCustomConfig(fn.Metadata, cfg)
	if err := l.svcCtx.FunctionModel.Update(l.ctx, fn.ID, map[string]interface{}{"metadata": meta}); err != nil {
		return nil, err
	}
	fn.Metadata = meta
	if err := persistFunctionFormVersion(l.ctx, l.svcCtx, fn, "rollback to version"); err != nil {
		return nil, err
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
	return &FunctionFormRollbackResponse{
		AppliedVersion: req.Version,
		Current: &FunctionFormResponse{
			Schema:           schema,
			Custom:           resolved.Custom,
			HasDefault:       resolved.HasDefault,
			FormSource:       resolved.FormSource,
			FormSourceDetail: resolved.FormSourceDetail,
		},
	}, nil
}
