// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package xrender

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type XRenderPreviewSchemaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 预览XRender模式
func NewXRenderPreviewSchemaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *XRenderPreviewSchemaLogic {
	return &XRenderPreviewSchemaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *XRenderPreviewSchemaLogic) XRenderPreviewSchema(req *types.XRenderPreviewRequest) (*types.XRenderPreviewSchemaResponse, error) {
	schema, err := normalizeSchemaInput(req.Schema)
	if err != nil {
		return nil, err
	}
	schema = ensureObjectSchema(schema)

	uiSchema := buildDefaultUISchema(schema)
	fields := extractSchemaFields(schema)
	sample := generateSampleData(schema)

	return &types.XRenderPreviewSchemaResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"schema":   schema,
			"uiSchema": uiSchema,
			"fields":   fields,
			"layout":   uiSchema["ui:layout"],
			"order":    uiSchema["ui:order"],
			"sample":   sample,
		},
	}, nil
}
