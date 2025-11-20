package logic

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionEnableLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionEnableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionEnableLogic {
	return &FunctionEnableLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionEnableLogic) FunctionEnable(req *types.FunctionActionRequest) (*types.FunctionActionResponse, error) {
	if req == nil || req.Id == "" {
		return nil, errors.New("function id required")
	}
	return &types.FunctionActionResponse{
		Ok:         true,
		Message:    "Function enable request processed",
		FunctionId: req.Id,
	}, nil
}
