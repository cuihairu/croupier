// AUD-017 regression: fill in the platform permission checks on the power
// domain operation entries.
//
// Root cause: the six powerops entries (submit/resume/reject/get/list/machine
// power info write) were operable with nothing more than authentication and a
// valid game/env scope — zero-role or game-side-only accounts could submit power
// actions and modify global node BMC configuration. The permission catalog
// (configs/permissions.json) has no power/hardware management keys, so the gate
// only accepts the super-admin domain admin:all: zero-role accounts and
// game-side-only accounts get 403 permission_denied on all six entries; the
// super-admin control case stays functional.
package ops

import (
	"context"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/powerops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// powerViewerUsername is a zero-role account: the admin exists, the login
// context is normal, but it holds no roles/permissions at all — the permission
// set is empty, so RequireAnyPermission must reject it.
const powerViewerUsername = "power_norole_viewer"

// powerScopedOperatorUsername is a role account holding only game-side
// permissions (function:invoke), standing in for a "legitimate single-game
// scope account" — a game scope does not imply hardware management permission.
const powerScopedOperatorUsername = "power_game_scoped_operator"

func seedPowerViewer(t *testing.T, db *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	adminModel := model.NewAdminModel(db)
	admin, err := adminModel.FindByUsername(ctx, powerViewerUsername)
	if err != nil {
		admin = &model.Admin{Username: powerViewerUsername, Status: 1}
		require.NoError(t, adminModel.Create(ctx, admin, "password123"))
	}
}

func seedPowerScopedOperator(t *testing.T, db *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	admin, err := adminModel.FindByUsername(ctx, powerScopedOperatorUsername)
	if err != nil {
		admin = &model.Admin{Username: powerScopedOperatorUsername, Status: 1}
		require.NoError(t, adminModel.Create(ctx, admin, "password123"))
	}

	hasRole := false
	roles, err := adminModel.GetAdminRoles(ctx, admin.ID)
	require.NoError(t, err)
	for _, r := range roles {
		if r.Name == "power_game_scoped" {
			hasRole = true
			break
		}
	}
	if !hasRole {
		role := &model.Role{Name: "power_game_scoped"}
		require.NoError(t, roleModel.Create(ctx, role))
		require.NoError(t, adminModel.AssignRole(ctx, admin.ID, role.ID))
	}
	// Game-side permission (outside the super-admin domain): legitimate, but not
	// enough to operate the power domain.
	roles, err = adminModel.GetAdminRoles(ctx, admin.ID)
	require.NoError(t, err)
	for _, r := range roles {
		if r.Name == "power_game_scoped" {
			require.NoError(t, roleModel.ReplacePermissions(ctx, r.ID, []string{"function:invoke"}))
			break
		}
	}
}

// powerCtx returns a request context carrying the given username identity.
func powerCtx(username string) context.Context {
	return context.WithValue(context.Background(), "username", username)
}

// assertPowerForbidden asserts a stable 403 / permission_denied rejection.
func assertPowerForbidden(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err, "低权限请求必须被拒绝")
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusForbidden, codeErr.Code)
	assert.Equal(t, "permission_denied", codeErr.ErrorCode())
}

