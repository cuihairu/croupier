package identity

import (
	"context"

	"github.com/cuihairu/croupier/internal/model"
)

// AdminValidator 是 LocalProvider 依赖的最小凭证校验接口，
// 由 *model.AdminModel 实现。定义为接口以便测试与替换。
type AdminValidator interface {
	ValidatePassword(ctx context.Context, username, password string) (*model.Admin, error)
}

// LocalProvider 是默认的本地身份提供方，基于 admins 表做 bcrypt 凭证校验。
type LocalProvider struct {
	admins AdminValidator
}

// NewLocalProvider 创建基于本地 admins 表的身份提供方。
func NewLocalProvider(admins AdminValidator) *LocalProvider {
	return &LocalProvider{admins: admins}
}

// Kind 实现 PasswordProvider。
func (p *LocalProvider) Kind() string { return KindLocal }

// Authenticate 实现 PasswordProvider：校验本地账号密码并映射为 Identity。
func (p *LocalProvider) Authenticate(ctx context.Context, username, password string) (*Identity, error) {
	admin, err := p.admins.ValidatePassword(ctx, username, password)
	if err != nil {
		// 统一映射为 ErrInvalidCredentials，不向上游暴露"用户不存在"与"密码错误"的差异。
		return nil, ErrInvalidCredentials
	}
	return &Identity{
		Provider: KindLocal,
		Username: admin.Username,
		Nickname: admin.Nickname,
		Email:    admin.Email,
	}, nil
}
