package approval

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// AUD-038：Approve/Reject 加显式审批权限检查 + two_person 两人规则运行时执行。
//
// 两组用例：
//  1. 低权限拒绝/授权放行：走真实 RBAC 层（sqlite + AdminModel + PermissionService），
//     低权限角色（无 approvals:write）批准被 403 拒绝；持 approvals:write 的角色放行。
//  2. 同主体二次批准拒绝：two_person 策略优先于全局自批开关——即使
//     allowSelfApprove=true，two_person 函数仍禁止同一主体批准/拒绝。

// newPermissionDB 建独立内存库 + 迁移，供权限用例用（每测试新建，避免共享态）。
func newPermissionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// 单连接：内存库经多连接会各自看到空表（no such table）。
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

// permissionTestServiceContext 组装带 RBAC 的 ServiceContext（AdminModel +
// PermissionService + 缓存层），供 RequireAnyPermission 的 LoadCurrentAdmin 链路。
func permissionTestServiceContext(t *testing.T, db *gorm.DB, store *approvals.MemStore) *svc.ServiceContext {
	t.Helper()
	nullCache := cache.NewNullCache()
	return &svc.ServiceContext{
		DB:                db,
		AdminModel:        model.NewAdminModel(db),
		RoleModel:         model.NewRoleModel(db),
		PermissionModel:   model.NewPermissionModel(db),
		PermissionService: permission.NewPermissionService(db),
		ApprovalsStore:    store,
		Cache:             nullCache,
		CacheHelper:       cache.NewCacheHelper(nullCache),
	}
}

// createApprovalRoleAdmin 建一个带指定权限（可为空=低权限）角色的管理员，返回其 username。
func createApprovalRoleAdmin(t *testing.T, db *gorm.DB, username, roleName string, permissions ...string) {
	t.Helper()
	adminModel := model.NewAdminModel(db)
	admin := &model.Admin{Username: username, Nickname: username + " User", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "Password123!"))

	role := &model.Role{Name: roleName}
	require.NoError(t, db.Where("name = ?", roleName).FirstOrCreate(role).Error)
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))

	for _, perm := range permissions {
		rp := &model.RolePermission{RoleID: role.ID, PermissionID: perm}
		require.NoError(t, db.Where("role_id = ? AND permission_id = ?", role.ID, perm).FirstOrCreate(rp).Error)
	}
}

// seedPendingNoFunction 造一条待审批记录（FunctionID 空 → 续跑链路提前返回）。
func seedPendingNoFunction(t *testing.T, store *approvals.MemStore, id, actor string, metadata map[string]string) {
	t.Helper()
	_, err := store.Create(&approvals.Approval{
		ID: id, State: "pending", Actor: actor,
		GameID: "game-1", Env: "prod", Metadata: metadata,
	})
	require.NoError(t, err)
}

// TestApproveRejectsLowPrivilegeOperator AUD-038：无 approvals:write 的低权限
// 角色批准被拒绝（403），审批保持 pending。
func TestApproveRejectsLowPrivilegeOperator(t *testing.T) {
	db := newPermissionDB(t)
	store := approvals.NewMemStore()
	seedPendingNoFunction(t, store, "ap-low-1", "requester", nil)
	// viewer 角色无任何审批权限。
	createApprovalRoleAdmin(t, db, "lowop", "viewer")

	s := NewService(permissionTestServiceContext(t, db, store))

	resp, err := s.Approve(approvalCtx("lowop"), &ApprovalApproveRequest{ID: "ap-low-1"})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "无权审批")

	current, gErr := store.Get("ap-low-1")
	require.NoError(t, gErr)
	assert.Equal(t, "pending", current.State, "低权限被拒后审批必须保持 pending")
	assert.Empty(t, current.Approver)
}

