package profile

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/rbac"
)

// splitForTest 直接走 rbac 的通配解析，确保 profile 侧与全后端口径同源。
func splitForTest(id string) (string, string) { return rbac.SplitLogicalPermission(id) }

// newPermService 造一个只用到权限解析所需模型的 Service。
func newPermService(t *testing.T, db *gorm.DB) *Service {
	t.Helper()
	return NewService(model.NewAdminModel(db), model.NewGameModel(db), model.NewRoleModel(db))
}

// createRoleWithPermissions 建角色并挂上给定权限 id（不建 permissions 行：
// 角色→权限的解析只读 role_permissions）。
func createRoleWithPermissions(t *testing.T, db *gorm.DB, name string, permIDs ...string) model.Role {
	t.Helper()
	ctx := context.Background()
	role := model.Role{Name: name}
	require.NoError(t, db.Where("name = ?", name).FirstOrCreate(&role).Error)
	require.NoError(t, model.NewRoleModel(db).ReplacePermissions(ctx, role.ID, permIDs))
	return role
}

// seedGameEnvScope 建账号 + 游戏 + 环境绑定，并（管理员时）授予该游戏的环境范围。
// 返回 service 与 gameID。
func seedGameEnvScope(t *testing.T, username string, isAdmin bool) (*Service, *model.Admin, string) {
	t.Helper()
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	roleName := "viewer"
	if isAdmin {
		roleName = "admin"
	}
	adminID := createTestAdminWithRole(t, db, username, "password123", roleName)

	game := &model.Game{GameID: username + "-game", Name: "G", Status: "dev"}
	require.NoError(t, svcCtx.GameModel.Create(context.Background(), game))
	// game_envs 是可选环境的唯一权威来源：没有绑定就没有可选择的环境
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(context.Background(), game.GameID, "production", "test", "", ""))
	require.NoError(t, svcCtx.AdminModel.SetGameEnvScope(context.Background(), adminID, game.ID, "production"))

	admin, err := svcCtx.AdminModel.FindOne(context.Background(), adminID)
	require.NoError(t, err)
	return newPermService(t, db), admin, game.GameID
}

// ------------------------------------------------------------ 纯逻辑层

func TestSplitLogicalPermission_ReusedFromRBAC(t *testing.T) {
	// 资源轴取自 id 前缀；这是权限树能渲染出 38 条目录对应节点的前提。
	cases := []struct {
		id, wantRes, wantAct string
	}{
		{"user:read", "user", "read"},
		{"pages:write", "pages", "write"},
		{"*", "*", "*"},
		{"admin:all", "*", "*"},
		{"", "*", "*"},
		{"dashboard", "dashboard", "*"},
		{"USER:READ", "user", "read"},     // 大小写归一
		{"  user:read  ", "user", "read"}, // 空白归一
		{"user:", "user", "*"},
		{":read", "*", "read"},
	}
	for _, c := range cases {
		res, act := splitForTest(c.id)
		assert.Equal(t, c.wantRes, res, "id=%q resource", c.id)
		assert.Equal(t, c.wantAct, act, "id=%q action", c.id)
	}
}

func TestResolvePermissions_GroupsByResource(t *testing.T) {
	r := resolvePermissions([]string{
		"user:read", "user:write", "pages:read", "log:read", "user:read",
	})
	assert.Equal(t, []string{"log:read", "pages:read", "user:read", "user:write"}, r.IDs,
		"id 应去重并排序")
	require.Len(t, r.Groups, 3)
	assert.Equal(t, ProfilePermission{Resource: "log", Actions: []string{"read"}}, r.Groups[0])
	assert.Equal(t, ProfilePermission{Resource: "pages", Actions: []string{"read"}}, r.Groups[1])
	assert.Equal(t, ProfilePermission{Resource: "user", Actions: []string{"read", "write"}}, r.Groups[2])
	assert.False(t, r.FullAccess)
	assert.Equal(t, accessScoped, r.AccessLevel)
}

