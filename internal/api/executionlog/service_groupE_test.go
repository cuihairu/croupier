// 覆盖目标：executionlog service.List 的 ExecutionLogModel.List 存储错误
// 分支（service.go:69-71）。
//
// mine=true 且 ctx 已注入 username 时不触发权限查询，模型 List 即首个
// query，注册 fail-all query 回调可稳定命中该分支。
package executionlog

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestService_List_StoreErrorGroupE(t *testing.T) {
	s, svcCtx, ctx := newExecLogTestService(t, "alice")
	seedExecLog(t, svcCtx, "alice", "mail.send")

	require.NoError(t, svcCtx.DB.Callback().Query().Before("gorm:query").
		Register("groupE_fail_query", func(tx *gorm.DB) {
			_ = tx.AddError(errors.New("forced query failure"))
		}))
	t.Cleanup(func() { _ = svcCtx.DB.Callback().Query().Remove("groupE_fail_query") })

	resp, err := s.List(ctx, &ListRequest{Mine: true})
	require.Error(t, err)
	assert.Nil(t, resp)
}
