package assignment

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// vanishingUsernameContext 注入「同一请求内 username 在鉴权之后消失」故障：
// 第一次读取（RequireAnyPermission → LoadCurrentAdmin → CurrentUsername）
// 返回有效用户名，之后读取返回空串， deterministic 地触发
// CurrentUsername 错误分支与 "system" 操作者回落。
type vanishingUsernameContext struct {
	context.Context
	reads int
}

func (c *vanishingUsernameContext) Value(key any) any {
	if key == "username" {
		c.reads++
		if c.reads <= 1 {
			return "testadmin"
		}
		return ""
	}
	return c.Context.Value(key)
}

// authorizeCloneTarget：game_env_bindings 表被破坏（缺表）→
// HasEnvBinding 的 COUNT 查询报错 → 500「校验目标环境失败」。
func TestService_Update_CloneTargetEnvBindingStoreError(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)
	ctx = svc.WithGameScope(ctx, svc.GameScope{GameID: "game1", Env: "prod"})

	require.NoError(t, db.Migrator().DropTable(&model.GameEnvBinding{}))

	service := NewService(svcCtx)
	_, err := service.Update(ctx, &AssignmentsUpdateRequest{
		GameId:    "game1",
		Env:       "prod",
		Action:    "clone",
		TargetEnv: "stage",
		Functions: []string{"func1"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "校验目标环境失败")
}

// authorizeCloneTarget：目标环境绑定存在，但鉴权后用户名消失 →
// LoadCurrentAdmin 报 ErrCurrentUserNotFound 并透传。
func TestService_Update_CloneTargetAdminLookupError(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)
	ctx = svc.WithGameScope(ctx, svc.GameScope{GameID: "game1", Env: "prod"})

	game := &model.Game{GameID: "game1", Name: "Game One"}
	require.NoError(t, svcCtx.GameModel.Create(ctx, game))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "game1", "stage", "game1_stage", "", ""))

	service := NewService(svcCtx)
	_, err := service.Update(&vanishingUsernameContext{Context: ctx}, &AssignmentsUpdateRequest{
		GameId:    "game1",
		Env:       "prod",
		Action:    "clone",
		TargetEnv: "stage",
		Functions: []string{"func1"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未找到登录用户")
}

// Update：鉴权通过后 username 读取失败 → 历史操作者回落 "system"。
func TestService_Update_HistoryOperatorFallsBackToSystem(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)

	service := NewService(svcCtx)
	resp, err := service.Update(&vanishingUsernameContext{Context: ctx}, &AssignmentsUpdateRequest{
		GameId:    "game1",
		Env:       "prod",
		Action:    "assign",
		Functions: []string{"func1"},
	})
	require.NoError(t, err)
	assert.True(t, resp.OK)

	entries, err := loadAssignmentHistory(assignmentHistoryPath(svcCtx))
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	assert.Equal(t, "system", entries[0].OperatedBy)
}
