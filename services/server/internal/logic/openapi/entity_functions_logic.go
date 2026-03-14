// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package openapi

import (
	"context"
	"log/slog"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type EntityFunctionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取实体关联的函数列表
func NewEntityFunctionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EntityFunctionsLogic {
	return &EntityFunctionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EntityFunctionsLogic) EntityFunctions(req *types.EntityFunctionsRequest) (resp *types.EntityFunctionsResponse, err error) {
	// 参数验证
	if req.ID == "" {
		return nil, errorx.NewBadRequest("entity ID is required")
	}

	// TODO: 从 Entity Manager 获取实体关联的函数
	// 目前先返回空列表，等 EntityManager 集成完成后实现
	slog.InfoContext(l.ctx, "Getting functions for entity", "id", req.ID)

	// 临时实现：从 registry store 获取所有操作，然后过滤
	operations := l.svcCtx.RegistryStore.ListOpenAPIOperations()

	items := []types.EntityFunction{}
	for funcID, op := range operations {
		// 检查操作的扩展字段
		if op.Extensions != nil {
			if entityID, ok := op.Extensions["x-entity"].(string); ok {
				if entityID == req.ID {
					// 提取操作类型
					operation := "custom"
					if opType, ok := op.Extensions["x-operation"].(string); ok {
						operation = opType
					}

					items = append(items, types.EntityFunction{
						Id:        funcID,
						Operation: operation,
						Name:      op.Summary,
					})
				}
			}
		}
	}

	slog.InfoContext(l.ctx, "Found functions for entity", "id", req.ID, "count", len(items))

	return &types.EntityFunctionsResponse{
		Items: items,
	}, nil
}
