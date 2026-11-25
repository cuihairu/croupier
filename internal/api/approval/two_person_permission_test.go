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

// AUD-038: Approve/Reject gain an explicit approval permission check plus
// runtime enforcement of the two_person two-person rule.
//
// Two groups of cases:
//  1. Low-privilege rejection / authorized pass-through: exercised through the
//     real RBAC layer (sqlite + AdminModel + PermissionService) —
//     a low-privilege role (without approvals:write) is rejected with 403 when
//     approving, while a role holding approvals:write is let through.
//  2. A second approval by the same subject is rejected: the two_person policy
//     takes precedence over the global self-approval switch — even with
//     allowSelfApprove=true, two_person functions still forbid the same subject
//     from approving/rejecting.

// newPermissionDB builds an isolated in-memory database with migrations, for
// the permission cases (created fresh per test to avoid shared state).
func newPermissionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// A single connection: an in-memory database seen through multiple
	// connections would each observe empty tables (no such table).
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

// permissionTestServiceContext assembles a ServiceContext with RBAC (AdminModel +
// PermissionService + cache layer), for the LoadCurrentAdmin chain of
// RequireAnyPermission.
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

// createApprovalRoleAdmin creates an admin whose role carries the given
// permissions (empty = low privilege).
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

// seedPendingNoFunction creates a pending approval record (an empty FunctionID
// makes the continuation chain return early).
func seedPendingNoFunction(t *testing.T, store *approvals.MemStore, id, actor string, metadata map[string]string) {
	t.Helper()
	_, err := store.Create(&approvals.Approval{
		ID: id, State: "pending", Actor: actor,
		GameID: "game-1", Env: "prod", Metadata: metadata,
	})
	require.NoError(t, err)
}

// TestApproveRejectsLowPrivilegeOperator AUD-038: an approval by a low-privilege
// role without approvals:write is rejected (403) and the approval stays
// pending.
func TestApproveRejectsLowPrivilegeOperator(t *testing.T) {
	db := newPermissionDB(t)
	store := approvals.NewMemStore()
	seedPendingNoFunction(t, store, "ap-low-1", "requester", nil)
	// The viewer role has no approval permission at all.
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

// TestRejectRejectsLowPrivilegeOperator AUD-038: the reject path likewise
// requires the approval permission.
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

// TestApproveAllowsOperatorWithApprovalPermission AUD-038 positive control: a
// non-admin role holding approvals:write is let through when approving.
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

// TestApproveBlocksSelfApprovalUnderTwoPersonEvenWhenAllowed AUD-038: the
// two_person policy takes precedence over the global self-approval switch —
// even allowSelfApprove=true cannot let the same subject approve.
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

// TestRejectBlocksSelfApprovalUnderTwoPersonEvenWhenAllowed AUD-038: the reject
// path follows the same rule.
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

// TestApproveDifferentPrincipalUnderTwoPersonSucceeds AUD-038 positive control:
// under two_person, a different authorized subject (bob != alice) is let
// through when approving.
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
