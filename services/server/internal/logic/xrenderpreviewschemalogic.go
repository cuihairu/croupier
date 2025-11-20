package logic

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type XRenderPreviewSchemaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewXRenderPreviewSchemaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *XRenderPreviewSchemaLogic {
	return &XRenderPreviewSchemaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *XRenderPreviewSchemaLogic) XRenderPreviewSchema(req *types.XRenderPreviewRequest) (*types.XRenderPreviewResponse, error) {
	if req == nil || len(req.Components) == 0 {
		return nil, errors.New("components required")
	}
	schema := map[string]interface{}{
		"$schema":     "http://json-schema.org/draft-07/schema#",
		"type":        "object",
		"title":       "Preview Schema",
		"description": "Preview from x-render components",
		"properties":  map[string]interface{}{},
		"required":    []string{},
	}
	uiSchema := map[string]interface{}{
		"type":        "object",
		"displayType": "column",
		"properties":  map[string]interface{}{},
	}
	generateSchemaFromComponents(schema, uiSchema, req.Components)
	return &types.XRenderPreviewResponse{
		Schema:   schema,
		UISchema: uiSchema,
	}, nil
}
