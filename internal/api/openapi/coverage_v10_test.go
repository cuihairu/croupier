package openapi

// 覆盖目标（组 B）：hasRegisteredFunction 遍历 registry 时跳过 nil 会话条目。
// 用一个任何 agent/DB 都不持有的 functionID 触发全量遍历（无早退），
// 使 nil 会话的 continue 分支确定性命中（既有 v9 用例因 map 遍历顺序
// 随机可能在命中前早退，覆盖率不稳定）。

import (
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registeredFunctionMetaInScope 的 nil 会话跳过同样受 map 遍历顺序影响：
// 用一个只存在于 functions 表（registry 无）的 functionID 逼出对 agent
// 全集的确定性遍历，nil 分支必命中，兜底 DB 路径返回 true。
func TestV10RegisteredFunctionMeta_SkipsNilSessionViaDBFallback(t *testing.T) {
	service, _ := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	require.NoError(t, service.svcCtx.DB.Create(&model.Function{
		FunctionID: "db.only.fn.v10",
		Name:       "db only",
		GameID:     "demo-game",
		Status:     1,
		Version:    "1.0.0",
	}).Error)

	store := service.svcCtx.RegistryStore
	store.Mu().Lock()
	store.AgentsUnsafe()["nil-agent-v10c"] = nil
	store.Mu().Unlock()

	meta, ok := registeredFunctionMetaInScope(service.svcCtx, "demo-game", "development", "db.only.fn.v10")
	require.True(t, ok, "DB fallback must resolve the function after scanning all agents")
	assert.True(t, meta.Enabled)
	assert.Equal(t, "1.0.0", meta.Version)
}

func TestV10HasRegisteredFunction_SkipsNilSessionDeterministically(t *testing.T) {
	service, _ := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	store := service.svcCtx.RegistryStore
	store.Mu().Lock()
	store.AgentsUnsafe()["nil-agent-v10"] = nil
	store.Mu().Unlock()

	// functionID 在任何 agent 与 functions 表都不存在：两个数据源都
	// 必须被完整遍历（nil 会话条目被跳过），最终返回 false。
	assert.False(t, hasRegisteredFunction(service.svcCtx, "definitely.missing.fn.v10"))

	// 缩回去，不影响其他用例的共享 store（每个用例独立 service，
	// 此处仅为语义完整）。
	store.Mu().Lock()
	delete(store.AgentsUnsafe(), "nil-agent-v10")
	store.Mu().Unlock()
}

// 已注册函数仍应命中（nil 条目不中断遍历）。
func TestV10HasRegisteredFunction_RegisteredStillFound(t *testing.T) {
	service, _ := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	store := service.svcCtx.RegistryStore
	store.Mu().Lock()
	store.AgentsUnsafe()["nil-agent-v10b"] = nil
	store.Mu().Unlock()

	require.True(t, hasRegisteredFunction(service.svcCtx, "player.list"))
}
