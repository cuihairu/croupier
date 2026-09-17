package menu

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newMenuTestService(t *testing.T, permissions ...string) (*Service, context.Context) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	admin := model.Admin{Username: "menu_tester", Status: 1, PasswordHash: "test"}
	require.NoError(t, db.Create(&admin).Error)

	role := model.Role{Name: "menu_tester_role", Description: "menu tester"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
	for _, permissionID := range permissions {
		grantMenuPermission(t, db, role.ID, permissionID)
	}

	svcCtx := &svc.ServiceContext{
		DB:              db,
		AdminModel:      model.NewAdminModel(db),
		RoleModel:       model.NewRoleModel(db),
		PermissionModel: model.NewPermissionModel(db),
		MenuModel:       model.NewMenuItemModel(db),
		PageSpecModel:   model.NewPageSpecModel(db),
	}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	ctx = context.WithValue(ctx, "username", admin.Username)
	return NewService(svcCtx), ctx
}

// newViewerContext returns a context for an additional admin with no roles
// (no permissions at all), sharing the same backend state.
func newViewerContext(t *testing.T, svcCtx *svc.ServiceContext, gameID, env string) context.Context {
	t.Helper()

	viewer := model.Admin{Username: "menu_viewer", Status: 1, PasswordHash: "test"}
	require.NoError(t, svcCtx.DB.Create(&viewer).Error)
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: gameID, Env: env})
	return context.WithValue(ctx, "username", viewer.Username)
}

func grantMenuPermission(t *testing.T, db *gorm.DB, roleID uint, permissionID string) {
	t.Helper()

	permissionID = strings.TrimSpace(permissionID)
	if permissionID == "" {
		return
	}
	parts := strings.SplitN(permissionID, ":", 2)
	action := "*"
	if len(parts) == 2 {
		action = parts[1]
	}
	permission := model.Permission{
		ID:       permissionID,
		Name:     permissionID,
		Resource: parts[0],
		Action:   action,
		Category: "dashboard",
	}
	require.NoError(t, db.Where("id = ?", permission.ID).FirstOrCreate(&permission).Error)
	require.NoError(t, db.Create(&model.RolePermission{RoleID: roleID, PermissionID: permissionID}).Error)
}

func menuLabels() spec.LocalizedText {
	return spec.LocalizedText{"zh-CN": "资源管理", "en-US": "Resource"}
}

func TestMenuCreateAndListTree(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:read")

	created, err := service.Create(ctx, &CreateMenuRequest{
		MenuKey:   "resource",
		Labels:    menuLabels(),
		Icon:      "DatabaseOutlined",
		SortOrder: ptrInt(1),
	})
	require.NoError(t, err)
	assert.Equal(t, "resource", created.MenuKey)
	assert.Equal(t, map[string]string{"zh-CN": "资源管理", "en-US": "Resource"}, map[string]string(created.Labels))
	assert.True(t, created.IsVisible, "IsVisible default true")
	assert.Nil(t, created.ParentID)
	assert.Empty(t, created.Children)

	child, err := service.Create(ctx, &CreateMenuRequest{
		MenuKey:  "player",
		ParentID: &created.ID,
		Labels:   spec.LocalizedText{"zh-CN": "玩家管理"},
	})
	require.NoError(t, err)
	require.NotNil(t, child.ParentID)
	assert.Equal(t, created.ID, *child.ParentID)

	resp, err := service.List(ctx)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "resource", resp.Items[0].MenuKey)
	require.Len(t, resp.Items[0].Children, 1)
	assert.Equal(t, "player", resp.Items[0].Children[0].MenuKey)
}

func TestMenuCreateValidation(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create")

	// 非法 key
	_, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "1bad key", Labels: menuLabels()})
	assert.Error(t, err)
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "", Labels: menuLabels()})
	assert.Error(t, err)

	// labels 全空
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "resource"})
	assert.Error(t, err)

	// 合法创建
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "resource", Labels: menuLabels()})
	require.NoError(t, err)

	// 重复 key → conflict
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "resource", Labels: menuLabels()})
	assert.Error(t, err)

	// 父菜单不存在
	badParent := int64(99999)
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "player", ParentID: &badParent, Labels: menuLabels()})
	assert.Error(t, err)

	// IsVisible 显式 false
	hidden, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "hidden", Labels: menuLabels(), IsVisible: ptrBool(false)})
	require.NoError(t, err)
	assert.False(t, hidden.IsVisible)
}