// 资源轴必须来自 id 前缀。若改用 permissions 表的 resource 列（存的是 module），
// 38 条目录会塌缩成 7 个值——这个用例锁住「不塌缩」。
func TestResolvePermissions_ResourceFromIDPrefixNotModule(t *testing.T) {
	// 这组 id 在真实数据里的 module 分别是 account / system，但资源轴必须保持
	// user / role / openapi_source 三个不同节点。
	r := resolvePermissions([]string{"user:read", "role:write", "openapi_source:read"})
	require.Len(t, r.Groups, 3, "必须按 id 前缀分成 3 个资源节点")
	got := []string{r.Groups[0].Resource, r.Groups[1].Resource, r.Groups[2].Resource}
	assert.Equal(t, []string{"openapi_source", "role", "user"}, got)
	for _, g := range r.Groups {
		assert.NotEqual(t, "account", g.Resource, "不得回落到 module 值")
		assert.NotEqual(t, "system", g.Resource)
	}
}

func TestResolvePermissions_WildcardSetsFullAccess(t *testing.T) {
	// admin:all 在 rbac 里被显式特判为 ("*","*")；大小写归一后同样成立
	for _, id := range []string{"*", "admin:all", "ADMIN:ALL", "  admin:all  "} {
		r := resolvePermissions([]string{"user:read", id})
		assert.True(t, r.FullAccess, "id=%q 应被识别为通配", id)
		assert.Equal(t, accessFull, r.AccessLevel)
		// 通配不得渲染成假的 "*" 资源节点
		for _, g := range r.Groups {
			assert.NotEqual(t, "*", g.Resource, "通配不得变成资源节点")
		}
	}
}

// 反例：user:* 只是「user 资源的全部操作」，**不是**全局全量。
// 把它算成 fullAccess 会让前端把所有条目渲染成「已授权」——又是一次假状态。
func TestResolvePermissions_PartialWildcardIsNotFullAccess(t *testing.T) {
	r := resolvePermissions([]string{"user:*", "log:read"})
	assert.False(t, r.FullAccess, "user:* 只覆盖单个资源，不得当成全部权限")
	assert.Equal(t, accessScoped, r.AccessLevel)
	// 但它确实要落成 user 节点下的通配操作，否则「user 全部可操作」会丢失
	require.Len(t, r.Groups, 2)
	assert.Equal(t, "log", r.Groups[0].Resource)
	assert.Equal(t, ProfilePermission{Resource: "user", Actions: []string{"*"}}, r.Groups[1])
}

// 空集合必须报 none 而不是 full，否则前端会把「无权限」显示成「全部权限」。
func TestResolvePermissions_EmptyIsNone(t *testing.T) {
	r := resolvePermissions(nil)
	assert.Empty(t, r.IDs)
	assert.Empty(t, r.Groups)
	assert.False(t, r.FullAccess)
	assert.Equal(t, accessNoAccess, r.AccessLevel)
}

func TestResolvePermissions_BlankAndDuplicateIDsDropped(t *testing.T) {
	r := resolvePermissions([]string{"", "   ", "user:read", "user:read"})
	assert.Equal(t, []string{"user:read"}, r.IDs)
}

// 单段 id（无冒号）按「整串即资源、动作为通配」处理，节点名就是 id 本身。
// 真实目录（configs/permissions.json）里 37 条带冒号 + 1 条 "*"，无冒号的只有
// 通配本身；这里锁住的是遇到遗留单段 id 时不至于被丢弃或错切成两段。
func TestResolvePermissions_SingleSegmentIDBecomesItsOwnResource(t *testing.T) {
	r := resolvePermissions([]string{"legacy.flag"})
	require.Len(t, r.Groups, 1)
	assert.Equal(t, "legacy.flag", r.Groups[0].Resource, "不得按 '.' 切分")
	assert.Equal(t, []string{"*"}, r.Groups[0].Actions)
	assert.Equal(t, accessScoped, r.AccessLevel, "单段 id 不是通配（只有纯 * 才是）")
}

