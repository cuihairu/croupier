// 覆盖目标：service.go 的 DB 错误分支（创建/更新/删除/列表事务失败传播）
// 与 ensurePermissionIDs 失败分支。使用独立内存库 + gorm 回调注入错误，
// 不与其他测试文件共享数据库状态。
package role

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// ---- gorm 错误注入基建 ----

var covSchemaCache = &sync.Map{}

// covStmtTable 解析当前语句操作的物理表（显式 Table 优先，其次 Model/Dest 的 schema）。
func covStmtTable(tx *gorm.DB) string {
	if tx.Statement == nil {
		return ""
	}
	if tx.Statement.Table != "" {
		return tx.Statement.Table
	}
	for _, v := range []interface{}{tx.Statement.Model, tx.Statement.Dest} {
		if v == nil {
			continue
		}
		if s, err := schema.Parse(v, covSchemaCache, schema.NamingStrategy{}); err == nil {
			return s.Table
		}
	}
	return ""
}

type covFailureInjector struct {
	mu      sync.Mutex
	counts  map[string]int
	failAt  map[string]int
	failAll map[string]bool
}

func newCovFailureInjector() *covFailureInjector {
	return &covFailureInjector{
		counts:  map[string]int{},
		failAt:  map[string]int{},
		failAll: map[string]bool{},
	}
}

// register 在增删改查四类回调前挂接注入点；failAt 按 "op:table" 计数命中即失败，
// failAll 对 "op:table" 全部失败。
func (f *covFailureInjector) register(db *gorm.DB) {
	callback := func(op string) func(tx *gorm.DB) {
		return func(tx *gorm.DB) {
			table := covStmtTable(tx)
			if table == "" {
				return
			}
			key := op + ":" + table
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.failAll[key] {
				_ = tx.AddError(fmt.Errorf("injected %s failure on %s", op, table))
				return
			}
			if n, ok := f.failAt[key]; ok {
				f.counts[key]++
				if f.counts[key] >= n {
					_ = tx.AddError(fmt.Errorf("injected %s failure on %s", op, table))
				}
			}
		}
	}
	_ = db.Callback().Create().Before("gorm:create").Register("cov_fail_create", callback("create"))
	_ = db.Callback().Query().Before("gorm:query").Register("cov_fail_query", callback("query"))
	// Scan/Rows 走 Row 处理器（gorm:row），需单独挂接才能命中
	// GetRolesPermissionIDs 这类 Table().Scan() 查询。
	_ = db.Callback().Row().Before("gorm:row").Register("cov_fail_row", callback("query"))
	_ = db.Callback().Update().Before("gorm:update").Register("cov_fail_update", callback("update"))
	_ = db.Callback().Delete().Before("gorm:delete").Register("cov_fail_delete", callback("delete"))
}

// ---- 独立测试环境 ----

func newRoleCovEnv(t *testing.T) (*Service, context.Context, *covFailureInjector, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	admin := &model.Admin{Username: "covadmin", Nickname: "Cov Admin", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password123"))
	role := &model.Role{Name: "admin", Description: "admin role", Category: "system"}
	require.NoError(t, roleModel.Create(context.Background(), role))
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))
	require.NoError(t, roleModel.ReplacePermissions(context.Background(), role.ID, []string{"admin:all", "roles:manage", "role:write"}))
	for _, perm := range []*model.Permission{
		{ID: "admin:all", Name: "Admin All", Resource: "admin", Action: "all", Category: "admin"},
		{ID: "roles:manage", Name: "Roles Manage", Resource: "roles", Action: "manage", Category: "role"},
		{ID: "role:write", Name: "Role Write", Resource: "role", Action: "write", Category: "role"},
		{ID: "game:read", Name: "Game Read", Resource: "game", Action: "read", Category: "game"},
	} {
		require.NoError(t, db.Create(perm).Error)
	}

	inj := newCovFailureInjector()
	inj.register(db)

	svcCtx := &svc.ServiceContext{
		DB:              db,
		AdminModel:      adminModel,
		RoleModel:       roleModel,
		PermissionModel: model.NewPermissionModel(db),
		Cache:           cache.NewNullCache(),
		CacheHelper:     cache.NewCacheHelper(cache.NewNullCache()),
	}
	ctx := context.WithValue(context.Background(), "username", "covadmin")
	ctx = context.WithValue(ctx, "adminID", admin.ID)
	return NewService(svcCtx), ctx, inj, db
}

