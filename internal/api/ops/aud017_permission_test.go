// AUD-017 回归：电源域操作入口补齐平台权限检查。
//
// 根因：powerops 六个入口（提交/续行/拒绝/查询/列表/机器电源信息写入）只凭
// 认证与合法的游戏/环境作用域即可操作，零角色或仅持游戏侧权限的账号可提交
// 电源动作并修改全局节点 BMC 配置。权限目录（configs/permissions.json）无
// 电源/硬件管理键，故门禁只认超管域 admin:all：零角色账号与仅持游戏侧权限
// 的账号在六个入口上一律 403 permission_denied；超管对照保持可用。
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

// powerViewerUsername 零角色账号：管理员存在、登录上下文正常，但无任何角色/
// 权限，权限集合为空，RequireAnyPermission 必须拒绝。
const powerViewerUsername = "power_norole_viewer"

// powerScopedOperatorUsername 仅持游戏侧权限（function:invoke）的角色账号，
// 代表「合法单游戏 scope 账号」——游戏作用域不等同硬件管理权限。
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
	// 游戏侧权限（不在超管域内）：合法但不足以操作电源域。
	roles, err = adminModel.GetAdminRoles(ctx, admin.ID)
	require.NoError(t, err)
	for _, r := range roles {
		if r.Name == "power_game_scoped" {
			require.NoError(t, roleModel.ReplacePermissions(ctx, r.ID, []string{"function:invoke"}))
			break
		}
	}
}

// powerCtx 返回携带指定用户名身份的请求上下文。
func powerCtx(username string) context.Context {
	return context.WithValue(context.Background(), "username", username)
}

// assertPowerForbidden 断言为 403 / permission_denied 的稳定拒绝。
func assertPowerForbidden(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err, "低权限请求必须被拒绝")
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusForbidden, codeErr.Code)
	assert.Equal(t, "permission_denied", codeErr.ErrorCode())
}

// TestPowerOps_Aud017_LowPrivilegeDeniedOnAllEntries 验证六个电源入口对零角色
// 与仅持游戏侧权限的账号一律 403，且提交/写入无副作用（不落请求行、不改机器
// 电源信息）。
func TestPowerOps_Aud017_LowPrivilegeDeniedOnAllEntries(t *testing.T) {
	h := newPowerHarness(t, true, []string{"on"})
	seedPowerViewer(t, h.db)
	seedPowerScopedOperator(t, h.db)

	for _, username := range []string{powerViewerUsername, powerScopedOperatorUsername} {
		t.Run(username, func(t *testing.T) {
			ctx := powerCtx(username)
			svc := h.service

			// 提交（变更动作）：403，且不落任何请求行。
			_, _, err := svc.OpsPowerSubmit(ctx, &OpsPowerSubmitRequest{
				NodeIDs: []string{"node-1"}, Action: "power_on", Reason: "maintenance",
			}, username)
			assertPowerForbidden(t, err)

			// 提交（status 只读查询）同样在授权面内：403。
			_, _, err = svc.OpsPowerSubmit(ctx, &OpsPowerSubmitRequest{
				NodeIDs: []string{"node-1"}, Action: "status", Reason: "health check",
			}, username)
			assertPowerForbidden(t, err)

			// 列表 / 查询 / 续行 / 拒绝：403（授权先于资源存在性）。
			_, err = svc.OpsPowerList(ctx, 10)
			assertPowerForbidden(t, err)

			_, _, err = svc.OpsPowerGet(ctx, "pw-x")
			assertPowerForbidden(t, err)

			_, err = svc.OpsPowerResume(ctx, "pw-x")
			assertPowerForbidden(t, err)

			_, err = svc.OpsPowerReject(ctx, "pw-x")
			assertPowerForbidden(t, err)

			// 机器电源信息写入（全局节点 BMC 配置）：403 且配置不变。
			err = svc.OpsPowerConfigPut(ctx, "node-1", &OpsPowerConfigRequest{
				Provider: "ipmi", BmcAddr: "10.9.9.9", BmcPort: 623,
			})
			assertPowerForbidden(t, err)

			// 无副作用：机器电源信息仍为 seed 值（写入未发生）。
			var node model.Node
			require.NoError(t, h.db.Where("node_id = ?", "node-1").First(&node).Error)
			powerMeta, ok := node.Meta["power"].(map[string]interface{})
			require.True(t, ok, "node-1 meta.power 必须存在: %v", node.Meta)
			assert.Equal(t, "ipmi", powerMeta["provider"])
			assert.Equal(t, "10.0.0.7", powerMeta["bmcAddr"], "低权限写入不得改写 BMC 地址")

			// 无副作用：提交被拒，未落任何电源请求行。
			requests, err := h.store.List(10)
			require.NoError(t, err)
			assert.Empty(t, requests, "被拒的提交不得落请求行")
		})
	}
}

// TestPowerOps_Aud017_SuperAdminControl 验证超管经 HasAdminRole 直通：六个
// 入口数据面可用（续行未批准报 409 状态冲突，证明已越过授权门）。
func TestPowerOps_Aud017_SuperAdminControl(t *testing.T) {
	h := newPowerHarness(t, true, []string{"on"})
	ctx := powerCtx(powerSuperAdminUsername)
	svc := h.service

	// 机器电源信息写入（此前审计实证的越权路径）：超管放行。
	require.NoError(t, svc.OpsPowerConfigPut(ctx, "node-1", &OpsPowerConfigRequest{
		Provider: "ipmi", BmcAddr: "10.0.0.7", BmcPort: 623,
	}))

	// 提交：pending + 审批单。
	req, _, err := svc.OpsPowerSubmit(ctx, &OpsPowerSubmitRequest{
		NodeIDs: []string{"node-1"}, Action: "power_on", Reason: "maintenance",
	}, powerSuperAdminUsername)
	require.NoError(t, err)
	assert.Equal(t, powerops.StatePending, req.State)
	assert.NotEmpty(t, req.ApprovalID)

	// 列表 / 查询：数据面可读。
	requests, err := svc.OpsPowerList(ctx, 10)
	require.NoError(t, err)
	require.Len(t, requests, 1)

	got, _, err := svc.OpsPowerGet(ctx, req.RequestID)
	require.NoError(t, err)
	assert.Equal(t, req.RequestID, got.RequestID)

	// 续行：已过授权门，落到业务状态冲突（未批准）。
	_, err = svc.OpsPowerResume(ctx, req.RequestID)
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusConflict, codeErr.Code)

	// 拒绝：放行。
	rejected, err := svc.OpsPowerReject(ctx, req.RequestID)
	require.NoError(t, err)
	assert.Equal(t, powerops.StateRejected, rejected.State)
}
