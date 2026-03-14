// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package profile

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type ProfileGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取当前用户资料
func NewProfileGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProfileGetLogic {
	return &ProfileGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProfileGetLogic) ProfileGet(req *types.ProfileGetRequest) (resp *types.ProfileGetResponse, err error) {
	admin, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}

	roleNames := utils.RoleNamesFromModels(roles)
	return &types.ProfileGetResponse{
		ProfileInfo: types.ProfileInfo{
			Id:        int64(admin.ID),
			Username:  admin.Username,
			Nickname:  admin.Nickname,
			Email:     admin.Email,
			Phone:     admin.Phone,
			Roles:     roleNames,
			Avatar:    admin.Avatar,
			CreatedAt: utils.FormatTimestamp(admin.CreatedAt),
			UpdatedAt: utils.FormatTimestamp(admin.UpdatedAt),
		},
	}, nil
}
