// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package profile

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"golang.org/x/crypto/bcrypt"

)

type ProfilePasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 修改密码
func NewProfilePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProfilePasswordLogic {
	return &ProfilePasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProfilePasswordLogic) ProfilePassword(req *types.ProfilePasswordRequest) error {
	admin, _, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return err
	}

	oldPassword, err := utils.ValidatePassword(req.OldPassword)
	if err != nil {
		return errorx.NewBadRequest("原密码无效: " + err.Error())
	}

	newPassword, err := utils.ValidatePassword(req.NewPassword)
	if err != nil {
		return errorx.NewBadRequest("新密码无效: " + err.Error())
	}

	if oldPassword == newPassword {
		return errors.New("新密码不能与原密码相同")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.New("原密码不正确")
	}

	if err := l.svcCtx.AdminModel.UpdatePassword(l.ctx, admin.ID, newPassword); err != nil {
		return errorx.NewInternalError("更新密码失败")
	}

	l.svcCtx.InvalidateAdminCache(l.ctx, admin.ID, admin.Username)

	return nil
}
