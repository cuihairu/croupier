// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package xrender_schema

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/logic/xrender"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UiSchemaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取UI模式
func NewUiSchemaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UiSchemaLogic {
	return &UiSchemaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UiSchemaLogic) UiSchema(req *types.UISchemaRequest) (resp *types.UISchemaResponse, err error) {
	if req == nil || strings.TrimSpace(req.Type) == "" {
		return nil, errors.New("type 参数不能为空")
	}
	schemaType := strings.TrimSpace(req.Type)

	if doc, err := loadCustomSchema(l.svcCtx.Config, schemaType); err == nil {
		return &types.UISchemaResponse{
			Code:    0,
			Message: "OK",
			Data: map[string]interface{}{
				"id":        doc.ID,
				"name":      doc.Name,
				"schema":    doc.Schema,
				"uiSchema":  doc.UIConfig,
				"source":    "custom",
				"createdAt": utils.FormatTimestamp(doc.CreatedAt),
				"updatedAt": utils.FormatTimestamp(doc.UpdatedAt),
			},
		}, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	component, err := xrender.FindComponentDefinition(l.svcCtx.Config, schemaType)
	if err != nil {
		return nil, fmt.Errorf("未找到类型 %s 的UI模式: %w", schemaType, err)
	}

	componentSchema, err := xrender.NormalizeSchemaInput(component.Schema)
	if err != nil {
		return nil, err
	}
	componentSchema = xrender.EnsureObjectSchema(componentSchema)

	uiSchema := component.UIConfig
	if uiSchema == nil {
		uiSchema = xrender.BuildDefaultUISchema(componentSchema)
	}

	return &types.UISchemaResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"id":           component.ID,
			"schema":       componentSchema,
			"uiSchema":     uiSchema,
			"pack":         component.Pack,
			"source":       "component",
			"schemaFile":   component.SchemaFile,
			"uiSchemaFile": component.UIConfigFile,
		},
	}, nil
}
