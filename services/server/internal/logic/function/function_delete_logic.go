// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type FunctionDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除函数
func NewFunctionDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionDeleteLogic {
	return &FunctionDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionDeleteLogic) FunctionDelete(req *types.FunctionActionRequest) error {
	// 1. Validate request
	if req.ID == "" {
		return errors.New("function id is required")
	}

	// 2. Call model layer to delete function
	err := l.svcCtx.FunctionModel.DeleteFunction(l.ctx, req.ID)
	if err != nil {
		return err
	}

	// 3. Return success (no error)
	return nil
}
