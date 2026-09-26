package profile

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	adminModel *model.AdminModel
	gameModel  *model.GameModel
	roleModel  *model.RoleModel
	opsStore   *svc.OpsStateStore
	db         *gorm.DB
	// objectStore 用于把头像对象 key 解析成当前可访问的 URL（可为 nil）。
	objectStore objstore.Store
	// invalidateCache 用于资料更新后失效 admin 缓存（可为 nil）。
	invalidateCache func(ctx context.Context, adminID uint, username string)
	// sms 短信服务商注册表（可为 nil → 视为未接入）。
	sms *approvals.SMSRegistry
	// notifyBase 平台侧（站内信/邮件）通道状态来源；nil 时读设置单例。
	notifyBase func() NotificationBaseStatus
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

// WithDB attaches the primary DB so profile lookups can consult persistent
// tables (e.g. audit_records for last-login fallbacks).
func (s *Service) WithDB(db *gorm.DB) *Service {
	s.db = db
	return s
}

// WithObjectStore attaches the object store used to resolve the stored avatar
// object key into a readable URL on every read. Optional: without it the
// avatar degrades to the file driver's relative path form.
func (s *Service) WithObjectStore(store objstore.Store) *Service {
	s.objectStore = store
	return s
}

// resolveAvatar turns the persisted avatar object key into a URL the browser can
// load right now. Signing on read (rather than persisting a signed URL) is what
// keeps the avatar from breaking once the signature expires.
func (s *Service) resolveAvatar(ctx context.Context, key string) string {
	url, err := objstore.ResolveAvatarURL(ctx, s.objectStore, key)
	if err != nil {
		// 解析失败不应让整个 profile 读取失败——头像退化为空，前端走占位图。
		slog.Default().Warn("头像地址解析失败，回退为占位图", "key", key, "error", err)
		return ""
	}
	return url
}

// WithNotifyBaseStatus overrides the platform-side (in-app / email) channel
// status source. Test seam: the production source is the settings singleton.
func (s *Service) WithNotifyBaseStatus(fn func() NotificationBaseStatus) *Service {
	s.notifyBase = fn
	return s
}

// WithSMSRegistry wires the SMS provider registry used to report whether the SMS
// channel is really available (docs/BUGS.md BUG-016).
func (s *Service) WithSMSRegistry(r *approvals.SMSRegistry) *Service {
	s.sms = r
	return s
}

// WithCacheInvalidator wires the admin-cache eviction hook.
//
// cache_layer caches the whole model.Admin (Avatar included) under both
// admin:id:<id> and admin:username:<name>; without eviction after a profile
// update the next reader keeps seeing the pre-update avatar until the TTL
// expires. The hook is injected rather than imported because invalidation lives
// on *svc.ServiceContext, which this package deliberately does not depend on.
func (s *Service) WithCacheInvalidator(fn func(ctx context.Context, adminID uint, username string)) *Service {
	s.invalidateCache = fn
	return s
}

// invalidateAdminCache drops cached admin rows after a profile update.
func (s *Service) invalidateAdminCache(ctx context.Context, id uint, username string) {
	if s.invalidateCache == nil {
		return
	}
	s.invalidateCache(ctx, id, username)
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
			Id:       int64(admin.ID),
			Username: admin.Username,
			Nickname: admin.Nickname,
			Email:    admin.Email,
			Phone:    admin.Phone,
			Active:   admin.Status == 1,
			Roles:    roles,
			// 库里存的是裸对象 key，这里现算一个当前有效的访问地址。
			Avatar:      s.resolveAvatar(ctx, admin.Avatar),
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
	// The legacy in-memory ops audit trail was removed; fall back to the
	// persistent audit_records table (auth.login by this actor).
	if s == nil || s.db == nil {
		return ""
	}
	var last string
	if err := s.db.Table("audit_records").
		Select("MAX(timestamp)").
		Where("event_type = ? AND outcome = 'success' AND actor_id = ?", "auth.login", strings.TrimSpace(username)).
		Scan(&last).Error; err != nil {
		return ""
	}
	last = strings.TrimSpace(last)
	if last == "" {
		return ""
	}
	parsedAt, err := time.Parse(time.RFC3339Nano, last)
	if err != nil {
		// SQLite stores timestamps as "2006-01-02 15:04:05.999999999-07:00".
		for _, layout := range []string{"2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"} {
			if parsed, parseErr := time.Parse(layout, last); parseErr == nil {
				parsedAt = parsed
				err = nil
				break
			}
		}
	}
	if err != nil || parsedAt.IsZero() {
		return ""
	}
	return parsedAt.UTC().Format(time.RFC3339)
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
			// bindingsByGameID 构建时（上方 147-152 行）已剔除 TrimSpace 后 env 为空的
			// 绑定，此处 env 恒非空，原空值分支为死代码，已删除。
			env := strings.TrimSpace(binding.Env)
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

	// 只写请求里真正携带的字段。指针为 nil = 未携带 = 保留库中原值；
	// 显式传空串则视为「清空该字段」（例如把头像重置回占位图）。
	updates := map[string]interface{}{}
	if req.Nickname != nil {
		updates["nickname"] = *req.Nickname
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	if req.Avatar != nil {
		// 头像只存对象 key，不存带签名/带前缀的 URL。签名 URL 会过期
		// （S3 驱动默认 15min TTL），存进库就成了定时失效的死链。
		// 具体驱动的 URL 解析见 objstore 包。
		key, err := objstore.NormalizeAvatarKey(*req.Avatar)
		if err != nil {
			return nil, err
		}
		if key == "" {
			// 归一后为空 = 用户清空头像，直接落空串。
			updates["avatar"] = ""
		} else {
			updates["avatar"] = key
		}
	}
	if len(updates) == 0 {
		// 全空请求无需写库（否则 gorm Updates 空 map 会误报 no-op 错误）。
		return &ProfileUpdateResponse{Ok: true}, nil
	}

	// 保存更新
	if err := s.adminModel.Update(ctx, admin.ID, updates); err != nil {
		return nil, errors.New("更新失败")
	}
	// 头像变更须失效 admin 缓存：cache_layer 缓存整个 model.Admin（含 Avatar），
	// 读取方若走 GetAdminCached 会继续命中旧头像。
	s.invalidateAdminCache(ctx, admin.ID, admin.Username)

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

	// 密码变更即吊销所有已签发 token（当前请求完成后旧 token 失效，
	// 前端需引导重新登录）
	if err := s.adminModel.BumpTokenVersion(ctx, admin.ID); err != nil {
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
