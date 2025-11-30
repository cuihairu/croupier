// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsFunctionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数列表
func NewOpsFunctionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsFunctionsLogic {
	return &OpsFunctionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsFunctionsLogic) OpsFunctions(req *types.OpsFunctionsRequest) (*types.OpsFunctionsResponse, error) {
	opts := model.ListFunctionsOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     1,
			PageSize: 200,
		},
	}
	items, _, err := l.svcCtx.FunctionModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	functions := make([]map[string]interface{}, 0, len(items))
	for i := range items {
		fn := items[i]
		functions = append(functions, map[string]interface{}{
			"id":        fn.FunctionID,
			"name":      fn.Name,
			"category":  fn.Category,
			"game_id":   fn.GameID,
			"status":    fn.Status,
			"updatedAt": utils.FormatTimestamp(fn.UpdatedAt),
		})
	}

	return &types.OpsFunctionsResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"functions": functions,
		},
	}, nil
}