// TestPowerOps_Aud017_LowPrivilegeDeniedOnAllEntries verifies that all six power
// entries return 403 for zero-role and game-side-only accounts, and that
// submit/write attempts have no side effects (no request rows written, machine
// power info unchanged).
func TestPowerOps_Aud017_LowPrivilegeDeniedOnAllEntries(t *testing.T) {
	h := newPowerHarness(t, true, []string{"on"})
	seedPowerViewer(t, h.db)
	seedPowerScopedOperator(t, h.db)

	for _, username := range []string{powerViewerUsername, powerScopedOperatorUsername} {
		t.Run(username, func(t *testing.T) {
			ctx := powerCtx(username)
			svc := h.service

			// Submit (a mutating action): 403, and no request row is written.
			_, _, err := svc.OpsPowerSubmit(ctx, &OpsPowerSubmitRequest{
				NodeIDs: []string{"node-1"}, Action: "power_on", Reason: "maintenance",
			}, username)
			assertPowerForbidden(t, err)

			// Submit (read-only status query) is inside the authorization surface
			// too: 403.
			_, _, err = svc.OpsPowerSubmit(ctx, &OpsPowerSubmitRequest{
				NodeIDs: []string{"node-1"}, Action: "status", Reason: "health check",
			}, username)
			assertPowerForbidden(t, err)

			// List / get / resume / reject: 403 (authorization precedes resource
			// existence).
			_, err = svc.OpsPowerList(ctx, 10)
			assertPowerForbidden(t, err)

			_, _, err = svc.OpsPowerGet(ctx, "pw-x")
			assertPowerForbidden(t, err)

			_, err = svc.OpsPowerResume(ctx, "pw-x")
			assertPowerForbidden(t, err)

			_, err = svc.OpsPowerReject(ctx, "pw-x")
			assertPowerForbidden(t, err)

			// Machine power info write (global node BMC configuration): 403 and
			// the config is unchanged.
			err = svc.OpsPowerConfigPut(ctx, "node-1", &OpsPowerConfigRequest{
				Provider: "ipmi", BmcAddr: "10.9.9.9", BmcPort: 623,
			})
			assertPowerForbidden(t, err)

			// No side effects: machine power info still holds the seed values
			// (no write happened).
			var node model.Node
			require.NoError(t, h.db.Where("node_id = ?", "node-1").First(&node).Error)
			powerMeta, ok := node.Meta["power"].(map[string]interface{})
			require.True(t, ok, "node-1 meta.power 必须存在: %v", node.Meta)
			assert.Equal(t, "ipmi", powerMeta["provider"])
			assert.Equal(t, "10.0.0.7", powerMeta["bmcAddr"], "低权限写入不得改写 BMC 地址")

			// No side effects: the submit was rejected and no power request row
			// was written.
			requests, err := h.store.List(10)
			require.NoError(t, err)
			assert.Empty(t, requests, "被拒的提交不得落请求行")
		})
	}
}

// TestPowerOps_Aud017_SuperAdminControl verifies the super-admin passthrough via
// HasAdminRole: all six entries work on the data plane (resume of an unapproved
// request reports a 409 state conflict, proving the authorization gate was
// passed).
func TestPowerOps_Aud017_SuperAdminControl(t *testing.T) {
	h := newPowerHarness(t, true, []string{"on"})
	ctx := powerCtx(powerSuperAdminUsername)
	svc := h.service

	// Machine power info write (the privilege escalation path proven by the
	// earlier audit): allowed for the super admin.
	require.NoError(t, svc.OpsPowerConfigPut(ctx, "node-1", &OpsPowerConfigRequest{
		Provider: "ipmi", BmcAddr: "10.0.0.7", BmcPort: 623,
	}))

	// Submit: pending + approval record.
	req, _, err := svc.OpsPowerSubmit(ctx, &OpsPowerSubmitRequest{
		NodeIDs: []string{"node-1"}, Action: "power_on", Reason: "maintenance",
	}, powerSuperAdminUsername)
	require.NoError(t, err)
	assert.Equal(t, powerops.StatePending, req.State)
	assert.NotEmpty(t, req.ApprovalID)

	// List / get: readable on the data plane.
	requests, err := svc.OpsPowerList(ctx, 10)
	require.NoError(t, err)
	require.Len(t, requests, 1)

	got, _, err := svc.OpsPowerGet(ctx, req.RequestID)
	require.NoError(t, err)
	assert.Equal(t, req.RequestID, got.RequestID)

	// Resume: past the authorization gate, lands in a business state conflict
	// (not approved).
	_, err = svc.OpsPowerResume(ctx, req.RequestID)
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusConflict, codeErr.Code)

	// Reject: allowed.
	rejected, err := svc.OpsPowerReject(ctx, req.RequestID)
	require.NoError(t, err)
	assert.Equal(t, powerops.StateRejected, rejected.State)
}