func TestPerGamePermissions_FullAccessReportsWildcard(t *testing.T) {
	got := perGamePermissions(resolvePermissions([]string{"*", "admin:all"}))
	assert.Equal(t, []string{"*"}, got, "持通配时每个游戏应报「全部权限」")
}

func TestPerGamePermissions_ScopedReportsRealIDs(t *testing.T) {
	got := perGamePermissions(resolvePermissions([]string{"user:read", "pages:read"}))
	assert.Equal(t, []string{"pages:read", "user:read"}, got)
}

// 调用方改动返回值不得污染底层切片。
func TestPerGamePermissions_ReturnsCopy(t *testing.T) {
	resolved := resolvePermissions([]string{"user:read"})
	got := perGamePermissions(resolved)
	got[0] = "tampered"
	assert.Equal(t, []string{"user:read"}, resolved.IDs)
}

// ------------------------------------------------------- 角色 / 权限收集

// 核心回归（BUG-019）：角色名不得混进 PermissionIDs。
func TestCollectUserPermissionSet_KeepsRoleNamesOut(t *testing.T) {
	db := setupTestDB(t)
	svc := newPermService(t, db)
	role := createRoleWithPermissions(t, db, "ops", "log:read", "log:write")

	set := collectUserPermissionSet(context.Background(), svc, []model.Role{role})
	assert.Equal(t, []string{"ops"}, set.Roles)
	assert.Equal(t, []string{"log:read", "log:write"}, set.PermissionIDs)
	assert.NotContains(t, set.PermissionIDs, "ops",
		"角色名曾被 appendPermission(role) 塞进 permissionIDs，前端会当成不存在的权限")
	assert.False(t, set.IsAdmin)
	require.Len(t, set.Resolved.Groups, 1)
	assert.Equal(t, "log", set.Resolved.Groups[0].Resource)
}

func TestCollectUserPermissionSet_AdminGetsWildcard(t *testing.T) {
	db := setupTestDB(t)
	svc := newPermService(t, db)
	role := createRoleWithPermissions(t, db, "admin", "user:read")

	set := collectUserPermissionSet(context.Background(), svc, []model.Role{role})
	assert.True(t, set.IsAdmin)
	assert.True(t, set.Resolved.FullAccess)
	assert.Equal(t, accessFull, set.Resolved.AccessLevel)
	// 通配 id 本身可以出现在列表里（configs/permissions.json 里就有这两条），
	// 但角色名不行
	assert.Contains(t, set.PermissionIDs, "*")
	assert.Contains(t, set.PermissionIDs, "admin:all")
	assert.NotContains(t, set.PermissionIDs, "admin")
}

func TestCollectUserPermissionSet_SuperAdminCaseInsensitive(t *testing.T) {
	db := setupTestDB(t)
	svc := newPermService(t, db)
	role := createRoleWithPermissions(t, db, "Super_Admin")

	set := collectUserPermissionSet(context.Background(), svc, []model.Role{role})
	assert.True(t, set.IsAdmin, "角色名应大小写无关")
	assert.Equal(t, []string{"Super_Admin"}, set.Roles, "展示用角色名保留原样")
}

// 多个角色的权限必须合并（并集），这正是 RBAC 的预期语义。
func TestCollectUserPermissionSet_MergesMultipleRoles(t *testing.T) {
	db := setupTestDB(t)
	svc := newPermService(t, db)
	r1 := createRoleWithPermissions(t, db, "viewer", "user:read")
	r2 := createRoleWithPermissions(t, db, "auditor", "log:read", "user:read")

	set := collectUserPermissionSet(context.Background(), svc, []model.Role{r1, r2})
	assert.Equal(t, []string{"auditor", "viewer"}, set.Roles, "角色名排序且去重")
	assert.Equal(t, []string{"log:read", "user:read"}, set.PermissionIDs, "权限取并集并去重")
}

