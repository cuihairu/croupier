// 覆盖目标：handler 错误分支（500）、NewService 可选 opsStore、
// GetProfile/GetUserGames/GetPermissions 的 model 错误分支、
// GetUserGames 的过滤分支、UpdateProfile/ChangePassword 的写库失败分支、
// resolveLastLoginAt 的查询失败与时间解析失败分支、UpdateScope 的环境校验分支。
package profile

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

// dropAndRestoreTables 删除指定表并在测试结束后通过 AutoMigrate 恢复，
// 用于触发 model 层数据库错误分支。
func dropAndRestoreTables(t *testing.T, db *gorm.DB, tables ...string) {
	t.Helper()
	for _, table := range tables {
		require.NoError(t, db.Migrator().DropTable(table))
	}
	t.Cleanup(func() {
		require.NoError(t, model.AutoMigrate(db))
		require.NoError(t, db.AutoMigrate(&audit.AuditModel{}))
	})
}

// newRealService 基于共享测试库构造真实 service，并保证结束时清理数据。
func newRealService(t *testing.T) *Service {
	t.Helper()
	db := setupTestDB(t)
	return NewService(model.NewAdminModel(db), model.NewGameModel(db), model.NewRoleModel(db))
}

// createPlainAdmin 创建不带角色的管理员。
func createPlainAdmin(t *testing.T, db *gorm.DB, username string) *model.Admin {
	t.Helper()
	admin := &model.Admin{
		Username: username,
		Nickname: username + " User",
		Status:   1,
	}
	require.NoError(t, model.NewAdminModel(db).Create(context.Background(), admin, "password123"))
	return admin
}

// assignRole 创建角色并绑定到管理员。
func assignRole(t *testing.T, db *gorm.DB, adminID uint, roleName string) {
	t.Helper()
	role := &model.Role{Name: roleName}
	require.NoError(t, db.Where("name = ?", roleName).FirstOrCreate(role).Error)
	require.NoError(t, model.NewAdminModel(db).AssignRole(context.Background(), adminID, role.ID))
}

func TestNewService_WithOpsStore(t *testing.T) {
	db := setupTestDB(t)
	opsStore := svc.NewOpsStateStore(t.TempDir())
	service := NewService(model.NewAdminModel(db), model.NewGameModel(db), model.NewRoleModel(db), opsStore)
	require.NotNil(t, service)
	assert.Equal(t, opsStore, service.opsStore)
}

func TestService_GetProfile_GetAdminRolesError(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	createPlainAdmin(t, db, "rolefailuser")

	dropAndRestoreTables(t, db, "admin_roles")

	_, err := service.GetProfile(context.Background(), "rolefailuser")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "获取用户角色失败")
}

func TestService_GetUserGames_TableErrors(t *testing.T) {
	cases := []struct {
		name   string
		tables []string
		want   string
	}{
		{"roles error", []string{"admin_roles"}, "获取用户角色失败"},
		{"env scopes error", []string{"admin_game_env_scopes"}, "获取游戏环境列表失败"},
		{"games error", []string{"games"}, "获取游戏列表失败"},
		{"env bindings error", []string{"game_envs"}, "获取游戏环境列表失败"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupTestDB(t)
			service := newRealService(t)
			createPlainAdmin(t, db, "tablefailuser")

			dropAndRestoreTables(t, db, tc.tables...)

			_, err := service.GetUserGames(context.Background(), "tablefailuser")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestService_GetPermissions_GetAdminRolesError(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	createPlainAdmin(t, db, "permrolefailuser")

	dropAndRestoreTables(t, db, "admin_roles")

	_, err := service.GetPermissions(context.Background(), "permrolefailuser")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "获取用户角色失败")
}

// registerFailUpdateCallback 注册一个在命中指定更新列时失败的 update callback。
func registerFailUpdateCallback(t *testing.T, db *gorm.DB, name string, failKeys ...string) {
	t.Helper()
	keySet := make(map[string]struct{}, len(failKeys))
	for _, k := range failKeys {
		keySet[k] = struct{}{}
	}
	boom := errors.New("update blocked by test callback")
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
		if m, ok := tx.Statement.Dest.(map[string]interface{}); ok {
			for k := range m {
				if _, hit := keySet[k]; hit {
					_ = tx.AddError(boom)
					return
				}
			}
		}
	}))
	t.Cleanup(func() {
		_ = db.Callback().Update().Remove(name)
	})
}

