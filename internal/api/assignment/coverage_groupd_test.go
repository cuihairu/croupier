package assignment

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// vanishingUsernameContext injects the fault "the username vanishes within the
// same request after authorization":
// the first read (RequireAnyPermission → LoadCurrentAdmin → CurrentUsername)
// returns a valid username, and later reads return an empty string, which
// deterministically triggers
// the CurrentUsername error branch and the "system" operator fallback.
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

// authorizeCloneTarget: the game_env_bindings table is destroyed (missing) →
// the COUNT query of HasEnvBinding errors out → 500 with the env-validation
// failure message asserted below.
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

// authorizeCloneTarget: the target environment binding exists, but the username
// vanishes after authorization →
// LoadCurrentAdmin reports ErrCurrentUserNotFound and it is passed through.
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

// Update: reading the username fails after authorization passes → the history
// operator falls back to "system".
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
