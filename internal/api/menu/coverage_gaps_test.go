package menu

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptrString(v string) *string { return &v }

// 权限拒绝矩阵：各写路径在无对应权限时统一拒绝。
func TestMenuServicePermissionDenied(t *testing.T) {
	service, ctx := newMenuTestService(t) // 无任何权限

	_, err := service.List(ctx)
	assert.Error(t, err, "List 无 menu:read 应拒绝")
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "x", Labels: menuLabels()})
	assert.Error(t, err, "Create 无 menu:create 应拒绝")
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: "1"})
	assert.Error(t, err, "Update 无 menu:update 应拒绝")
	err = service.Delete(ctx, &UpdateMenuRequest{ID: "1"})
	assert.Error(t, err, "Delete 无 menu:delete 应拒绝")
	_, err = service.UpdateSort(ctx, &SortMenuRequest{ID: "1"})
	assert.Error(t, err, "UpdateSort 无 menu:sort 应拒绝")
}

// scope 缺失：所有入口统一要求 X-Game-ID；缺 env 单独提示。
// 权限检查先于 scope 检查，上下文须带已授权登录用户（newMenuTestService
// 固定创建 username=menu_tester）但剥离 game scope。
func TestMenuServiceScopeMissing(t *testing.T) {
	service, _ := newMenuTestService(t, "admin:all")
	noScope := context.WithValue(context.Background(), "username", "menu_tester")

	_, err := service.List(noScope)
	assert.ErrorContains(t, err, "X-Game-ID")
	_, err = service.Create(noScope, &CreateMenuRequest{MenuKey: "x", Labels: menuLabels()})
	assert.ErrorContains(t, err, "X-Game-ID")
	_, err = service.Update(noScope, &UpdateMenuRequest{ID: "1"})
	assert.ErrorContains(t, err, "X-Game-ID")
	err = service.Delete(noScope, &UpdateMenuRequest{ID: "1"})
	assert.ErrorContains(t, err, "X-Game-ID")
	_, err = service.UpdateSort(noScope, &SortMenuRequest{ID: "1"})
	assert.ErrorContains(t, err, "X-Game-ID")
	_, err = AccessibleTree(noScope, service.svcCtx)
	assert.ErrorContains(t, err, "X-Game-ID")
	_, err = service.Accessible(noScope)
	assert.ErrorContains(t, err, "X-Game-ID")

	// 有 gameId 缺 env：requireScope 的第二个守卫
	noEnv := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: ""})
	noEnv = context.WithValue(noEnv, "username", "menu_tester")
	_, err = service.List(noEnv)
	assert.ErrorContains(t, err, "X-Env")
}

// 更新/删除路径的参数与状态校验：非法 ID、非法 key、key 冲突、labels 非法、目标缺失。
func TestMenuServiceUpdateValidation(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:update")

	first, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "first", Labels: menuLabels()})
	require.NoError(t, err)
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "second", Labels: menuLabels()})
	require.NoError(t, err)

	// 非法数字 ID
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: "abc"})
	assert.Error(t, err)
	err = service.Delete(ctx, &UpdateMenuRequest{ID: "abc"})
	assert.Error(t, err)
	_, err = service.UpdateSort(ctx, &SortMenuRequest{ID: "abc", SortOrder: 1})
	assert.Error(t, err)

	// 更新为非法 menuKey
	badKey := "bad key!"
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(int64(first.ID)), MenuKey: &badKey})
	assert.Error(t, err)

	// 更新为已存在 key → 冲突
	dup := "second"
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(int64(first.ID)), MenuKey: &dup})
	assert.Error(t, err)

	// 更新 labels 为全空 → 校验失败
	emptyLabels := spec.LocalizedText{"zh-CN": ""}
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(int64(first.ID)), Labels: &emptyLabels})
	assert.Error(t, err)

	// 目标菜单不存在
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: "424242"})
	assert.Error(t, err)
	_, err = service.UpdateSort(ctx, &SortMenuRequest{ID: "424242", SortOrder: 3})
	assert.Error(t, err)
	err = service.Delete(ctx, &UpdateMenuRequest{ID: "424242"})
	assert.Error(t, err)
}

