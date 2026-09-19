package menu

import (
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// service.go 残余错误分支补齐。手法沿用仓库既有两类：
//   1. 关闭底层连接（首个 DB 调用即失败）；
//   2. gorm 回调按表名/变量定点注错（alertrule_v8_test.go 先例），带调用
//      计数器以命中「第 N 次调用才失败」的级联路径。
//
// 论证为不可达、不在此覆盖的 4 个死分支：
//   - SetLabels error ×2（service.go:108/171）：map[string]string 的
//     json.Marshal 恒成功，模型注释已声明恒返回 nil；
//   - filterAccessibleTree service.go:380 的 !check(parent)：递归不变式
//     保证 check(child)=true 时父链已验证为可达（父不存在走孤儿提升分支），
//     该二次校验为防御性恒真，无可构造输入使其为假。

// registerMenuQueryError 注册 menu_items 查询注错：needle 非空时仅当查询
// 变量含该字符串才注错（FindByScopeAndKey 按 key 查询），skip=N 表示前 N
// 次放行、第 N+1 次起注错。
func registerMenuQueryError(t *testing.T, db *gorm.DB, name, needle string, skip int) {
	t.Helper()
	var calls int
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "menu_items" {
			return
		}
		if needle != "" {
			found := false
			for _, v := range tx.Statement.Vars {
				if s, ok := v.(string); ok && s == needle {
					found = true
					break
				}
			}
			if !found {
				return
			}
		}
		calls++
		if calls > skip {
			_ = tx.AddError(errors.New("injected menu query failure"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(name) })
}

func registerMenuWriteError(t *testing.T, db *gorm.DB, name, op string, skip int) {
	t.Helper()
	var calls int
	inject := func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "menu_items" {
			return
		}
		calls++
		if calls > skip {
			_ = tx.AddError(errors.New("injected menu write failure"))
		}
	}
	switch op {
	case "create":
		require.NoError(t, db.Callback().Create().Before("gorm:create").Register(name, inject))
	case "update":
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register(name, inject))
	case "delete":
		require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register(name, inject))
	}
	t.Cleanup(func() {
		_ = db.Callback().Create().Remove(name)
		_ = db.Callback().Update().Remove(name)
		_ = db.Callback().Delete().Remove(name)
	})
}

func TestMenuList_ListByScopeError(t *testing.T) {
	service, ctx := newMenuTestService(t, "admin:all")
	registerMenuQueryError(t, service.svcCtx.DB, "gap2:list", "", 0)

	_, err := service.List(ctx)
	assert.Error(t, err)
}

func TestMenuCreate_KeyLookupError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create")
	registerMenuQueryError(t, service.svcCtx.DB, "gap2:keylookup", "clashkey", 0)

	_, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "clashkey", Labels: menuLabels()})
	assert.Error(t, err, "key 查重查询故障应返回错误而非误判冲突")
}

func TestMenuUpdate_KeyLookupError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:update")
	first, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "origin", Labels: menuLabels()})
	require.NoError(t, err)

	newKey := "renamed"
	registerMenuQueryError(t, service.svcCtx.DB, "gap2:rename", "renamed", 0)

	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(int64(first.ID)), MenuKey: &newKey})
	assert.Error(t, err)
}

func TestMenuCreate_InsertError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create")
	registerMenuWriteError(t, service.svcCtx.DB, "gap2:insert", "create", 0)

	_, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "doomed", Labels: menuLabels()})
	assert.Error(t, err)
}

func TestMenuUpdate_FindError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:update")
	first, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "target", Labels: menuLabels()})
	require.NoError(t, err)

	registerMenuQueryError(t, service.svcCtx.DB, "gap2:ufind", "", 0)

	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(int64(first.ID)), Icon: ptrString("X")})
	assert.Error(t, err)
}

func TestMenuUpdate_SaveError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:update")
	first, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "saver", Labels: menuLabels()})
	require.NoError(t, err)

	registerMenuWriteError(t, service.svcCtx.DB, "gap2:usave", "update", 0)

	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(int64(first.ID)), Icon: ptrString("X")})
	assert.Error(t, err)
}

func TestMenuDelete_ParseIDError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:delete")
	err := service.Delete(ctx, &UpdateMenuRequest{ID: "not-a-number"})
	assert.Error(t, err)
}

