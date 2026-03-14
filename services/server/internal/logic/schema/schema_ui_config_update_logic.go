// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package schema

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type SchemaUiConfigUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新模式UI配置
func NewSchemaUiConfigUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaUiConfigUpdateLogic {
	return &SchemaUiConfigUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaUiConfigUpdateLogic) SchemaUiConfigUpdate(req *types.SchemaUIConfigUpdateRequest) (*types.SchemaUIConfigUpdateResponse, error) {
	doc, err := loadSchema(l.svcCtx.Config, req.ID)
	if err != nil {
		return nil, err
	}

	doc.UIConfig = req.Config
	doc.UpdatedAt = time.Now()

	if err := saveSchema(l.svcCtx.Config, doc); err != nil {
		return nil, err
	}

	return &types.SchemaUIConfigUpdateResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"id":       doc.ID,
			"uiConfig": doc.UIConfig,
		},
	}, nil
}
