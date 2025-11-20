package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminFunctionUIUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminFunctionUIUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminFunctionUIUpdateLogic {
	return &AdminFunctionUIUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminFunctionUIUpdateLogic) AdminFunctionUIUpdate(req *types.AdminFunctionUIUpdateRequest) (*types.AdminFunctionUIUpdateResponse, error) {
	if req == nil || strings.TrimSpace(req.Fid) == "" {
		return nil, errors.New("function id required")
	}
	if req.UI == nil {
		return nil, errors.New("override payload required")
	}
	if err := l.svcCtx.SaveUIOverride(req.Fid, req.UI); err != nil {
		return nil, err
	}
	return &types.AdminFunctionUIUpdateResponse{Ok: true}, nil
}