// 输入里出现同名角色时，展示用的角色名不得重复。
func TestCollectUserPermissionSet_DedupesRoleNames(t *testing.T) {
	db := setupTestDB(t)
	svc := newPermService(t, db)
	r := createRoleWithPermissions(t, db, "viewer", "user:read")

	set := collectUserPermissionSet(context.Background(), svc, []model.Role{r, r, r})
	assert.Equal(t, []string{"viewer"}, set.Roles, "同名角色只出现一次")
	assert.Equal(t, []string{"user:read"}, set.PermissionIDs, "重复角色不得放大权限列表")
}

func TestCollectUserPermissionSet_NoRolesIsNone(t *testing.T) {
	db := setupTestDB(t)
	svc := newPermService(t, db)
	set := collectUserPermissionSet(context.Background(), svc, nil)
	assert.Empty(t, set.Roles)
	assert.Empty(t, set.PermissionIDs)
	assert.False(t, set.IsAdmin)
	assert.Equal(t, accessNoAccess, set.Resolved.AccessLevel)
}

// 有角色但一个权限都没挂：PermissionIDs 为空，级别必须是 none。
func TestCollectUserPermissionSet_RoleWithoutPermissions(t *testing.T) {
	db := setupTestDB(t)
	svc := newPermService(t, db)
	role := createRoleWithPermissions(t, db, "guest")

	set := collectUserPermissionSet(context.Background(), svc, []model.Role{role})
	assert.Equal(t, []string{"guest"}, set.Roles)
	assert.Empty(t, set.PermissionIDs)
	assert.Equal(t, accessNoAccess, set.Resolved.AccessLevel)
}

// ------------------------------------------------------------ 端点契约

// BUG-018：admin 的 /profile/games 每行 permissions 曾恒为空数组。
func TestGetUserGames_AdminSeesFullAccessPerGame(t *testing.T) {
	svc, _, gameID := seedGameEnvScope(t, "gameadmin", true)

	resp, err := svc.GetUserGames(context.Background(), "gameadmin")
	require.NoError(t, err)
	require.NotEmpty(t, resp.Games, "admin 应看到全部游戏")

	var target *ProfileGame
	for i := range resp.Games {
		if resp.Games[i].GameId == gameID {
			target = &resp.Games[i]
		}
	}
	require.NotNil(t, target, "新建的游戏必须出现在列表里")
	assert.Equal(t, []string{"*"}, target.Permissions,
		"admin 的游戏权限应是「全部权限」，而不是空数组")
	assert.Equal(t, accessFull, target.AccessLevel)
	assert.Equal(t, accessScopeRole, target.PermissionScope)
}

// 普通用户无显式权限：必须报 none + 空数组（可解释），不是空白。
func TestGetUserGames_NonAdminWithoutPermissionsIsNone(t *testing.T) {
	svc, _, gameID := seedGameEnvScope(t, "plainuser", false)

	resp, err := svc.GetUserGames(context.Background(), "plainuser")
	require.NoError(t, err)
	var target *ProfileGame
	for i := range resp.Games {
		if resp.Games[i].GameId == gameID {
			target = &resp.Games[i]
		}
	}
	require.NotNil(t, target, "已授权环境的游戏应出现在列表里")
	assert.Empty(t, target.Permissions)
	assert.Equal(t, accessNoAccess, target.AccessLevel,
		"空权限必须有可解释的级别，前端据此显示说明而不是留白")
	assert.Equal(t, accessScopeRole, target.PermissionScope)
}

