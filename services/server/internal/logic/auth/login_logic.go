// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/security/jwtutil"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

var warnDefaultJWT sync.Once

// 用户登录
func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginRequest) (*types.LoginResponse, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, errors.New("用户名不能为空")
	}

	password, err := utils.ValidatePassword(req.Password)
	if err != nil {
		return nil, err
	}

	admin, roles, err := l.authenticateDBUser(username, password)
	if err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	issuedAt := extractIssuedAt(admin)
	token, err := l.issueToken(admin.Username, roles, issuedAt)
	if err != nil {
		return nil, err
	}

	if roles == nil {
		roles = []string{}
	}

	return &types.LoginResponse{
		Token: token,
		User: types.UserInfo{
			Username: admin.Username,
			Roles:    roles,
			Nickname: admin.Nickname,
		},
	}, nil
}

func (l *LoginLogic) authenticateDBUser(username, password string) (*model.Admin, []string, error) {
	admin, err := l.svcCtx.AdminModel.ValidatePassword(l.ctx, username, password)
	if err != nil {
		return nil, nil, err
	}

	l.svcCtx.InvalidateAdminCache(l.ctx, admin.ID, admin.Username)

	roleModels, err := l.svcCtx.AdminModel.GetAdminRoles(l.ctx, admin.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("获取管理员角色失败: %w", err)
	}

	return admin, utils.RoleNamesFromModels(roleModels), nil
}

func extractIssuedAt(admin *model.Admin) time.Time {
	if admin != nil && admin.LastLoginAt != nil {
		return admin.LastLoginAt.UTC()
	}
	return time.Now().UTC()
}

func (l *LoginLogic) issueToken(username string, roles []string, issuedAt time.Time) (string, error) {
	secret, fallback := jwtutil.ResolveSecret(l.svcCtx.Config)
	if fallback {
		warnDefaultJWT.Do(func() {
			logx.WithContext(l.ctx).Error("JWT secret not configured; using dev fallback. Set auth.jwt_secret for production deployments.")
		})
	}

	return jwtutil.Sign(secret, username, roles, issuedAt)
}
