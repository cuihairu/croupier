package assignment

// R52 覆盖率补缺：gate.go 两翼（BUG-035 域 2026-09-27 收口后回避解除）
//   - :34-35 EnsureFunctionAssigned 的 svcCtx==nil 守卫——nil 即默认开放，
//     且守卫必须先于 LoadScopeAssignments（否则 assignmentsPath(nil) 之后
//     的真值读取无从谈起）。
//   - :38-39 LoadScopeAssignments 读故障透传——坏 JSON 触发 loadAssignments
//     的 Unmarshal 错误，闸门契约「不静默 fail-open」的直接断言（gate.go
//     文件头注释承诺：文件读取失败按错误透传，与 List 真值表一致）。

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureFunctionAssigned_NilSvcCtxOpenByDefault(t *testing.T) {
	assert.NoError(t, EnsureFunctionAssigned(context.Background(), nil, "demo.echo", "demo", "prod"),
		"svcCtx 为 nil 应默认开放（守卫在读取真值之前）")
}

func TestEnsureFunctionAssigned_CorruptFilePropagatesError(t *testing.T) {
	svcCtx := newGateSvcCtx(t, t.TempDir())
	writeAssignments(t, svcCtx, "{not-json")

	err := EnsureFunctionAssigned(context.Background(), svcCtx, "demo.echo", "demo", "prod")
	require.Error(t, err, "assignments.json 损坏必须透传错误，不得静默 fail-open")
	assert.Contains(t, err.Error(), "invalid character",
		"应为 JSON 解析错误而非其他读取故障")
}