func TestMenuUpdatePartial(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:update")

	created, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "resource", Labels: menuLabels()})
	require.NoError(t, err)

	newIcon := "TeamOutlined"
	updated, err := service.Update(ctx, &UpdateMenuRequest{ID: formatID(created.ID), Icon: &newIcon})
	require.NoError(t, err)
	assert.Equal(t, "TeamOutlined", updated.Icon)
	assert.Equal(t, "resource", updated.MenuKey, "未提供字段保持不变")
	assert.Equal(t, map[string]string{"zh-CN": "资源管理", "en-US": "Resource"}, map[string]string(updated.Labels))

	// 改 key
	newKey := "resource2"
	updated, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(created.ID), MenuKey: &newKey})
	require.NoError(t, err)
	assert.Equal(t, "resource2", updated.MenuKey)

	// 改 key 冲突
	other, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "operation", Labels: menuLabels()})
	require.NoError(t, err)
	conflictKey := "operation"
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(created.ID), MenuKey: &conflictKey})
	assert.Error(t, err)

	// 移到根（ParentID=0）
	root := int64(0)
	moved, err := service.Update(ctx, &UpdateMenuRequest{ID: formatID(other.ID), ParentID: &root})
	require.NoError(t, err)
	assert.Nil(t, moved.ParentID)

	// 菜单不存在
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: "99999"})
	assert.Error(t, err)
}

func TestMenuUpdateCycleRejected(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:update")

	root, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "resource", Labels: menuLabels()})
	require.NoError(t, err)
	child, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "player", ParentID: &root.ID, Labels: menuLabels()})
	require.NoError(t, err)
	grandchild, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "leaderboard", ParentID: &child.ID, Labels: menuLabels()})
	require.NoError(t, err)

	// 把祖先移到自己子孙下 → 环
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(root.ID), ParentID: &grandchild.ID})
	assert.Error(t, err, "moving root under its own descendant must be rejected")

	// 自环
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(root.ID), ParentID: &root.ID})
	assert.Error(t, err)
}

func TestMenuDeleteCascadesDescendants(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:delete", "menu:read")

	root, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "resource", Labels: menuLabels()})
	require.NoError(t, err)
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "player", ParentID: &root.ID, Labels: menuLabels()})
	require.NoError(t, err)
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "operation", Labels: menuLabels()})
	require.NoError(t, err)

	require.NoError(t, service.Delete(ctx, &UpdateMenuRequest{ID: formatID(root.ID)}))

	resp, err := service.List(ctx)
	require.NoError(t, err)
	keys := make([]string, 0, len(resp.Items))
	for _, item := range resp.Items {
		keys = append(keys, item.MenuKey)
	}
	assert.Equal(t, []string{"operation"}, keys, "descendants removed, unrelated kept")

	// 再删一次 → not found
	err = service.Delete(ctx, &UpdateMenuRequest{ID: formatID(root.ID)})
	assert.Error(t, err)
}

func TestMenuUpdateSort(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:sort")

	created, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "resource", Labels: menuLabels()})
	require.NoError(t, err)

	updated, err := service.UpdateSort(ctx, &SortMenuRequest{ID: formatID(created.ID), SortOrder: 42})
	require.NoError(t, err)
	assert.Equal(t, 42, updated.SortOrder)

	_, err = service.UpdateSort(ctx, &SortMenuRequest{ID: "99999", SortOrder: 1})
	assert.Error(t, err)
}

func TestMenuScopeIsolation(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:read")

	_, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "resource", Labels: menuLabels()})
	require.NoError(t, err)

	prodCtx := svc.WithGameScope(ctx, svc.GameScope{GameID: "demo-game", Env: "production"})
	resp, err := service.List(prodCtx)
	require.NoError(t, err)
	assert.Empty(t, resp.Items, "不同 env 的菜单互不可见")
}

func TestMenuPermissionDenied(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:read") // 无 create 权限

	_, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "resource", Labels: menuLabels()})
	assert.Error(t, err, "无 menu:create 权限应被拒绝")

	resp, err := service.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

func TestMenuMissingScope(t *testing.T) {
	service, _ := newMenuTestService(t, "admin:all")
	ctx := context.WithValue(context.Background(), "username", "menu_tester")

	_, err := service.List(ctx)
	assert.Error(t, err, "缺少 X-Game-ID/X-Env 应报错")
}

func TestMenuAccessiblePermissionFiltering(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "resource:read", "order:read")

	// 结构：resource(需 resource:read) → player(需 player:read，用户无)
	//                       → order(需 order:read，用户有)
	//       operation(无权限要求，所有人可见)
	//       secret(需 secret:read，用户无；隐藏分支)
	resource, err := service.Create(ctx, &CreateMenuRequest{
		MenuKey: "resource", Labels: menuLabels(), Permission: "resource:read",
	})
	require.NoError(t, err)
	_, err = service.Create(ctx, &CreateMenuRequest{
		MenuKey: "player", ParentID: &resource.ID, Labels: menuLabels(), Permission: "player:read",
	})
	require.NoError(t, err)
	_, err = service.Create(ctx, &CreateMenuRequest{
		MenuKey: "order", ParentID: &resource.ID, Labels: menuLabels(), Permission: "order:read",
	})
	require.NoError(t, err)
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "operation", Labels: menuLabels()})
	require.NoError(t, err)
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "secret", Labels: menuLabels(), Permission: "secret:read"})
	require.NoError(t, err)

	resp, err := service.Accessible(ctx)
	require.NoError(t, err)
	keys := menuTreeKeys(resp.Items)
	assert.Equal(t, []string{"resource", "operation"}, keys, "受限分支与无权限分支整体隐藏")

	// resource 下只有 order 可见（player 被 player:read 过滤）
	require.Len(t, resp.Items[0].Children, 1)
	assert.Equal(t, "order", resp.Items[0].Children[0].MenuKey)
}

