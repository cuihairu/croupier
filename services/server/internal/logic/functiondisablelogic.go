package logic

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionDisableLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionDisableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionDisableLogic {
	return &FunctionDisableLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionDisableLogic) FunctionDisable(req *types.FunctionActionRequest) (*types.FunctionActionResponse, error) {
	if req == nil || req.Id == "" {
		return nil, errors.New("function id required")
	}
	return &types.FunctionActionResponse{
		Ok:         true,
		Message:    "Function disable request processed",
		FunctionId: req.Id,
	}, nil
}
