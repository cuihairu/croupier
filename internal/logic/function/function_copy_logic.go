
package function

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/internal/svc"
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

func (l *FunctionCopyLogic) FunctionCopy(req *FunctionCopyRequest) (*FunctionCopyResponse, error) {
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
	return &FunctionCopyResponse{
		FunctionId: req.ID,
		NewId:      newId,
	}, nil
}
