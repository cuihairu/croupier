// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package profile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProfileUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新当前用户资料
func NewProfileUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProfileUpdateLogic {
	return &ProfileUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProfileUpdateLogic) ProfileUpdate(req *types.ProfileUpdateRequest) (resp *types.ProfileGetResponse, err error) {
	admin, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if v := strings.TrimSpace(req.Nickname); v != "" {
		updates["nickname"] = v
	}
	if v := strings.TrimSpace(req.Email); v != "" {
		updates["email"] = v
	}
	if v := strings.TrimSpace(req.Phone); v != "" {
		updates["phone"] = v
	}
	if v := strings.TrimSpace(req.Avatar); v != "" {
		updates["avatar"] = v
	}

	if len(updates) == 0 {
		return nil, errors.New("请提供需要更新的字段")
	}

	if err := l.svcCtx.AdminModel.Update(l.ctx, admin.ID, updates); err != nil {
		return nil, fmt.Errorf("更新个人资料失败: %w", err)
	}

	l.svcCtx.InvalidateAdminCache(l.ctx, admin.ID, admin.Username)

	updated, err := l.svcCtx.GetAdminCached(l.ctx, admin.ID)
	if err != nil {
		return nil, fmt.Errorf("查询更新后的资料失败: %w", err)
	}

	return &types.ProfileGetResponse{
		ProfileInfo: types.ProfileInfo{
			Id:        int64(updated.ID),
			Username:  updated.Username,
			Nickname:  updated.Nickname,
			Email:     updated.Email,
			Phone:     updated.Phone,
			Roles:     utils.RoleNamesFromModels(roles),
			Avatar:    updated.Avatar,
			CreatedAt: utils.FormatTimestamp(updated.CreatedAt),
			UpdatedAt: utils.FormatTimestamp(updated.UpdatedAt),
		},
	}, nil
}
