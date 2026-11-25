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

// Filling in the remaining error branches of service.go. The approach follows
// the two kinds already used in the repo:
//   1. Close the underlying connection (the first DB call fails);
//   2. Inject errors at a fixed point in a gorm callback by table name or a
//      variable (precedent: alertrule_v8_test.go), with a call counter to hit
//      cascading paths where "only the Nth call fails".
//
// The 4 dead branches argued unreachable and not covered here:
//   - SetLabels error ×2 (service.go:108/171): json.Marshal of
//     map[string]string always succeeds, and the model comment already
//     states it always returns nil;
//   - the !check(parent) at filterAccessibleTree service.go:380: the
//     recursion invariant guarantees the parent chain is already verified as
//     reachable when check(child)=true (a missing parent takes the orphan
//     promotion branch), so this second check is defensively always true and
//     no constructible input makes it false.

// registerMenuQueryError registers a menu_items query error injection: when
// needle is non-empty, errors are injected only when a query variable contains
// that string (FindByScopeAndKey queries by key); skip=N means the first N
// calls pass through and errors start at call N+1.
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

	// The 1st menu_items query (FindByID) passes, the 2nd (ListByScope) is injected with an error.
	registerMenuQueryError(t, service.svcCtx.DB, "gap2:dlist", "", 1)

	err = service.Delete(ctx, &UpdateMenuRequest{ID: formatID(int64(first.ID))})
	assert.Error(t, err)
}

func TestMenuDelete_ClearPageRefsError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:delete")
	first, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "linked", Labels: menuLabels()})
	require.NoError(t, err)

	// PageSpecModel is attached separately to a second, already-closed DB: the
	// menu DB is healthy, but clearing page references must fail.
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

	// The 1st delete (main row) passes, the 2nd (cascaded child row) is injected with an error.
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

	// The 1st menu_items query (pre-sort validation) passes, the 2nd (re-read
	// after the write) is injected with an error.
	registerMenuQueryError(t, service.svcCtx.DB, "gap2:sreload", "", 1)

	_, err = service.UpdateSort(ctx, &SortMenuRequest{ID: formatID(int64(first.ID)), SortOrder: 4})
	assert.Error(t, err)
}

func TestMenuAccessibleTree_AdminLoadError(t *testing.T) {
	service, ctx := newMenuTestService(t, "admin:all")
	// AdminModel uninitialized → LoadCurrentAdmin errors.
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
	// The role name matches admin → the HasAdminRole branch appends the wildcard to permIDs.
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

// Pure-function branches of filterAccessibleTree: an orphan node (parent
// record missing) is promoted to a root; when the parent is filtered out by
// permissions, its whole subtree is hidden.
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
	// Without secret:read → the gated subtree is hidden as a whole; the
	// orphan's parent does not exist → promoted to a root.
	roots := filterAccessibleTree(items, []string{"other:read"})
	require.Len(t, roots, 1)
	assert.EqualValues(t, 3, roots[0].ID, "孤儿节点应提升为根")
	assert.Empty(t, roots[0].Children)

	// With the wildcard → the gated tree is visible (the child is attached
	// correctly); the orphan is still promoted to a root.
	roots = filterAccessibleTree(items, []string{"*"})
	require.Len(t, roots, 2)
	assert.EqualValues(t, 1, roots[0].ID)
	require.Len(t, roots[0].Children, 1)
	assert.EqualValues(t, 2, roots[0].Children[0].ID)
	assert.EqualValues(t, 3, roots[1].ID)
}

func TestMenuCreate_ResolveParentFindError(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create")

	// The 1st menu_items query (key dedup check) passes, the 2nd (parent
	// existence check) is injected with an error.
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
