// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type FunctionCopyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 复制函数
func NewFunctionCopyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionCopyLogic {
	return &FunctionCopyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionCopyLogic) FunctionCopy(req *types.FunctionCopyRequest) (*types.FunctionCopyResponse, error) {
	// 1. Validate request
	if req.ID == "" {
		return nil, errors.New("function id is required")
	}

	// 2. Call model layer to copy function
	newId, err := l.svcCtx.FunctionModel.CopyFunction(l.ctx, req.ID)
	if err != nil {
		return nil, err
	}

	// 3. Return result
	return &types.FunctionCopyResponse{
		FunctionId: req.ID,
		NewId:      newId,
	}, nil
}