// 部分更新指针字段：permission/sortOrder/isVisible 单独下发即生效。
func TestMenuServiceUpdatePartialPointerFields(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:update")

	created, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "partial", Labels: menuLabels()})
	require.NoError(t, err)

	permission := "secret:read"
	sortOrder := 9
	invisible := false
	updated, err := service.Update(ctx, &UpdateMenuRequest{
		ID:         formatID(int64(created.ID)),
		Permission: &permission,
		SortOrder:  &sortOrder,
		IsVisible:  &invisible,
	})
	require.NoError(t, err)
	assert.Equal(t, "secret:read", updated.Permission)
	assert.EqualValues(t, 9, updated.SortOrder)
	assert.False(t, updated.IsVisible)
}

// 父菜单校验：不存在 / 自身 / 环路拒绝、无环移动成功、链上孤儿父记录容忍。
func TestMenuServiceParentResolution(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:update")
	const gameID, env = "demo-game", "development"

	root, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "root", Labels: menuLabels()})
	require.NoError(t, err)
	child, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "child", ParentID: &root.ID, Labels: menuLabels()})
	require.NoError(t, err)
	other, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "other", Labels: menuLabels()})
	require.NoError(t, err)

	// 父不存在
	missing := int64(424242)
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "orphan", ParentID: &missing, Labels: menuLabels()})
	assert.ErrorContains(t, err, "父菜单不存在")
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(int64(other.ID)), ParentID: &missing})
	assert.ErrorContains(t, err, "父菜单不存在")

	// 父是自身
	selfID := int64(root.ID)
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(int64(root.ID)), ParentID: &selfID})
	assert.ErrorContains(t, err, "父菜单不能是自身")

	// 环路：把祖先移到自己的后代下
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(int64(root.ID)), ParentID: &child.ID})
	assert.ErrorContains(t, err, "子级")

	// 无环移动：other 挂到 child 下，向上走到 root 不成环 → 成功
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(int64(other.ID)), ParentID: &child.ID})
	require.NoError(t, err)

	// 链上父记录缺失（悬挂 parentId）：直接硬删中间父制造 dangling 链，
	// 环检测向上走到缺失行时视为无环（孤儿容忍）。
	dangParent, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "dang-parent", Labels: menuLabels()})
	require.NoError(t, err)
	dangChild, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "dang-child", ParentID: &dangParent.ID, Labels: menuLabels()})
	require.NoError(t, err)
	require.NoError(t, service.menuModel().Delete(ctx, gameID, env, uint(dangParent.ID)))
	mover, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "mover", Labels: menuLabels()})
	require.NoError(t, err)
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(int64(mover.ID)), ParentID: &dangChild.ID})
	require.NoError(t, err, "环检测走到缺失父记录应视为无环")
}

// DB 故障注入（关闭底层连接）：各读写的 error 分支统一返回错误而非 panic。
func TestMenuServiceDBFailureBranches(t *testing.T) {
	service, ctx := newMenuTestService(t, "admin:all", "menu:create", "menu:read", "menu:update", "menu:delete", "menu:sort")
	const gameID, env = "demo-game", "development"

	created, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "seedroot", Labels: menuLabels()})
	require.NoError(t, err)

	sqlDB, err := service.svcCtx.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = service.List(ctx)
	assert.Error(t, err)
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "another", Labels: menuLabels()})
	assert.Error(t, err)
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(int64(created.ID)), Icon: ptrString("X")})
	assert.Error(t, err)
	_, err = service.UpdateSort(ctx, &SortMenuRequest{ID: formatID(int64(created.ID)), SortOrder: 2})
	assert.Error(t, err)
	err = service.Delete(ctx, &UpdateMenuRequest{ID: formatID(int64(created.ID))})
	assert.Error(t, err)
	_, err = AccessibleTree(ctx, service.svcCtx)
	assert.Error(t, err)
}
