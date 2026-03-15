package profile

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
)

type Service struct {
	adminModel *model.AdminModel
	gameModel  *model.GameModel
	roleModel  *model.RoleModel
}

func NewService(adminModel *model.AdminModel, gameModel *model.GameModel, roleModel *model.RoleModel) *Service {
	return &Service{
		adminModel: adminModel,
		gameModel:  gameModel,
		roleModel:  roleModel,
	}
}

// GetProfile 获取个人资料
func (s *Service) GetProfile(ctx context.Context, username string) (*ProfileGetResponse, error) {
	// 查询管理员信息
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	// 获取角色
	roleModels, err := s.adminModel.GetAdminRoles(ctx, admin.ID)
	if err != nil {
		return nil, errors.New("获取用户角色失败")
	}

	roles := make([]string, 0, len(roleModels))
	for _, role := range roleModels {
		roles = append(roles, role.Name)
	}

	return &ProfileGetResponse{
		ProfileInfo: ProfileInfo{
			Id:        int64(admin.ID),
			Username:  admin.Username,
			Nickname:  admin.Nickname,
			Email:     admin.Email,
			Phone:     admin.Phone,
			Roles:     roles,
			Avatar:    admin.Avatar,
			CreatedAt: admin.CreatedAt.String(),
			UpdatedAt: admin.UpdatedAt.String(),
		},
	}, nil
}

// GetUserGames 获取用户的游戏列表
func (s *Service) GetUserGames(ctx context.Context, username string) (*ProfileGamesResponse, error) {
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	roleModels, err := s.adminModel.GetAdminRoles(ctx, admin.ID)
	if err != nil {
		return nil, errors.New("获取用户角色失败")
	}

	adminGames, err := s.adminModel.GetAdminGames(ctx, admin.ID)
	if err != nil {
		return nil, errors.New("获取游戏列表失败")
	}

	var gameModels []model.Game
	if hasProfileAdminRole(roleModels) || len(adminGames) == 0 {
		gameModels, err = s.gameModel.ListAll(ctx)
		if err != nil {
			return nil, errors.New("获取游戏列表失败")
		}
	} else {
		gameModels = make([]model.Game, 0, len(adminGames))
		for _, ag := range adminGames {
			game, err := s.gameModel.FindByGameID(ctx, ag.GameID)
			if err != nil || game == nil {
				continue
			}
			gameModels = append(gameModels, *game)
		}
	}

	games := make([]ProfileGame, 0, len(gameModels))
	seen := make(map[string]struct{}, len(gameModels))
	for _, game := range gameModels {
		gameID := strings.TrimSpace(game.Name)
		if gameID == "" {
			continue
		}
		if _, ok := seen[gameID]; ok {
			continue
		}
		seen[gameID] = struct{}{}

		envMeta, _ := game.GetEnvs()
		envs := make([]string, 0, len(envMeta))
		for _, env := range envMeta {
			if trimmed := strings.TrimSpace(env.Env); trimmed != "" {
				envs = append(envs, trimmed)
			}
		}

		gameName := strings.TrimSpace(game.AliasName)
		if gameName == "" {
			gameName = gameID
		}

		games = append(games, ProfileGame{
			GameId:      gameID,
			GameName:    gameName,
			Color:       game.Color,
			Envs:        envs,
			EnvMeta:     envMeta,
			Permissions: []string{},
		})
	}

	return &ProfileGamesResponse{
		Games: games,
	}, nil
}

func hasProfileAdminRole(roles []model.Role) bool {
	for _, role := range roles {
		switch strings.ToLower(strings.TrimSpace(role.Name)) {
		case "admin", "super_admin":
			return true
		}
	}
	return false
}

// UpdateProfile 更新个人资料
func (s *Service) UpdateProfile(ctx context.Context, username string, req *ProfileUpdateRequest) (*ProfileUpdateResponse, error) {
	// 查询管理员信息
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	// 准备更新字段
	updates := map[string]interface{}{
		"nickname": req.Nickname,
		"email":    req.Email,
		"phone":    req.Phone,
		"avatar":   req.Avatar,
	}

	// 保存更新
	if err := s.adminModel.Update(ctx, admin.ID, updates); err != nil {
		return nil, errors.New("更新失败")
	}

	return &ProfileUpdateResponse{Ok: true}, nil
}

// ChangePassword 修改密码
func (s *Service) ChangePassword(ctx context.Context, username string, req *ChangePasswordRequest) (*ChangePasswordResponse, error) {
	// 验证旧密码
	_, err := s.adminModel.ValidatePassword(ctx, username, req.OldPassword)
	if err != nil {
		return nil, errors.New("旧密码错误")
	}

	// 查询管理员信息
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	// 更新密码
	if err := s.adminModel.UpdatePassword(ctx, admin.ID, req.NewPassword); err != nil {
		return nil, errors.New("修改密码失败")
	}

	return &ChangePasswordResponse{Ok: true}, nil
}

// GetPermissions 获取用户权限列表
func (s *Service) GetPermissions(ctx context.Context, username string) (*ProfilePermissionsResponse, error) {
	// 查询管理员信息
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	// 获取角色
	roleModels, err := s.adminModel.GetAdminRoles(ctx, admin.ID)
	if err != nil {
		return nil, errors.New("获取用户角色失败")
	}

	// 返回角色名称作为权限
	roles := make([]string, 0, len(roleModels))
	isAdmin := false
	for _, role := range roleModels {
		roles = append(roles, role.Name)
		// 检查是否是管理员角色
		if role.Name == "admin" || role.Name == "super_admin" {
			isAdmin = true
		}
	}

	// 构建权限ID列表
	permissionSet := make(map[string]struct{}, len(roles)+8)
	permissionIDs := make([]string, 0, len(roles)+8)
	appendPermission := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := permissionSet[id]; ok {
			return
		}
		permissionSet[id] = struct{}{}
		permissionIDs = append(permissionIDs, id)
	}
	roleIDs := make([]uint, 0, len(roleModels))
	for _, role := range roles {
		appendPermission(role)
		// 如果是管理员角色，添加通配符权限
		if role == "admin" || role == "super_admin" {
			appendPermission("admin")
			appendPermission("*")
		}
	}
	for _, role := range roleModels {
		roleIDs = append(roleIDs, role.ID)
	}
	if s.roleModel != nil && len(roleIDs) > 0 {
		rolePermMap, err := s.roleModel.GetRolesPermissionIDs(ctx, roleIDs)
		if err == nil {
			for _, ids := range rolePermMap {
				for _, id := range ids {
					appendPermission(id)
				}
			}
		}
	}
	sort.Strings(permissionIDs)

	permissions := make([]ProfilePermission, 0, len(roleModels))
	for _, role := range roleModels {
		permissions = append(permissions, ProfilePermission{
			Resource: "role",
			Actions:  []string{role.Name},
		})
	}

	return &ProfilePermissionsResponse{
		Permissions:   permissions,
		Admin:         isAdmin,
		Roles:         roles,
		PermissionIDs: permissionIDs,
	}, nil
}
