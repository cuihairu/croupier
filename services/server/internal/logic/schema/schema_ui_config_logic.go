// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package schema

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SchemaUiConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取模式UI配置
func NewSchemaUiConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaUiConfigLogic {
	return &SchemaUiConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaUiConfigLogic) SchemaUiConfig(req *types.SchemaUIConfigRequest) (*types.SchemaUIConfigResponse, error) {
	doc, err := loadSchema(l.svcCtx.Config, req.ID)
	if err != nil {
		return nil, err
	}

	return &types.SchemaUIConfigResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"id":       doc.ID,
			"uiConfig": doc.UIConfig,
		},
	}, nil
}
