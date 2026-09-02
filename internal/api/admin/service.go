package admin

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List retrieves a paginated list of admins
func (s *Service) List(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看管理员列表", "admin:all", "user:read", "user:write"); err != nil {
		return nil, err
	}

	opts := model.ListAdminsOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		Search:   strings.TrimSpace(req.Search),
		Role:     strings.TrimSpace(req.Role),
	}

	// Only apply status filter if Status > 0
	// Status 0 is the default and means "no filter" in the context of this API
	// Status 1 means "active", Status < 0 means "no filter"
	if req.Status > 0 {
		status := req.Status
		opts.Status = &status
	}

	admins, total, err := s.svcCtx.AdminModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	resp := &ListResponse{
		Items: make([]Admin, 0, len(admins)),
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}

	if len(admins) == 0 {
		return resp, nil
	}

	adminIDs := make([]uint, 0, len(admins))
	for i := range admins {
		adminIDs = append(adminIDs, admins[i].ID)
	}

	roleMap, err := s.loadAdminRoleNames(ctx, adminIDs)
	if err != nil {
		return nil, err
	}

	for i := range admins {
		admin := admins[i]
		resp.Items = append(resp.Items, buildAdminResponse(&admin, roleMap[admin.ID]))
	}

	return resp, nil
}

// Create creates a new admin user
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*CreateResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权创建管理员", "admin:all", "user:write"); err != nil {
		return nil, err
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, errorx.NewBadRequest("用户名不能为空")
	}
	password, err := utils.ValidatePassword(req.Password)
	if err != nil {
		return nil, err
	}

	var createdAdmin *model.Admin
	var assignedRoles []model.Role

	err = s.svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		adminModel := model.NewAdminModel(tx)
		admin := &model.Admin{
			Username: username,
			Nickname: strings.TrimSpace(req.Nickname),
			Email:    strings.TrimSpace(req.Email),
			Phone:    strings.TrimSpace(req.Phone),
			Status:   1,
		}

		if err := adminModel.Create(ctx, admin, password); err != nil {
			return err
		}

		if len(req.Roles) > 0 {
			roles, err := fetchRolesByNames(ctx, tx, req.Roles)
			if err != nil {
				return err
			}
			for _, role := range roles {
				if err := adminModel.AssignRole(ctx, admin.ID, role.ID); err != nil {
					return errorx.NewInternalError("绑定角色失败")
				}
			}
			assignedRoles = roles
		}

		createdAdmin = admin
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &CreateResponse{
		Admin: buildAdminResponse(createdAdmin, roleNamesFromModels(assignedRoles)),
	}, nil
}

// Get retrieves details of a specific admin
func (s *Service) Get(ctx context.Context, req *GetRequest) (*GetResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看管理员", "admin:all", "user:read", "user:write"); err != nil {
		return nil, err
	}

	adminID, err := parseAdminID(req.ID)
	if err != nil {
		return nil, err
	}

	admin, err := s.svcCtx.GetAdminCached(ctx, adminID)
	if err != nil {
		return nil, err
	}

	roles, err := s.svcCtx.GetAdminRolesCached(ctx, admin.ID)
	if err != nil {
		return nil, errorx.NewInternalError("获取管理员角色失败")
	}

	return &GetResponse{
		Admin: buildAdminResponse(admin, roleNamesFromModels(roles)),
	}, nil
}

