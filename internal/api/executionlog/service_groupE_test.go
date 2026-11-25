// Coverage target: the ExecutionLogModel.List store-error branch of
// executionlog service.List (service.go:69-71).
//
// With mine=true and a username already injected into ctx no permission query
// fires, so the model List is the first query; registering a fail-all query
// callback hits that branch reliably.
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
