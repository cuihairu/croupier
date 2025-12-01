// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package profile

import (
	"context"
	"errors"
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"golang.org/x/crypto/bcrypt"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProfilePasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 修改密码
func NewProfilePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProfilePasswordLogic {
	return &ProfilePasswordLogic{
		Logger: logx.WithContext(ctx),
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
		return fmt.Errorf("原密码无效: %w", err)
	}

	newPassword, err := utils.ValidatePassword(req.NewPassword)
	if err != nil {
		return fmt.Errorf("新密码无效: %w", err)
	}

	if oldPassword == newPassword {
		return errors.New("新密码不能与原密码相同")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.New("原密码不正确")
	}

	if err := l.svcCtx.AdminModel.UpdatePassword(l.ctx, admin.ID, newPassword); err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}

	l.svcCtx.InvalidateAdminCache(l.ctx, admin.ID, admin.Username)

	return nil
}
