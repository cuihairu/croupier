package dispatch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/cuihairu/croupier/internal/errors"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// E9：版本门槛拦截的执行侧可解释性。函数被函数级最低版本门槛拦截时
// （注册警告 + 门槛仍配置，双条件），调度无候选返回专属错误码
// FUNCTION_VERSION_BELOW_MINIMUM，与「agent 真没活」(SERVICE_UNAVAILABLE)
// 按码区分；门槛已删的陈旧警告不得误述。
func TestDispatcherVersionFloorBlockedError(t *testing.T) {
	store := reg.NewStore()
	require.NoError(t, store.SetFunctionVersionFloor("game-1", "dev", "player.ban", "2.0.0", "admin"))
	store.UpsertRegistrationWarning(context.Background(), reg.FunctionRegistrationWarning{
		GameID:     "game-1",
		Env:        "dev",
		AgentID:    "agent-old",
		FunctionID: "player.ban",
		Version:    "1.0.0",
		Code:       reg.WarningCodeFunctionVersionBelowMinimum,
		Message:    "function_id=player.ban version=1.0.0 below configured minimum 2.0.0: function registration rejected",
	})
	d := NewDispatcher(store)

	codeOf := func(t *testing.T, err error) apperrors.ErrorCode {
		t.Helper()
		require.Error(t, err)
		var appErr *apperrors.AppError
		require.ErrorAs(t, err, &appErr)
		return appErr.Code
	}

	// 同步调用路径（scoped 路由，无候选）
	_, err := d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{
		FunctionId: "player.ban",
		Metadata:   map[string]string{"gameId": "game-1", "env": "dev"},
	})
	assert.Equal(t, apperrors.ErrCodeFunctionVersionBelowMinimum, codeOf(t, err))
	assert.Contains(t, err.Error(), "blocked by version floor")
	assert.Contains(t, err.Error(), "1.0.0", "错误信息应带上被拦版本")

	// 广播路径同样归因
	_, err = d.InvokeBroadcast(context.Background(), &sdkv1.InvokeRequest{
		FunctionId: "player.ban",
		Metadata:   map[string]string{"gameId": "game-1", "env": "dev"},
	})
	assert.Equal(t, apperrors.ErrCodeFunctionVersionBelowMinimum, codeOf(t, err))

	// 无门槛拦截证据的函数：保持 no live agent 原语义
	_, err = d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{
		FunctionId: "player.list",
		Metadata:   map[string]string{"gameId": "game-1", "env": "dev"},
	})
	assert.Equal(t, apperrors.ErrCodeServiceUnavailable, codeOf(t, err))

	// 警告在但门槛已删：陈旧警告不得把「agent 真没活」误述成门槛拦截
	require.NoError(t, store.DeleteFunctionVersionFloor("game-1", "dev", "player.ban"))
	_, err = d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{
		FunctionId: "player.ban",
		Metadata:   map[string]string{"gameId": "game-1", "env": "dev"},
	})
	assert.Equal(t, apperrors.ErrCodeServiceUnavailable, codeOf(t, err))

	// 非 scoped 路由不做门槛归因（警告/门槛都按 scope 过滤）
	require.NoError(t, store.SetFunctionVersionFloor("game-1", "dev", "player.ban", "2.0.0", "admin"))
	_, err = d.InvokeRequest(context.Background(), &sdkv1.InvokeRequest{FunctionId: "player.ban"})
	assert.Equal(t, apperrors.ErrCodeServiceUnavailable, codeOf(t, err))
}
