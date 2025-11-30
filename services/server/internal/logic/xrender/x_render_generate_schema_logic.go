// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package xrender

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type XRenderGenerateSchemaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 生成XRender模式
func NewXRenderGenerateSchemaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *XRenderGenerateSchemaLogic {
	return &XRenderGenerateSchemaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *XRenderGenerateSchemaLogic) XRenderGenerateSchema(req *types.XRenderGenerateRequest) (*types.XRenderGenerateSchemaResponse, error) {
	schema, err := normalizeSchemaInput(req.Schema)
	if err != nil {
		return nil, err
	}
	schema = ensureObjectSchema(schema)

	uiSchema := buildDefaultUISchema(schema)
	fields := extractSchemaFields(schema)
	sample := generateSampleData(schema)

	return &types.XRenderGenerateSchemaResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"schema":      schema,
			"uiSchema":    uiSchema,
			"fields":      fields,
			"sample":      sample,
			"generatedAt": utils.FormatTimestamp(time.Now()),
		},
	}, nil
}