// BUG-019：/profile/permissions 的 permissions[] 曾是 resource:"role" 的编造数据。
func TestGetPermissions_ReturnsRealResourceGroups(t *testing.T) {
	db := setupTestDB(t)
	adminID := createTestAdminWithRole(t, db, "permview", "password123", "placeholder")
	createRoleWithPermissions(t, db, "permview_role", "user:read", "pages:write")
	assignRole(t, db, adminID, "permview_role")

	resp, err := newPermService(t, db).GetPermissions(context.Background(), "permview")
	require.NoError(t, err)
	require.NotEmpty(t, resp.Permissions)

	resources := map[string]bool{}
	for _, p := range resp.Permissions {
		assert.NotEqual(t, "role", p.Resource,
			"resource 恒为字面量 \"role\" 是旧实现的编造数据")
		assert.NotEmpty(t, p.Actions)
		resources[p.Resource] = true
	}
	assert.True(t, resources["user"] && resources["pages"],
		"应按真实 id 前缀分组，得到 %v", resources)
	assert.Equal(t, accessScoped, resp.AccessLevel)
	assert.False(t, resp.FullAccess)
	assert.Equal(t, accessScopeRole, resp.PermissionScope)
	assert.Equal(t, []string{"pages:write", "user:read"}, resp.PermissionIDs)
	assert.Contains(t, resp.Roles, "permview_role")
	assert.NotContains(t, resp.PermissionIDs, "permview_role", "角色名不得进 permissionIDs")
}

func TestGetPermissions_AdminReportsFullAccess(t *testing.T) {
	db := setupTestDB(t)
	adminID := createTestAdminWithRole(t, db, "permadmin", "password123", "placeholder")
	createRoleWithPermissions(t, db, "admin", "user:read")
	assignRole(t, db, adminID, "admin")

	resp, err := newPermService(t, db).GetPermissions(context.Background(), "permadmin")
	require.NoError(t, err)
	assert.True(t, resp.Admin)
	assert.True(t, resp.FullAccess)
	assert.Equal(t, accessFull, resp.AccessLevel)
	// admin 的 groups 仍应保留具体资源节点，不能因为通配就整组为空
	assert.NotEmpty(t, resp.Permissions, "admin 也应看到具体资源分组")
}

// 树形渲染需要知道「哪个角色授予了哪些权限」；只有并集时无法回答。
func TestGetPermissions_RoleGrantsAttributeIDs(t *testing.T) {
	db := setupTestDB(t)
	adminID := createPlainAdmin(t, db, "treeuser").ID
	createRoleWithPermissions(t, db, "reader", "user:read", "pages:read")
	createRoleWithPermissions(t, db, "writer", "pages:write")
	assignRole(t, db, adminID, "reader")
	assignRole(t, db, adminID, "writer")

	resp, err := newPermService(t, db).GetPermissions(context.Background(), "treeuser")
	require.NoError(t, err)
	require.Len(t, resp.RolePermissions, 2, "两个角色都应出现在树里")

	byRole := map[string][]string{}
	for _, g := range resp.RolePermissions {
		byRole[g.Role] = g.PermissionIDs
	}
	assert.Equal(t, []string{"pages:read", "user:read"}, byRole["reader"])
	assert.Equal(t, []string{"pages:write"}, byRole["writer"])
	// 并集仍应包含全部
	assert.Equal(t, []string{"pages:read", "pages:write", "user:read"}, resp.PermissionIDs)
}

// 有角色但零权限时也必须出现在树里，否则用户在树上找不到自己的角色。
func TestGetPermissions_RoleGrantsIncludeEmptyRoles(t *testing.T) {
	db := setupTestDB(t)
	adminID := createPlainAdmin(t, db, "treeempty").ID
	createRoleWithPermissions(t, db, "guest")
	assignRole(t, db, adminID, "guest")

	resp, err := newPermService(t, db).GetPermissions(context.Background(), "treeempty")
	require.NoError(t, err)
	require.Len(t, resp.RolePermissions, 1)
	assert.Equal(t, "guest", resp.RolePermissions[0].Role)
	assert.Empty(t, resp.RolePermissions[0].PermissionIDs)
}
