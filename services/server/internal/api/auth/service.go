package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/pkg/jwt"
)

type Service struct {
	adminModel *model.AdminModel
}

func NewService(adminModel *model.AdminModel) *Service {
	return &Service{
		adminModel: adminModel,
	}
}

// Login 用户登录
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, errors.New("用户名不能为空")
	}

	password := strings.TrimSpace(req.Password)
	if password == "" {
		return nil, errors.New("密码不能为空")
	}

	// 验证用户密码
	admin, err := s.adminModel.ValidatePassword(ctx, username, password)
	if err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	// 获取用户角色
	roleModels, err := s.adminModel.GetAdminRoles(ctx, admin.ID)
	if err != nil {
		return nil, errors.New("获取用户角色失败")
	}

	// 提取角色名称
	roles := make([]string, 0, len(roleModels))
	for _, role := range roleModels {
		roles = append(roles, role.Name)
	}

	// 生成 JWT token
	token, err := jwt.GenerateToken(admin.Username, roles, admin.ID)
	if err != nil {
		return nil, errors.New("生成 token 失败")
	}

	return &LoginResponse{
		Token: token,
		User: UserInfo{
			Username: admin.Username,
			Nickname: admin.Nickname,
			Roles:    roles,
		},
	}, nil
}

// Logout 用户登出
func (s *Service) Logout(ctx context.Context, req *LogoutRequest) (*LogoutResponse, error) {
	// 如果需要实现 token 黑名单，可以在这里添加逻辑
	return &LogoutResponse{}, nil
}