func TestMenuAccessibleInheritsParentPermission(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "resource:read")

	// 父有权限要求且用户持有；子自身无权限字段 → 继承父级可见
	root, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "resource", Labels: menuLabels(), Permission: "resource:read"})
	require.NoError(t, err)
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "player", ParentID: &root.ID, Labels: menuLabels()})
	require.NoError(t, err)

	resp, err := service.Accessible(ctx)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.Len(t, resp.Items[0].Children, 1, "子菜单继承父菜单权限后可见")
	assert.Equal(t, "player", resp.Items[0].Children[0].MenuKey)
}

func TestMenuAccessibleHidesInvisible(t *testing.T) {
	service, ctx := newMenuTestService(t, "admin:all")

	root, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "resource", Labels: menuLabels()})
	require.NoError(t, err)
	child, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "player", ParentID: &root.ID, Labels: menuLabels()})
	require.NoError(t, err)

	// 根不可见 → 整支隐藏
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(root.ID), IsVisible: ptrBool(false)})
	require.NoError(t, err)
	resp, err := service.Accessible(ctx)
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	// 根可见、子不可见 → 只隐藏子
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(root.ID), IsVisible: ptrBool(true)})
	require.NoError(t, err)
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(child.ID), IsVisible: ptrBool(false)})
	require.NoError(t, err)
	resp, err = service.Accessible(ctx)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Empty(t, resp.Items[0].Children)
}

func TestMenuAccessibleAdminSeesAll(t *testing.T) {
	service, ctx := newMenuTestService(t, "admin:all")

	_, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "resource", Labels: menuLabels(), Permission: "resource:read"})
	require.NoError(t, err)

	resp, err := service.Accessible(ctx)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1, "admin 角色不受菜单权限过滤")
}

func TestMenuAccessibleNoPermissionUserSeesUngated(t *testing.T) {
	// 创建者带全部权限；查询者无任何角色权限
	service, ctx := newMenuTestService(t, "menu:create")

	_, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "gated", Labels: menuLabels(), Permission: "secret:read"})
	require.NoError(t, err)
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "open", Labels: menuLabels()})
	require.NoError(t, err)

	viewerCtx := newViewerContext(t, service.svcCtx, "demo-game", "development")
	resp, err := service.Accessible(viewerCtx)
	require.NoError(t, err)
	keys := menuTreeKeys(resp.Items)
	assert.Equal(t, []string{"open"}, keys, "无权限用户只看到无权限要求的菜单")
}

func TestMenuDeleteClearsPageReferences(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:delete")

	root, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "resource", Labels: menuLabels()})
	require.NoError(t, err)
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "player", ParentID: &root.ID, Labels: menuLabels()})
	require.NoError(t, err)

	// 两个页面分别挂到父/子菜单
	pageModel := service.svcCtx.PageSpecModel
	for _, key := range []string{"resource--player", "resource--order"} {
		page := &model.PageSpec{
			GameID:   "demo-game",
			Env:      "development",
			PageKey:  key,
			SpecJSON: `{"pageKey":"` + key + `"}`,
		}
		require.NoError(t, service.svcCtx.DB.Create(page).Error)
	}
	childMenu, err := service.menuModel().FindByScopeAndKey(ctx, "demo-game", "development", "player")
	require.NoError(t, err)
	rootMenuID := uint(root.ID)
	require.NoError(t, pageModel.UpdateMenuID(ctx, "demo-game", "development", "resource--player", &rootMenuID))
	require.NoError(t, pageModel.UpdateMenuID(ctx, "demo-game", "development", "resource--order", &childMenu.ID))

	// 删除父菜单（级联子菜单）→ 页面引用全部解除
	require.NoError(t, service.Delete(ctx, &UpdateMenuRequest{ID: formatID(root.ID)}))

	for _, key := range []string{"resource--player", "resource--order"} {
		page, err := pageModel.FindByScopeAndPageKey(ctx, "demo-game", "development", key)
		require.NoError(t, err)
		assert.Nil(t, page.MenuID, "%s 的菜单引用应被清理", key)
	}
}

func menuTreeKeys(items []*MenuDTO) []string {
	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, item.MenuKey)
	}
	return keys
}

func TestBuildMenuTreeOrphanPromotedToRoot(t *testing.T) {
	items := []model.MenuItem{
		{MenuKey: "orphan", ParentID: ptrUint(12345)},
		{MenuKey: "root"},
	}
	tree := buildMenuTree(items)
	require.Len(t, tree, 2)
}

func ptrInt(v int) *int    { return &v }
func ptrBool(v bool) *bool { return &v }
func ptrUint(v uint) *uint { return &v }

func formatID(v int64) string { return strconv.FormatInt(v, 10) }
