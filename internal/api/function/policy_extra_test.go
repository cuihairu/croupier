// Coverage target: the role-restriction / admin-bypass / role-mismatch
// branches of enforceFunctionPolicy.
package function

import (
	"context"
	"testing"

	policymgr "github.com/cuihairu/croupier/internal/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnforceFunctionPolicy_AllowedRolesAdminBypass(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "policy-admin", "admin")
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "restricted.fn", &policymgr.Policy{
		AllowedRoles: []string{"operator"},
	}))

	ctx := f.ctxFor("policy-admin")
	p, err := enforceFunctionPolicy(ctx, f.svcCtx, "restricted.fn", "", "", []string{"admin"})
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestEnforceFunctionPolicy_NoRoleRestriction(t *testing.T) {
	f := newInvokeFixture(t)
	// Default policy (no override) with the admin role bypassing the Casbin check
	f.createOperator(t, "policy-free", "admin")

	ctx := f.ctxFor("policy-free")
	p, err := enforceFunctionPolicy(ctx, f.svcCtx, "any.fn", "", "", []string{"admin"})
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestEnforceFunctionPolicy_RoleMismatchForbidden(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "policy-viewer", "viewer")
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "restricted.fn2", &policymgr.Policy{
		AllowedRoles: []string{"operator"},
	}))

	ctx := f.ctxFor("policy-viewer")
	_, err := enforceFunctionPolicy(ctx, f.svcCtx, "restricted.fn2", "", "", []string{"viewer"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权调用")
}