// Update updates an existing admin
func (s *Service) Update(ctx context.Context, req *UpdateRequest) (*UpdateResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权更新管理员", "admin:all", "user:write"); err != nil {
		return nil, err
	}

	adminID, err := parseAdminID(req.ID)
	if err != nil {
		return nil, err
	}

	var existing *model.Admin
	err = s.svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		adminModel := model.NewAdminModel(tx)

		adminRecord, err := adminModel.FindOne(ctx, adminID)
		if err != nil {
			return err
		}
		existing = adminRecord

		updates := make(map[string]interface{})
		if nickname := strings.TrimSpace(req.Nickname); nickname != "" {
			updates["nickname"] = nickname
		}
		if email := strings.TrimSpace(req.Email); email != "" {
			updates["email"] = email
		}
		if phone := strings.TrimSpace(req.Phone); phone != "" {
			updates["phone"] = phone
		}
		if req.Status != -1 {
			updates["status"] = req.Status
		}

		if len(updates) > 0 {
			if err := adminModel.Update(ctx, adminID, updates); err != nil {
				return err
			}
			// 禁用账号即吊销所有已签发 token
			if req.Status == 0 {
				if err := adminModel.BumpTokenVersion(ctx, adminID); err != nil {
					return err
				}
			}
		}

		if req.Roles != nil {
			if err := tx.WithContext(ctx).
				Where("admin_id = ?", adminID).
				Delete(&model.AdminRole{}).Error; err != nil {
				return errorx.NewInternalError("清理旧角色失败")
			}

			if len(req.Roles) > 0 {
				roles, err := fetchRolesByNames(ctx, tx, req.Roles)
				if err != nil {
					return err
				}
				for _, role := range roles {
					if err := adminModel.AssignRole(ctx, adminID, role.ID); err != nil {
						return errorx.NewInternalError("分配角色失败")
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	username := ""
	if existing != nil {
		username = existing.Username
	}
	s.svcCtx.InvalidateAdminCache(ctx, adminID, username)

	admin, err := s.svcCtx.GetAdminCached(ctx, adminID)
	if err != nil {
		return nil, err
	}

	roles, err := s.svcCtx.GetAdminRolesCached(ctx, adminID)
	if err != nil {
		return nil, errorx.NewInternalError("获取管理员角色失败")
	}

	return &UpdateResponse{
		Admin: buildAdminResponse(admin, roleNamesFromModels(roles)),
	}, nil
}

// Delete deletes an admin
func (s *Service) Delete(ctx context.Context, req *DeleteRequest) error {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权删除管理员", "admin:all", "user:write"); err != nil {
		return err
	}

	adminID, err := parseAdminID(req.ID)
	if err != nil {
		return err
	}

	var existing *model.Admin
	if err := s.svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		adminModel := model.NewAdminModel(tx)

		adminRecord, err := adminModel.FindOne(ctx, adminID)
		if err != nil {
			return err
		}
		existing = adminRecord

		if err := tx.WithContext(ctx).
			Where("admin_id = ?", adminID).
			Delete(&model.AdminRole{}).Error; err != nil {
			return errorx.NewInternalError("删除角色绑定失败")
		}

		if err := tx.WithContext(ctx).
			Where("admin_id = ?", adminID).
			Delete(&model.AdminGameScope{}).Error; err != nil {
			return errorx.NewInternalError("删除游戏范围失败")
		}

		if err := tx.WithContext(ctx).
			Where("admin_id = ?", adminID).
			Delete(&model.AdminGameEnvScope{}).Error; err != nil {
			return errorx.NewInternalError("删除环境范围失败")
		}

		return adminModel.Delete(ctx, adminID)
	}); err != nil {
		return err
	}

	username := ""
	if existing != nil {
		username = existing.Username
	}
	s.svcCtx.InvalidateAdminCache(ctx, adminID, username)

	return nil
}

// PasswordReset resets an admin's password
func (s *Service) PasswordReset(ctx context.Context, req *PasswordResetRequest) error {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权重置管理员密码", "admin:all", "user:write"); err != nil {
		return err
	}

	adminID, err := parseAdminID(req.ID)
	if err != nil {
		return err
	}

	password, err := utils.ValidatePassword(req.NewPassword)
	if err != nil {
		return err
	}

	admin, err := s.svcCtx.AdminModel.FindOne(ctx, adminID)
	if err != nil {
		return err
	}

	if err := s.svcCtx.AdminModel.UpdatePassword(ctx, adminID, password); err != nil {
		return err
	}

	// 密码变更即吊销该账号所有已签发 token
	if err := s.svcCtx.AdminModel.BumpTokenVersion(ctx, adminID); err != nil {
		return err
	}

	s.svcCtx.InvalidateAdminCache(ctx, adminID, admin.Username)

	return nil
}

