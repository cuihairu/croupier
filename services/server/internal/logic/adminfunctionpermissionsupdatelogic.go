package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminFunctionPermissionsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminFunctionPermissionsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminFunctionPermissionsUpdateLogic {
	return &AdminFunctionPermissionsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminFunctionPermissionsUpdateLogic) AdminFunctionPermissionsUpdate(req *types.AdminFunctionPermissionsUpdateRequest) (*types.AdminFunctionPermissionsUpdateResponse, error) {
	if req == nil || strings.TrimSpace(req.Fid) == "" {
		return nil, errors.New("function id required")
	}
	if req.Permissions == nil {
		return nil, errors.New("permissions payload required")
	}
	entry := map[string]any{}
	if cur := l.svcCtx.LoadUIOverrides(); cur != nil {
		if ex, ok := cur[req.Fid]; ok && ex != nil {
			for k, v := range ex {
				entry[k] = v
			}
		}
	}
	entry["permissions"] = req.Permissions
	if err := l.svcCtx.SaveUIOverride(req.Fid, entry); err != nil {
		return nil, err
	}
	return &types.AdminFunctionPermissionsUpdateResponse{Ok: true}, nil
}