func TestMenuDelete_FindError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:delete")
	first, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "gone", Labels: menuLabels()})
	require.NoError(t, err)

	registerMenuQueryError(t, service.svcCtx.DB, "gap2:dfind", "", 0)

	err = service.Delete(ctx, &UpdateMenuRequest{ID: formatID(int64(first.ID))})
	assert.Error(t, err)
}

func TestMenuDelete_ListScopeError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:delete")
	first, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "cascade", Labels: menuLabels()})
	require.NoError(t, err)

	// 第 1 次 menu_items 查询（FindByID）放行，第 2 次（ListByScope）注错。
	registerMenuQueryError(t, service.svcCtx.DB, "gap2:dlist", "", 1)

	err = service.Delete(ctx, &UpdateMenuRequest{ID: formatID(int64(first.ID))})
	assert.Error(t, err)
}

func TestMenuDelete_ClearPageRefsError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:delete")
	first, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "linked", Labels: menuLabels()})
	require.NoError(t, err)

	// PageSpecModel 独立挂到已关闭的第二个库：菜单库健康、页面引用清理必败。
	pdb, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	psqlDB, err := pdb.DB()
	require.NoError(t, err)
	require.NoError(t, psqlDB.Close())
	service.svcCtx.PageSpecModel = model.NewPageSpecModel(pdb)

	err = service.Delete(ctx, &UpdateMenuRequest{ID: formatID(int64(first.ID))})
	assert.Error(t, err)
}

func TestMenuDelete_MainRowError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:delete")
	first, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "rootdel", Labels: menuLabels()})
	require.NoError(t, err)

	registerMenuWriteError(t, service.svcCtx.DB, "gap2:ddel", "delete", 0)

	err = service.Delete(ctx, &UpdateMenuRequest{ID: formatID(int64(first.ID))})
	assert.Error(t, err)
}

func TestMenuDelete_ChildRowError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:delete")
	root, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "rootc", Labels: menuLabels()})
	require.NoError(t, err)
	_, err = service.Create(ctx, &CreateMenuRequest{MenuKey: "childc", ParentID: &root.ID, Labels: menuLabels()})
	require.NoError(t, err)

	// 第 1 次 delete（主行）放行，第 2 次（级联子行）注错。
	registerMenuWriteError(t, service.svcCtx.DB, "gap2:dchild", "delete", 1)

	err = service.Delete(ctx, &UpdateMenuRequest{ID: formatID(int64(root.ID))})
	assert.Error(t, err)
}

func TestMenuUpdateSort_FindError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:sort")
	first, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "sorted", Labels: menuLabels()})
	require.NoError(t, err)

	registerMenuQueryError(t, service.svcCtx.DB, "gap2:sfind", "", 0)

	_, err = service.UpdateSort(ctx, &SortMenuRequest{ID: formatID(int64(first.ID)), SortOrder: 2})
	assert.Error(t, err)
}

func TestMenuUpdateSort_UpdateOrderError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:sort")
	first, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "sorted2", Labels: menuLabels()})
	require.NoError(t, err)

	registerMenuWriteError(t, service.svcCtx.DB, "gap2:sorder", "update", 0)

	_, err = service.UpdateSort(ctx, &SortMenuRequest{ID: formatID(int64(first.ID)), SortOrder: 3})
	assert.Error(t, err)
}

func TestMenuUpdateSort_ReloadError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:sort")
	first, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "sorted3", Labels: menuLabels()})
	require.NoError(t, err)

	// 第 1 次 menu_items 查询（排序前校验）放行，第 2 次（写后重读）注错。
	registerMenuQueryError(t, service.svcCtx.DB, "gap2:sreload", "", 1)

	_, err = service.UpdateSort(ctx, &SortMenuRequest{ID: formatID(int64(first.ID)), SortOrder: 4})
	assert.Error(t, err)
}

func TestMenuAccessibleTree_AdminLoadError(t *testing.T) {
	service, ctx := newMenuTestService(t, "admin:all")
	// AdminModel 未初始化 → LoadCurrentAdmin 报错。
	bare := &svc.ServiceContext{DB: service.svcCtx.DB, MenuModel: service.svcCtx.MenuModel}
	_, err := AccessibleTree(ctx, bare)
	assert.Error(t, err)
}