// TestRejectRejectsLowPrivilegeOperator AUD-038：拒绝路径同样要求审批权限。
func TestRejectRejectsLowPrivilegeOperator(t *testing.T) {
	db := newPermissionDB(t)
	store := approvals.NewMemStore()
	seedPendingNoFunction(t, store, "ap-low-2", "requester", nil)
	createApprovalRoleAdmin(t, db, "lowop", "viewer")

	s := NewService(permissionTestServiceContext(t, db, store))

	_, err := s.Reject(actorCtx("lowop"), &ApprovalRejectRequest{ID: "ap-low-2", Reason: "越权拒绝"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权审批")

	current, gErr := store.Get("ap-low-2")
	require.NoError(t, gErr)
	assert.Equal(t, "pending", current.State)
}

// TestApproveAllowsOperatorWithApprovalPermission AUD-038 正向对照：持
// approvals:write 的非管理员角色批准放行。
func TestApproveAllowsOperatorWithApprovalPermission(t *testing.T) {
	db := newPermissionDB(t)
	store := approvals.NewMemStore()
	seedPendingNoFunction(t, store, "ap-auth-1", "requester", nil)
	createApprovalRoleAdmin(t, db, "approver", "reviewer", "approvals:write")

	s := NewService(permissionTestServiceContext(t, db, store))

	resp, err := s.Approve(approvalCtx("approver"), &ApprovalApproveRequest{ID: "ap-auth-1"})
	require.NoError(t, err)
	assert.Equal(t, "approved", resp.State)

	current, gErr := store.Get("ap-auth-1")
	require.NoError(t, gErr)
	assert.Equal(t, "approved", current.State)
	assert.Equal(t, "approver", current.Approver)
}

// TestApproveBlocksSelfApprovalUnderTwoPersonEvenWhenAllowed AUD-038：two_person
// 策略优先于全局自批开关——allowSelfApprove=true 也不能让同一主体批准。
func TestApproveBlocksSelfApprovalUnderTwoPersonEvenWhenAllowed(t *testing.T) {
	store := approvals.NewMemStore()
	seedPendingNoFunction(t, store, "ap-tp-1", "alice", map[string]string{
		approvals.ApprovalPolicyMetaKey: "two_person",
	})
	s := NewService(&svc.ServiceContext{
		ApprovalsStore: store,
		Config:         config.Config{Approval: config.ApprovalConfig{AllowSelfApprove: true}},
	})

	resp, err := s.Approve(approvalCtx("alice"), &ApprovalApproveRequest{ID: "ap-tp-1"})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "self_approval_forbidden")

	current, gErr := store.Get("ap-tp-1")
	require.NoError(t, gErr)
	assert.Equal(t, "pending", current.State, "two_person 自批被拒后必须保持 pending")
	assert.Empty(t, current.Approver)
}

// TestRejectBlocksSelfApprovalUnderTwoPersonEvenWhenAllowed AUD-038：拒绝路径同规。
func TestRejectBlocksSelfApprovalUnderTwoPersonEvenWhenAllowed(t *testing.T) {
	store := approvals.NewMemStore()
	seedPendingNoFunction(t, store, "ap-tp-2", "alice", map[string]string{
		approvals.ApprovalPolicyMetaKey: "two_person",
	})
	s := NewService(&svc.ServiceContext{
		ApprovalsStore: store,
		Config:         config.Config{Approval: config.ApprovalConfig{AllowSelfApprove: true}},
	})

	_, err := s.Reject(actorCtx("alice"), &ApprovalRejectRequest{ID: "ap-tp-2", Reason: "自拒"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "self_approval_forbidden")

	current, gErr := store.Get("ap-tp-2")
	require.NoError(t, gErr)
	assert.Equal(t, "pending", current.State)
}

// TestApproveDifferentPrincipalUnderTwoPersonSucceeds AUD-038 正向对照：
// two_person 下不同授权主体（bob ≠ alice）批准放行。
func TestApproveDifferentPrincipalUnderTwoPersonSucceeds(t *testing.T) {
	store := approvals.NewMemStore()
	seedPendingNoFunction(t, store, "ap-tp-3", "alice", map[string]string{
		approvals.ApprovalPolicyMetaKey: "two_person",
	})
	s := NewService(&svc.ServiceContext{
		ApprovalsStore: store,
		Config:         config.Config{Approval: config.ApprovalConfig{AllowSelfApprove: true}},
	})

	resp, err := s.Approve(approvalCtx("bob"), &ApprovalApproveRequest{ID: "ap-tp-3"})
	require.NoError(t, err)
	assert.Equal(t, "approved", resp.State)

	current, gErr := store.Get("ap-tp-3")
	require.NoError(t, gErr)
	assert.Equal(t, "bob", current.Approver)
}
