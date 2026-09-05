package svc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R1 回归守卫：NewServiceContext 必须把 writer/model 注入 ServiceContext。
// 曾经漏赋值导致线上留痕永远为空（audit 有调用而 execution_logs 恒空）。
func TestExecutionLogWriterWired(t *testing.T) {
	ctx := NewServiceContext(newSvcConfig(t, false))
	require.NotNil(t, ctx)

	assert.NotNil(t, ctx.ExecutionLogWriter, "writer must be wired into ServiceContext")
	assert.NotNil(t, ctx.ExecutionLogModel, "execution log model must be wired")

	// multiGame：router 注入且 writer 持有非 nil 路由
	mg := NewServiceContext(newSvcConfig(t, true))
	require.NotNil(t, mg)
	assert.NotNil(t, mg.ExecutionLogWriter)
	assert.NotNil(t, mg.ExecutionLogModel)
}