func TestService_UpdateProfile_UpdateError(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	createPlainAdmin(t, db, "updfailuser")

	registerFailUpdateCallback(t, db, "test_fail_nickname", "nickname")

	_, err := service.UpdateProfile(context.Background(), "updfailuser", &ProfileUpdateRequest{Nickname: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "更新失败")
}

func TestService_ChangePassword_UpdatePasswordError(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	createPlainAdmin(t, db, "pwdupdfailuser")

	registerFailUpdateCallback(t, db, "test_fail_password", "password_hash")

	_, err := service.ChangePassword(context.Background(), "pwdupdfailuser", &ChangePasswordRequest{
		OldPassword: "password123",
		NewPassword: "newpassword456",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "修改密码失败")
}

func TestService_ChangePassword_BumpTokenVersionError(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	createPlainAdmin(t, db, "bumpfailuser")

	registerFailUpdateCallback(t, db, "test_fail_token_version", "token_version")

	_, err := service.ChangePassword(context.Background(), "bumpfailuser", &ChangePasswordRequest{
		OldPassword: "password123",
		NewPassword: "newpassword456",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "修改密码失败")
}

func TestService_ResolveLastLoginAt_QueryError(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t).WithDB(db)

	dropAndRestoreTables(t, db, "audit_records")

	assert.Empty(t, service.resolveLastLoginAt("anyone", nil))
}

func TestService_ResolveLastLoginAt_UnparsableTimestamp(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("DELETE FROM audit_records")
	service := newRealService(t).WithDB(db)

	seedAuditLogin(t, db, "badtsuser", "success", time.Time{})
	db.Exec("UPDATE audit_records SET timestamp = 'not-a-timestamp' WHERE actor_id = ?", "badtsuser")

	assert.Empty(t, service.resolveLastLoginAt("badtsuser", nil))
}

func TestService_GetUserGames_ScopeWithZeroGameIDSkipped(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	admin := createPlainAdmin(t, db, "zeroscopeuser")
	assignRole(t, db, admin.ID, "operator")

	game := &model.Game{Name: "zeroscope_game", AliasName: "Zero Scope Game", Color: "#123456", Status: "running"}
	require.NoError(t, game.SetEnvs([]model.GameEnv{{Env: "prod"}}))
	createProfileTestGame(t, model.NewGameModel(db), game)

	// 有效 scope + GameID 为 0 的脏数据 scope（应被跳过）
	require.NoError(t, model.NewAdminModel(db).SetGameEnvScope(context.Background(), admin.ID, game.ID, "prod"))
	require.NoError(t, db.Exec("INSERT INTO admin_game_env_scopes (admin_id, game_id, env, created_at, updated_at) VALUES (?, 0, 'prod', ?, ?)",
		admin.ID, time.Now(), time.Now()).Error)

	resp, err := service.GetUserGames(context.Background(), "zeroscopeuser")
	require.NoError(t, err)
	require.Len(t, resp.Games, 1)
	assert.Equal(t, "zeroscope_game", resp.Games[0].GameId)
}

func TestService_GetUserGames_BlankGameIDSkipped(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	createPlainAdmin(t, db, "blankgameuser")
	assignRole(t, db, mustGetAdminID(t, db, "blankgameuser"), "admin")

	game := &model.Game{Name: "blankid_game", AliasName: "Blank ID Game", Color: "#123456", Status: "running"}
	require.NoError(t, game.SetEnvs([]model.GameEnv{{Env: "prod"}}))
	createProfileTestGame(t, model.NewGameModel(db), game)
	db.Exec("UPDATE games SET game_id = '  ' WHERE name = ?", "blankid_game")

	resp, err := service.GetUserGames(context.Background(), "blankgameuser")
	require.NoError(t, err)
	for _, g := range resp.Games {
		assert.NotEqual(t, "  ", g.GameId)
	}
}

func TestService_GetUserGames_BlankBindingEnvSkipped(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	admin := createPlainAdmin(t, db, "blankenvuser")
	assignRole(t, db, admin.ID, "admin")

	game := &model.Game{Name: "blankenv_game", AliasName: "Blank Env Game", Color: "#123456", Status: "running"}
	require.NoError(t, game.SetEnvs([]model.GameEnv{{Env: "prod"}}))
	createProfileTestGame(t, model.NewGameModel(db), game)
	require.NoError(t, db.Exec("INSERT INTO game_envs (game_id, env, database_name, created_at, updated_at) VALUES (?, '  ', 'db_blank', ?, ?)",
		"blankenv_game", time.Now(), time.Now()).Error)

	resp, err := service.GetUserGames(context.Background(), "blankenvuser")
	require.NoError(t, err)
	require.Len(t, resp.Games, 1)
	assert.Equal(t, []string{"prod"}, resp.Games[0].Envs)
}

func TestService_GetUserGames_UnauthorizedEnvSkipped(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	admin := createPlainAdmin(t, db, "partialscopeuser")
	assignRole(t, db, admin.ID, "operator")

	game := &model.Game{Name: "partial_game", AliasName: "Partial Game", Color: "#123456", Status: "running"}
	require.NoError(t, game.SetEnvs([]model.GameEnv{{Env: "prod"}, {Env: "staging"}}))
	createProfileTestGame(t, model.NewGameModel(db), game)

	// operator 仅被授权 prod，staging 环境应被过滤
	require.NoError(t, model.NewAdminModel(db).SetGameEnvScope(context.Background(), admin.ID, game.ID, "prod"))

	resp, err := service.GetUserGames(context.Background(), "partialscopeuser")
	require.NoError(t, err)
	require.Len(t, resp.Games, 1)
	assert.Equal(t, []string{"prod"}, resp.Games[0].Envs)
}

func TestService_GetUserGames_GameWithoutBindingSkipped(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	createPlainAdmin(t, db, "nobindinguser")
	assignRole(t, db, mustGetAdminID(t, db, "nobindinguser"), "admin")

	// 只创建 game 元数据，不创建 game_envs 绑定
	game := &model.Game{Name: "nobinding_game", AliasName: "No Binding Game", Color: "#123456", Status: "running"}
	require.NoError(t, model.NewGameModel(db).Create(context.Background(), game))

	resp, err := service.GetUserGames(context.Background(), "nobindinguser")
	require.NoError(t, err)
	for _, g := range resp.Games {
		assert.NotEqual(t, "nobinding_game", g.GameId)
	}
}

func TestService_GetPermissions_EmptyRoleNameIgnored(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	admin := createPlainAdmin(t, db, "emptyrolenameuser")
	assignRole(t, db, admin.ID, "")

	resp, err := service.GetPermissions(context.Background(), "emptyrolenameuser")
	require.NoError(t, err)
	assert.False(t, resp.Admin)
	for _, id := range resp.PermissionIDs {
		assert.NotEmpty(t, id)
	}
}

func TestService_GetPermissions_MergesRolePermissionIDs(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	admin := createPlainAdmin(t, db, "rolepermuser")
	assignRole(t, db, admin.ID, "custom_operator")

	role := &model.Role{}
	require.NoError(t, db.Where("name = ?", "custom_operator").First(role).Error)
	require.NoError(t, db.Create(&model.RolePermission{
		RoleID:       role.ID,
		PermissionID: "function.invoke",
	}).Error)

	resp, err := service.GetPermissions(context.Background(), "rolepermuser")
	require.NoError(t, err)
	assert.Contains(t, resp.PermissionIDs, "function.invoke")
	assert.Contains(t, resp.PermissionIDs, "custom_operator")
}

func TestService_UpdateScope_EnvNotBound(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	admin := createPlainAdmin(t, db, "scopeunbounduser")
	assignRole(t, db, admin.ID, "admin")

	game := &model.Game{Name: "scopeunbound_game", AliasName: "Scope Unbound", Color: "#123456", Status: "running"}
	require.NoError(t, game.SetEnvs([]model.GameEnv{{Env: "prod"}}))
	createProfileTestGame(t, model.NewGameModel(db), game)

	err := service.UpdateScope(context.Background(), admin.ID, "scopeunbound_game", "staging")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "游戏环境不存在")
}

func TestService_UpdateScope_HasEnvBindingError(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	admin := createPlainAdmin(t, db, "scopebinderruser")
	assignRole(t, db, admin.ID, "admin")

	game := &model.Game{Name: "binderr_game", AliasName: "Bind Err", Color: "#123456", Status: "running"}
	require.NoError(t, game.SetEnvs([]model.GameEnv{{Env: "prod"}}))
	createProfileTestGame(t, model.NewGameModel(db), game)

	dropAndRestoreTables(t, db, "game_envs")

	err := service.UpdateScope(context.Background(), admin.ID, "binderr_game", "prod")
	require.Error(t, err)
}

// mustGetAdminID 按用户名查询管理员 ID。
func mustGetAdminID(t *testing.T, db *gorm.DB, username string) uint {
	t.Helper()
	admin, err := model.NewAdminModel(db).FindByUsername(context.Background(), username)
	require.NoError(t, err)
	return admin.ID
}

// --- handler 错误与成功分支 ---

func TestHandler_GetProfile_NotFound_Returns500(t *testing.T) {
	handler := NewHandler(newRealService(t))
	ctx, rec := newProfileTestContext("GET", "/profile", "")
	ctx.Set("username", "ghost-user")
	handler.GetProfile(ctx)
	assertProfileHTTPStatus(t, rec, http.StatusInternalServerError)
}

// newHandlerOnDB 在指定 db 上构造 handler 与 service，避免重复清库。
func newHandlerOnDB(db *gorm.DB) *Handler {
	return NewHandler(NewService(model.NewAdminModel(db), model.NewGameModel(db), model.NewRoleModel(db)))
}

func TestHandler_GetProfile_Success(t *testing.T) {
	db := setupTestDB(t)
	handler := newHandlerOnDB(db)
	createPlainAdmin(t, db, "handlerprofileuser")

	ctx, rec := newProfileTestContext("GET", "/profile", "")
	ctx.Set("username", "handlerprofileuser")
	handler.GetProfile(ctx)
	assertProfileHTTPStatus(t, rec, http.StatusOK)
}

func TestHandler_GetGames_NotFound_Returns500(t *testing.T) {
	handler := NewHandler(newRealService(t))
	ctx, rec := newProfileTestContext("GET", "/profile/games", "")
	ctx.Set("username", "ghost-user")
	handler.GetGames(ctx)
	assertProfileHTTPStatus(t, rec, http.StatusInternalServerError)
}

func TestHandler_GetGames_Success(t *testing.T) {
	handler := NewHandler(newRealService(t))
	createPlainAdmin(t, setupTestDB(t), "handlergamesuser")

	ctx, rec := newProfileTestContext("GET", "/profile/games", "")
	ctx.Set("username", "handlergamesuser")
	handler.GetGames(ctx)
	assertProfileHTTPStatus(t, rec, http.StatusOK)
}

func TestHandler_GetPermissions_NotFound_Returns500(t *testing.T) {
	handler := NewHandler(newRealService(t))
	ctx, rec := newProfileTestContext("GET", "/profile/permissions", "")
	ctx.Set("username", "ghost-user")
	handler.GetPermissions(ctx)
	assertProfileHTTPStatus(t, rec, http.StatusInternalServerError)
}

func TestHandler_GetPermissions_Success(t *testing.T) {
	db := setupTestDB(t)
	handler := NewHandler(newRealService(t))
	admin := createPlainAdmin(t, db, "handlerpermuser")
	assignRole(t, db, admin.ID, "admin")

	ctx, rec := newProfileTestContext("GET", "/profile/permissions", "")
	ctx.Set("username", "handlerpermuser")
	handler.GetPermissions(ctx)
	assertProfileHTTPStatus(t, rec, http.StatusOK)
}
