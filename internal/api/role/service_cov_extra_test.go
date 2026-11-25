// Coverage target: the DB error branches of service.go (create/update/delete/list
// transaction failure propagation) plus the ensurePermissionIDs failure branch. Uses
// an isolated in-memory database with gorm callback error injection, so it does not
// share database state with other test files.
package role

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// ---- gorm error injection harness ----

var covSchemaCache = &sync.Map{}

// covStmtTable resolves the physical table the current statement operates on
// (explicit Table wins, then the schema of Model/Dest).
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

// register hooks the injection points before the create/query/update/delete
// callbacks; failAt fails the Nth hit counted per "op:table", failAll fails every
// "op:table" unconditionally.
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
	// Scan/Rows runs through the Row processor (gorm:row), so it must be hooked
	// separately to reach Table().Scan() queries such as GetRolesPermissionIDs.
	_ = db.Callback().Row().Before("gorm:row").Register("cov_fail_row", callback("query"))
	_ = db.Callback().Update().Before("gorm:update").Register("cov_fail_update", callback("update"))
	_ = db.Callback().Delete().Before("gorm:delete").Register("cov_fail_delete", callback("delete"))
}

// ---- isolated test environment ----

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

// seedCovRole creates a role to operate on and returns its string ID.
func seedCovRole(t *testing.T, db *gorm.DB, name string) string {
	t.Helper()
	role := &model.Role{Name: name, Category: "cov"}
	require.NoError(t, model.NewRoleModel(db).Create(context.Background(), role))
	return strconv.FormatUint(uint64(role.ID), 10)
}

// ---- service.go error branches ----

// RoleCreate: ReplacePermissions inside the transaction fails (the role_permissions
// delete errors).
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

// RoleDelete: deleting the role_permissions rows fails.
func TestRoleCov_Delete_PermissionRowsError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	id := seedCovRole(t, db, "covdelete")
	inj.failAll["delete:role_permissions"] = true

	err := s.RoleDelete(ctx, &RoleDeleteRequest{ID: id})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "删除角色权限失败")
}

// RoleDetail: the GetRolePermissionIDsCached query fails (the permission check
// consumes the 1st role_permissions query, the detail call the 2nd).
func TestRoleCov_Detail_PermissionIDsQueryError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	id := seedCovRole(t, db, "covdetail")
	inj.failAt["query:role_permissions"] = 2

	resp, err := s.RoleDetail(ctx, &RoleDetailRequest{ID: id})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// RoleUpdate: the permission list contains a nonexistent permission ID.
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

// RoleUpdate: the roles table update fails.
func TestRoleCov_Update_ModelUpdateError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	id := seedCovRole(t, db, "covupdate2")
	inj.failAll["update:roles"] = true

	resp, err := s.RoleUpdate(ctx, &RoleUpdateRequest{ID: id, Name: "renamed"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// RoleUpdate: ReplacePermissions after the update fails.
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

// RoleUpdate: GetRoleCached fails after a successful update (the 3rd roles query:
// 1x permission check + 1x transaction FindOne + 1x read-back).
func TestRoleCov_Update_GetRoleCachedError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	id := seedCovRole(t, db, "covupdate4")
	inj.failAt["query:roles"] = 3

	resp, err := s.RoleUpdate(ctx, &RoleUpdateRequest{ID: id, Name: "renamed"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// RoleUpdate: with no permissions attached, GetRolePermissionIDsCached fails after
// the update (the 2nd role_permissions query: 1x permission check + 1x read-back).
func TestRoleCov_Update_PermissionIDsCachedError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	id := seedCovRole(t, db, "covupdate5")
	inj.failAt["query:role_permissions"] = 2

	resp, err := s.RoleUpdate(ctx, &RoleUpdateRequest{ID: id, Name: "renamed"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// RolesList: the List query fails (the 2nd roles query: 1x permission check + 1x Count).
func TestRoleCov_List_QueryError(t *testing.T) {
	s, ctx, inj, _ := newRoleCovEnv(t)
	inj.failAt["query:roles"] = 2

	resp, err := s.RolesList(ctx, &RolesListRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// RolesList: GetRolesPermissionIDs fails after the list succeeds (the 2nd
// role_permissions query).
func TestRoleCov_List_RolesPermissionIDsError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	seedCovRole(t, db, "covlist")
	inj.failAt["query:role_permissions"] = 2

	resp, err := s.RolesList(ctx, &RolesListRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// ParseRoleID: input beyond the uint64 range errors out in ParseUint.
// Note: an out-of-uint64-range input errors inside ParseUint itself, so there is no
// overflow dead branch to exercise.
func TestRoleCov_ParseRoleID_Overflow(t *testing.T) {
	_, err := utils.ParseRoleID("18446744073709551616")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无效的角色ID")
}
