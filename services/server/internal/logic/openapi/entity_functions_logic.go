// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package openapi

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EntityFunctionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取实体关联的函数列表
func NewEntityFunctionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EntityFunctionsLogic {
	return &EntityFunctionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EntityFunctionsLogic) EntityFunctions(req *types.EntityFunctionsRequest) (resp *types.EntityFunctionsResponse, err error) {
	// 参数验证
	if req.ID == "" {
		return nil, fmt.Errorf("entity ID is required")
	}

	// TODO: 从 Entity Manager 获取实体关联的函数
	// 目前先返回空列表，等 EntityManager 集成完成后实现
	l.Infof("Getting functions for entity '%s'", req.ID)

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

	l.Infof("Found %d functions for entity '%s'", len(items), req.ID)

	return &types.EntityFunctionsResponse{
		Items: items,
	}, nil
}
