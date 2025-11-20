package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminFunctionPermissionsGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminFunctionPermissionsGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminFunctionPermissionsGetLogic {
	return &AdminFunctionPermissionsGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminFunctionPermissionsGetLogic) AdminFunctionPermissionsGet(req *types.AdminFunctionRequest) (*types.AdminFunctionPermissionsResponse, error) {
	if req == nil || strings.TrimSpace(req.Fid) == "" {
		return nil, errors.New("function id required")
	}
	overrides := l.svcCtx.LoadUIOverrides()
	if entry, ok := overrides[req.Fid]; ok {
		if perm, ok := entry["permissions"]; ok {
			return &types.AdminFunctionPermissionsResponse{
				FunctionId:  req.Fid,
				Permissions: perm,
			}, nil
		}
	}
	if l.svcCtx.RegistryStore != nil {
		meta := l.svcCtx.RegistryStore.BuildFunctionIndex()
		if data, ok := meta[req.Fid]; ok {
			if perm, ok := data["permissions"]; ok {
				return &types.AdminFunctionPermissionsResponse{
					FunctionId:  req.Fid,
					Permissions: perm,
				}, nil
			}
		}
	}
	return &types.AdminFunctionPermissionsResponse{FunctionId: req.Fid}, nil
}
