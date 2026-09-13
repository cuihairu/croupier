// 覆盖目标：RemoveRegistrationWarnings 过滤器矩阵、SetOnReport、
// UpsertRegistrationWarning 去重计数。
// 设计债清理：UpsertRegistrationWarning 原签名声明 error 但纯内存实现
// 恒返 nil，已删除 error 返回值——下列用例同步去掉 require.NoError 包裹，
// warning 写入/去重计数行为保持不变。
package registry

import (
	"context"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveRegistrationWarnings_FilterMatrix(t *testing.T) {
	ctx := context.Background()
	s := NewStore()

	s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{GameID: "g1", Env: "prod", AgentID: "a1", FunctionID: "f1", Code: "empty_function_id", Message: "m1"})
	s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{GameID: "g2", Env: "dev", AgentID: "a2", FunctionID: "f2", Code: "invalid_function_id", Message: "m2"})
	// 相同 key 再上报：计数 +1
	s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{GameID: "g1", Env: "prod", AgentID: "a1", FunctionID: "f1", Code: "empty_function_id", Message: "m1"})

	// 空 message 静默跳过
	s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{GameID: "g1", Message: ""})

	// 按 agent 精确移除
	removed := s.RemoveRegistrationWarnings(RegistrationWarningFilter{AgentID: "a1"})
	assert.Equal(t, 1, removed)
	// 幂等：再移除同一 agent 无可删
	assert.Equal(t, 0, s.RemoveRegistrationWarnings(RegistrationWarningFilter{AgentID: "a1"}))
	// 空过滤器清空全部
	assert.Equal(t, 1, s.RemoveRegistrationWarnings(RegistrationWarningFilter{}))
	assert.Equal(t, 0, s.RemoveRegistrationWarnings(RegistrationWarningFilter{}))
}

func TestRemoveRegistrationWarnings_ScopedFilters(t *testing.T) {
	ctx := context.Background()
	s := NewStore()
	s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{GameID: "g1", Env: "prod", AgentID: "a1", FunctionID: "f1", Code: "c", Message: "m"})
	s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{GameID: "g1", Env: "dev", AgentID: "a2", FunctionID: "f2", Code: "c", Message: "m"})

	// env 过滤只删 dev
	assert.Equal(t, 1, s.RemoveRegistrationWarnings(RegistrationWarningFilter{Env: "dev"}))
	// function 过滤
	assert.Equal(t, 1, s.RemoveRegistrationWarnings(RegistrationWarningFilter{FunctionID: "f1"}))
}

func TestMetricsStore_SetOnReport(t *testing.T) {
	s := NewMetricsStore()
	s.SetOnReport(func(ctx context.Context, agentID string, report *opsv1.MetricsReport) {})
	// SetOnReport 仅设置回调；再次覆盖为 nil 不 panic
	s.SetOnReport(nil)
	assert.NotNil(t, s)
}