// GetGames retrieves the game scopes for an admin
func (s *Service) GetGames(ctx context.Context, req *GetGamesRequest) (*GetGamesResponse, error) {
	adminID, err := parseAdminID(req.ID)
	if err != nil {
		return nil, err
	}

	// Only super roles can read other admin scopes.
	_, roles, err := utils.LoadCurrentAdmin(ctx, s.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	permIDs, err := utils.PermissionIDsFromRoles(ctx, s.svcCtx, roles)
	if err != nil {
		return nil, err
	}
	if !utils.HasAdminRole(roleNames) && !utils.HasPermissionID(permIDs, "admin:all") && !utils.HasPermissionID(permIDs, "*") && !utils.HasPermissionID(permIDs, "user:write") {
		return nil, errorx.NewForbidden("无权查看管理员游戏范围")
	}

	if s.svcCtx.DB == nil || s.svcCtx.GameModel == nil {
		return nil, errorx.NewInternalError("DB/GameModel 未初始化")
	}

	type row struct {
		GameID   uint
		GameName string
		Alias    string
		Env      string
	}
	var rows []row
	err = s.svcCtx.DB.WithContext(ctx).
		Table("admin_game_env_scopes").
		Select("admin_game_env_scopes.game_id as game_id, games.name as game_name, games.alias_name as alias, admin_game_env_scopes.env as env").
		Joins("INNER JOIN games ON games.id = admin_game_env_scopes.game_id").
		Where("admin_game_env_scopes.admin_id = ?", adminID).
		Find(&rows).Error
	if err != nil {
		return nil, errorx.NewInternalError("query env scopes failed")
	}

	envByGame := make(map[uint][]string)
	gameMeta := make(map[uint]AdminGame)
	for _, r := range rows {
		env := strings.TrimSpace(r.Env)
		if env == "" {
			continue
		}
		envByGame[r.GameID] = append(envByGame[r.GameID], env)
		if _, ok := gameMeta[r.GameID]; !ok {
			name := strings.TrimSpace(r.GameName)
			if name == "" {
				name = strings.TrimSpace(r.Alias)
			}
			gameMeta[r.GameID] = AdminGame{
				GameId:   r.GameName,
				GameName: name,
				Envs:     []string{},
			}
		}
	}

	// Also include game-only scopes (all envs).
	var gameScopes []model.AdminGameScope
	if err := s.svcCtx.DB.WithContext(ctx).Where("admin_id = ?", adminID).Find(&gameScopes).Error; err != nil {
		return nil, errorx.NewInternalError("query game scopes failed")
	}
	for _, scope := range gameScopes {
		if _, ok := gameMeta[scope.GameID]; ok {
			continue
		}
		game, err := s.svcCtx.GameModel.FindOne(ctx, scope.GameID)
		if err != nil || game == nil {
			continue
		}
		gameMeta[scope.GameID] = AdminGame{
			GameId:   game.Name,
			GameName: strings.TrimSpace(game.AliasName),
			Envs:     []string{},
		}
	}

	items := make([]AdminGame, 0, len(gameMeta))
	for gid, item := range gameMeta {
		envs := uniqueStrings(envByGame[gid])
		sort.Strings(envs)
		item.Envs = envs
		if item.GameName == "" {
			item.GameName = item.GameId
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GameId < items[j].GameId })
	return &GetGamesResponse{Games: items}, nil
}

// UpdateGames updates the game scopes for an admin
func (s *Service) UpdateGames(ctx context.Context, req *UpdateGamesRequest) (*GetGamesResponse, error) {
	targetAdminID, err := parseAdminID(req.ID)
	if err != nil {
		return nil, err
	}

	// Only super roles can update other admin scopes.
	_, roles, err := utils.LoadCurrentAdmin(ctx, s.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	permIDs, err := utils.PermissionIDsFromRoles(ctx, s.svcCtx, roles)
	if err != nil {
		return nil, err
	}
	if !utils.HasAdminRole(roleNames) && !utils.HasPermissionID(permIDs, "admin:all") && !utils.HasPermissionID(permIDs, "*") && !utils.HasPermissionID(permIDs, "user:write") {
		return nil, errorx.NewForbidden("无权更新管理员游戏范围")
	}

	if s.svcCtx.DB == nil || s.svcCtx.GameModel == nil {
		return nil, errorx.NewInternalError("DB/GameModel 未初始化")
	}

	// Normalize input early.
	games := req.Games
	if games == nil {
		games = []AdminGame{}
	}

	err = s.svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("admin_id = ?", targetAdminID).Delete(&model.AdminGameEnvScope{}).Error; err != nil {
			return err
		}
		if err := tx.Where("admin_id = ?", targetAdminID).Delete(&model.AdminGameScope{}).Error; err != nil {
			return err
		}

		for _, entry := range games {
			gameName := strings.TrimSpace(entry.GameId)
			if gameName == "" {
				continue
			}
			game, err := s.svcCtx.GameModel.FindByName(ctx, gameName)
			if err != nil || game == nil {
				return errorx.NewNotFound("game not found: " + gameName)
			}

			// Always insert game scope entry for quick allow (envs empty means all env).
			if err := tx.Create(&model.AdminGameScope{AdminID: targetAdminID, GameID: game.ID}).Error; err != nil {
				return err
			}

			for _, env := range entry.Envs {
				trimmed := strings.TrimSpace(env)
				if trimmed == "" {
					continue
				}
				if err := tx.Create(&model.AdminGameEnvScope{
					AdminID: targetAdminID,
					GameID:  game.ID,
					Env:     trimmed,
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Return latest.
	return s.GetGames(ctx, &GetGamesRequest{ID: req.ID})
}

// Helper functions

func (s *Service) loadAdminRoleNames(ctx context.Context, adminIDs []uint) (map[uint][]string, error) {
	if len(adminIDs) == 0 {
		return map[uint][]string{}, nil
	}

	type row struct {
		AdminID  uint
		RoleName string
	}

	var rows []row
	if err := s.svcCtx.DB.WithContext(ctx).
		Table("admin_roles").
		Select("admin_roles.admin_id AS admin_id, roles.name AS role_name").
		Joins("INNER JOIN roles ON roles.id = admin_roles.role_id").
		Where("admin_roles.admin_id IN ?", adminIDs).
		Order("admin_roles.admin_id").
		Scan(&rows).Error; err != nil {
		return nil, errorx.NewInternalError("查询管理员角色失败")
	}

	roleMap := make(map[uint][]string, len(adminIDs))
	for _, row := range rows {
		roleMap[row.AdminID] = append(roleMap[row.AdminID], row.RoleName)
	}

	return roleMap, nil
}

func parseAdminID(id string) (uint, error) {
	if strings.TrimSpace(id) == "" {
		return 0, errorx.NewBadRequest("管理员ID不能为空")
	}

	value, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, errorx.NewBadRequest("无效的管理员ID")
	}

	if value == 0 {
		return 0, errorx.NewBadRequest("管理员ID必须大于0")
	}

	if value > math.MaxUint {
		return 0, errorx.NewBadRequest("管理员ID超出范围")
	}

	return uint(value), nil
}

func buildAdminResponse(admin *model.Admin, roleNames []string) Admin {
	return Admin{
		Id:        int64(admin.ID),
		Username:  admin.Username,
		Nickname:  admin.Nickname,
		Email:     admin.Email,
		Phone:     admin.Phone,
		Roles:     roleNames,
		Status:    admin.Status,
		CreatedAt: formatTimestamp(admin.CreatedAt),
		UpdatedAt: formatTimestamp(admin.UpdatedAt),
	}
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func roleNamesFromModels(roles []model.Role) []string {
	if len(roles) == 0 {
		return nil
	}

	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name)
	}
	return names
}

func fetchRolesByNames(ctx context.Context, db *gorm.DB, names []string) ([]model.Role, error) {
	ordered, lowered := uniqueRoleInputs(names)
	if len(ordered) == 0 {
		return nil, nil
	}

	var roles []model.Role
	if err := db.WithContext(ctx).
		Where("LOWER(name) IN ?", lowered).
		Find(&roles).Error; err != nil {
		return nil, errorx.NewInternalError("查询角色失败")
	}

	if len(roles) == 0 {
		return nil, errorx.NewBadRequest("角色不存在: " + strings.Join(ordered, ", "))
	}

	found := make(map[string]model.Role, len(roles))
	for _, role := range roles {
		found[strings.ToLower(role.Name)] = role
	}

	var missing []string
	for _, name := range ordered {
		if _, ok := found[strings.ToLower(name)]; !ok {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return nil, errorx.NewBadRequest("角色不存在: " + strings.Join(missing, ", "))
	}

	orderedRoles := make([]model.Role, 0, len(ordered))
	for _, name := range ordered {
		orderedRoles = append(orderedRoles, found[strings.ToLower(name)])
	}
	return orderedRoles, nil
}

func uniqueRoleInputs(names []string) ([]string, []string) {
	if len(names) == 0 {
		return nil, nil
	}

	ordered := make([]string, 0, len(names))
	lowered := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, raw := range names {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		ordered = append(ordered, trimmed)
		lowered = append(lowered, key)
	}

	return ordered, lowered
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		t := strings.TrimSpace(v)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	return out
}
