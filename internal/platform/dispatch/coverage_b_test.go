// 覆盖目标（B 批）：noLiveAgentError 的「注册警告无版本号」分支——
// 被门槛拦截的证据成立但警告里 Version 为空（历史数据/旧写入路径），
// 错误信息回落 "unknown" 而非空串，保持可读性与既有测试互补。
package dispatch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/cuihairu/croupier/internal/errors"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

func TestDispatcherCovB_NoLiveAgentError_WarningWithoutVersion(t *testing.T) {
	store := reg.NewStore()
	require.NoError(t, store.SetFunctionVersionFloor("game-1", "dev", "player.ban", "2.0.0", "admin"))
	// 警告 Version 留空：门槛拦截证据（警告 + 门槛）双条件仍成立
	store.UpsertRegistrationWarning(context.Background(), reg.FunctionRegistrationWarning{
		GameID:     "game-1",
		Env:        "dev",
		AgentID:    "agent-legacy",
		FunctionID: "player.ban",
		Version:    "",
		Code:       reg.WarningCodeFunctionVersionBelowMinimum,
		Message:    "function_id=player.ban below configured minimum 2.0.0: function registration rejected",
	})
	d := NewDispatcher(store)

	err := d.noLiveAgentError("player.ban", "game-1", "dev", true)

	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.ErrCodeFunctionVersionBelowMinimum, appErr.Code)
	// 空版本回落 unknown，错误信息不出现空版本占位
	assert.Contains(t, err.Error(), "registered version unknown")
}
