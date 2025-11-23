// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionRegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionRegisterLogic {
	return &FunctionRegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionRegisterLogic) FunctionRegister(req *types.FunctionRegisterRequest) (resp *types.FunctionRegisterResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