// seedCovRole 创建一个待操作角色并返回其字符串 ID。
func seedCovRole(t *testing.T, db *gorm.DB, name string) string {
	t.Helper()
	role := &model.Role{Name: name, Category: "cov"}
	require.NoError(t, model.NewRoleModel(db).Create(context.Background(), role))
	return strconv.FormatUint(uint64(role.ID), 10)
}

// ---- service.go 错误分支 ----

// RoleCreate：事务内 ReplacePermissions 失败（role_permissions 删除报错）。
func TestRoleCov_Create_ReplacePermissionsError(t *testing.T) {
	s, ctx, inj, _ := newRoleCovEnv(t)
	inj.failAll["delete:role_permissions"] = true

	resp, err := s.RoleCreate(ctx, &RoleCreateRequest{
		Name:        "covcreated",
		Permissions: []string{"game:read"},
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// RoleDelete：删除 role_permissions 行失败。
func TestRoleCov_Delete_PermissionRowsError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	id := seedCovRole(t, db, "covdelete")
	inj.failAll["delete:role_permissions"] = true

	err := s.RoleDelete(ctx, &RoleDeleteRequest{ID: id})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "删除角色权限失败")
}

// RoleDetail：GetRolePermissionIDsCached 查询失败（权限校验占用第 1 次 rp 查询，
// 详情为第 2 次）。
func TestRoleCov_Detail_PermissionIDsQueryError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	id := seedCovRole(t, db, "covdetail")
	inj.failAt["query:role_permissions"] = 2

	resp, err := s.RoleDetail(ctx, &RoleDetailRequest{ID: id})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// RoleUpdate：权限列表含不存在的权限 ID。
func TestRoleCov_Update_InvalidPermissions(t *testing.T) {
	s, ctx, _, db := newRoleCovEnv(t)
	id := seedCovRole(t, db, "covupdate")

	resp, err := s.RoleUpdate(ctx, &RoleUpdateRequest{
		ID:          id,
		Permissions: []string{"ghost:perm"},
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not found")
}

// RoleUpdate：roles 表更新失败。
func TestRoleCov_Update_ModelUpdateError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	id := seedCovRole(t, db, "covupdate2")
	inj.failAll["update:roles"] = true

	resp, err := s.RoleUpdate(ctx, &RoleUpdateRequest{ID: id, Name: "renamed"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// RoleUpdate：更新后 ReplacePermissions 失败。
func TestRoleCov_Update_ReplacePermissionsError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	id := seedCovRole(t, db, "covupdate3")
	inj.failAll["delete:role_permissions"] = true

	resp, err := s.RoleUpdate(ctx, &RoleUpdateRequest{
		ID:          id,
		Permissions: []string{"game:read"},
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// RoleUpdate：更新成功后 GetRoleCached 失败（roles 第 3 次查询：
// 权限校验 1 次 + 事务 FindOne 1 次 + 回读 1 次）。
func TestRoleCov_Update_GetRoleCachedError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	id := seedCovRole(t, db, "covupdate4")
	inj.failAt["query:roles"] = 3

	resp, err := s.RoleUpdate(ctx, &RoleUpdateRequest{ID: id, Name: "renamed"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// RoleUpdate：未携带权限时更新后 GetRolePermissionIDsCached 失败
// （rp 第 2 次查询：权限校验 1 次 + 回读 1 次）。
func TestRoleCov_Update_PermissionIDsCachedError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	id := seedCovRole(t, db, "covupdate5")
	inj.failAt["query:role_permissions"] = 2

	resp, err := s.RoleUpdate(ctx, &RoleUpdateRequest{ID: id, Name: "renamed"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// RolesList：List 查询失败（roles 第 2 次查询：权限校验 1 次 + Count 1 次）。
func TestRoleCov_List_QueryError(t *testing.T) {
	s, ctx, inj, _ := newRoleCovEnv(t)
	inj.failAt["query:roles"] = 2

	resp, err := s.RolesList(ctx, &RolesListRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// RolesList：列表成功后 GetRolesPermissionIDs 失败（rp 第 2 次查询）。
func TestRoleCov_List_RolesPermissionIDsError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	seedCovRole(t, db, "covlist")
	inj.failAt["query:role_permissions"] = 2

	resp, err := s.RolesList(ctx, &RolesListRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// parseRoleID：超出 uint64 上限的输入在 ParseUint 即报错。
// 注：value > math.MaxUint 分支在 64 位平台数学不可达——ParseUint(_, 10, 64)
// 成功时值域上限即 math.MaxUint（= MaxUint64），不可能严格大于。
func TestRoleCov_ParseRoleID_Overflow(t *testing.T) {
	s, _, _, _ := newRoleCovEnv(t)
	_, err := s.parseRoleID("18446744073709551616")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无效的角色ID")
}
