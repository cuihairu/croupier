// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package schema

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SchemasListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取模式列表
func NewSchemasListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemasListLogic {
	return &SchemasListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemasListLogic) SchemasList(req *types.SchemasListRequest) (*types.SchemasListResponse, error) {
	docs, err := listSchemas(l.svcCtx.Config)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		items = append(items, doc.toMap())
	}

	return &types.SchemasListResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": items,
			"total": len(items),
		},
	}, nil
}
