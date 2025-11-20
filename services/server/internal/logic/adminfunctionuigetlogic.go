package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminFunctionUIGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminFunctionUIGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminFunctionUIGetLogic {
	return &AdminFunctionUIGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminFunctionUIGetLogic) AdminFunctionUIGet(req *types.AdminFunctionRequest) (*types.AdminFunctionUIResponse, error) {
	if req == nil || strings.TrimSpace(req.Fid) == "" {
		return nil, errors.New("function id required")
	}
	overrides := l.svcCtx.LoadUIOverrides()
	if v, ok := overrides[req.Fid]; ok {
		return &types.AdminFunctionUIResponse{
			FunctionId: req.Fid,
			UI:         v,
		}, nil
	}
	return &types.AdminFunctionUIResponse{FunctionId: req.Fid}, nil
}