func TestMenuAccessibleTree_PermissionLookupError(t *testing.T) {
	service, ctx := newMenuTestService(t, "admin:all")
	db := service.svcCtx.DB
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("gap2:permids", func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "role_permissions" {
			return
		}
		_ = tx.AddError(errors.New("injected role permission failure"))
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove("gap2:permids") })

	_, err := AccessibleTree(ctx, service.svcCtx)
	assert.Error(t, err)
}

func TestMenuAccessibleTree_AdminRoleSeesAll(t *testing.T) {
	service, ctx := newMenuTestService(t)
	// 角色名命中 admin → HasAdminRole 分支为 permIDs 追加通配。
	adminRole := model.Role{Name: "admin", Description: "builtin admin"}
	require.NoError(t, service.svcCtx.DB.Create(&adminRole).Error)
	var tester model.Admin
	require.NoError(t, service.svcCtx.DB.Where("username = ?", "menu_tester").First(&tester).Error)
	require.NoError(t, service.svcCtx.DB.Create(&model.AdminRole{AdminID: tester.ID, RoleID: adminRole.ID}).Error)

	resp, err := service.Accessible(ctx)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestMenuAccessibleTree_ListError(t *testing.T) {
	service, ctx := newMenuTestService(t, "admin:all")
	registerMenuQueryError(t, service.svcCtx.DB, "gap2:alist", "", 0)

	_, err := AccessibleTree(ctx, service.svcCtx)
	assert.Error(t, err)
}

// filterAccessibleTree 纯函数分支：孤儿节点（父记录缺失）提升为根；父被
// 权限过滤掉时其子树整体隐藏。
func TestFilterAccessibleTree_OrphanAndGatedParent(t *testing.T) {
	secret := "secret:read"
	visibleRoot := model.MenuItem{MenuKey: "gated", Permission: secret, IsVisible: true}
	child := model.MenuItem{MenuKey: "gated-child", IsVisible: true}
	visibleRoot.ID = 1
	child.ID = 2
	missing := uint(424242)
	orphan := model.MenuItem{MenuKey: "orphan", IsVisible: true}
	orphan.ID = 3
	child.ParentID = &visibleRoot.ID
	orphan.ParentID = &missing

	items := []model.MenuItem{visibleRoot, child, orphan}
	// 不持 secret:read → gated 子树整体隐藏；orphan 的父不存在 → 提升为根。
	roots := filterAccessibleTree(items, []string{"other:read"})
	require.Len(t, roots, 1)
	assert.EqualValues(t, 3, roots[0].ID, "孤儿节点应提升为根")
	assert.Empty(t, roots[0].Children)

	// 持有通配 → gated 树可见（子挂接正确），孤儿仍提升为根。
	roots = filterAccessibleTree(items, []string{"*"})
	require.Len(t, roots, 2)
	assert.EqualValues(t, 1, roots[0].ID)
	require.Len(t, roots[0].Children, 1)
	assert.EqualValues(t, 2, roots[0].Children[0].ID)
	assert.EqualValues(t, 3, roots[1].ID)
}

func TestMenuCreate_ResolveParentFindError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create")

	// 第 1 次 menu_items 查询（key 查重）放行，第 2 次（父存在性校验）注错。
	registerMenuQueryError(t, service.svcCtx.DB, "gap2:parent", "", 1)
	missing := int64(424242)

	_, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "withparent", ParentID: &missing, Labels: menuLabels()})
	assert.Error(t, err, "父存在性查询故障应返回错误而非「父菜单不存在」")
}

func TestMenuUpdate_EnsureNoCycleFindError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:update")
	root, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "cyc-root", Labels: menuLabels()})
	require.NoError(t, err)
	child, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "cyc-child", ParentID: &root.ID, Labels: menuLabels()})
	require.NoError(t, err)

	// Update 流程 menu_items 查询次序：FindByID(1) → resolveParentID(2) →
	// ensureNoCycle(3)。前两次放行、第三次注错。
	registerMenuQueryError(t, service.svcCtx.DB, "gap2:cycle", "", 2)

	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(int64(root.ID)), ParentID: &child.ID})
	assert.Error(t, err)
}
