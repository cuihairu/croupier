package profile

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	adminModel *model.AdminModel
	gameModel  *model.GameModel
	roleModel  *model.RoleModel
	opsStore   *svc.OpsStateStore
}

func NewService(adminModel *model.AdminModel, gameModel *model.GameModel, roleModel *model.RoleModel, opsStore ...*svc.OpsStateStore) *Service {
	var store *svc.OpsStateStore
	if len(opsStore) > 0 {
		store = opsStore[0]
	}
	return &Service{
		adminModel: adminModel,
		gameModel:  gameModel,
		roleModel:  roleModel,
		opsStore:   store,
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
			Id:          int64(admin.ID),
			Username:    admin.Username,
			Nickname:    admin.Nickname,
			Email:       admin.Email,
			Phone:       admin.Phone,
			Active:      admin.Status == 1,
			Roles:       roles,
			Avatar:      admin.Avatar,
			CreatedAt:   admin.CreatedAt.String(),
			UpdatedAt:   admin.UpdatedAt.String(),
			LastLoginAt: s.resolveLastLoginAt(admin.Username, admin.LastLoginAt),
		},
	}, nil
}

func (s *Service) resolveLastLoginAt(username string, ts *time.Time) string {
	if ts != nil && !ts.IsZero() {
		return ts.UTC().Format(time.RFC3339)
	}
	if s == nil || s.opsStore == nil {
		return ""
	}
	state := s.opsStore.Snapshot()
	for i := len(state.Audit.Entries) - 1; i >= 0; i-- {
		entry := state.Audit.Entries[i]
		if !strings.EqualFold(strings.TrimSpace(entry.UserID), strings.TrimSpace(username)) {
			continue
		}
		action := strings.ToLower(strings.TrimSpace(entry.Action))
		if action != "auth.login" && action != "login" {
			continue
		}
		if strings.TrimSpace(entry.Result) != "" && !strings.EqualFold(strings.TrimSpace(entry.Result), "success") {
			continue
		}
		if entry.CreatedAt.IsZero() {
			continue
		}
		return entry.CreatedAt.UTC().Format(time.RFC3339)
	}
	return ""
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

	envScopes, err := s.adminModel.GetAdminEnvScopes(ctx, admin.ID)
	if err != nil {
		return nil, errors.New("获取游戏环境列表失败")
	}

	gameModels, err := s.gameModel.ListAll(ctx)
	if err != nil {
		return nil, errors.New("获取游戏列表失败")
	}
	bindings, err := s.gameModel.ListAllEnvBindings(ctx)
	if err != nil {
		return nil, errors.New("获取游戏环境列表失败")
	}

	// game_envs is the single source of truth for selectable environments.
	// admin_game_env_scopes only narrows that authoritative set for non-admins.
	bindingsByGameID := make(map[string][]model.GameEnvBinding)
	for _, binding := range bindings {
		gameID := strings.TrimSpace(binding.GameID)
		env := strings.TrimSpace(binding.Env)
		if gameID == "" || env == "" {
			continue
		}
		bindingsByGameID[gameID] = append(bindingsByGameID[gameID], binding)
	}

	isAdmin := hasProfileAdminRole(roleModels)
	envScopeFilter := make(map[uint]map[string]struct{}, len(envScopes))
	if !isAdmin {
		for _, scope := range envScopes {
			env := strings.ToLower(strings.TrimSpace(scope.Env))
			if scope.GameID == 0 || env == "" {
				continue
			}
			if envScopeFilter[scope.GameID] == nil {
				envScopeFilter[scope.GameID] = make(map[string]struct{})
			}
			envScopeFilter[scope.GameID][env] = struct{}{}
		}
	}

	games := make([]ProfileGame, 0, len(gameModels))
	seen := make(map[string]struct{}, len(gameModels))
	for _, game := range gameModels {
		gameID := strings.TrimSpace(game.GameID)
		if gameID == "" {
			continue
		}
		if !isAdmin && len(envScopeFilter[game.ID]) == 0 {
			continue
		}
		if _, ok := seen[gameID]; ok {
			continue
		}

		gameBindings := bindingsByGameID[gameID]
		envMeta := make([]model.GameEnv, 0, len(gameBindings))
		envs := make([]string, 0, len(gameBindings))
		for _, binding := range gameBindings {
			env := strings.TrimSpace(binding.Env)
			if env == "" {
				continue
			}
			if !isAdmin {
				if _, authorized := envScopeFilter[game.ID][strings.ToLower(env)]; !authorized {
					continue
				}
			}
			envMeta = append(envMeta, model.GameEnv{
				Env:         env,
				Description: binding.Description,
				Color:       binding.Color,
			})
			envs = append(envs, env)
		}
		if len(envs) == 0 {
			// A game without an authoritative environment binding cannot produce
			// a valid request scope and must not be selectable in the UI.
			continue
		}
		seen[gameID] = struct{}{}

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

// UpdateScope persists the user's game/env selection after validating
// that the game/env exists and the user is authorized.
func (s *Service) UpdateScope(ctx context.Context, adminID uint, gameID, env string) error {
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	if gameID == "" || env == "" {
		return errorx.NewBadRequest("gameId 和 env 不能为空")
	}

	// Validate game/env exists
	game, err := s.gameModel.FindByGameIDString(ctx, gameID)
	if err != nil || game == nil {
		return errorx.NewNotFound("游戏不存在")
	}
	bound, err := s.gameModel.HasEnvBinding(ctx, gameID, env)
	if err != nil {
		return err
	}
	if !bound {
		return errorx.NewNotFound("游戏环境不存在")
	}

	// Validate user authorization
	roles, err := s.adminModel.GetAdminRoles(ctx, adminID)
	isAdmin := false
	if err == nil {
		for _, r := range roles {
			name := strings.ToLower(strings.TrimSpace(r.Name))
			if name == "admin" || name == "super_admin" {
				isAdmin = true
				break
			}
		}
	}

	if !isAdmin {
		authorized := false
		envScopes, err := s.adminModel.GetAdminEnvScopes(ctx, adminID)
		if err == nil {
			for _, s := range envScopes {
				if s.GameID == game.ID && strings.EqualFold(strings.TrimSpace(s.Env), env) {
					authorized = true
					break
				}
			}
		}
		if !authorized {
			return errorx.NewForbidden("无权访问该游戏环境")
		}
	}

	return s.adminModel.UpdateLastScope(ctx, adminID, gameID, env)
}
