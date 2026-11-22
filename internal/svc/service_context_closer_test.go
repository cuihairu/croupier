package svc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// AUD-024：RegisterCloser 是服务生命周期后台 worker 的释放钩子。Close 必须
// 调用全部注册的 closer（先于连接池释放），且幂等不重复调用。
func TestServiceContext_CloseRunsRegisteredClosers(t *testing.T) {
	ctx := &ServiceContext{}
	var n int
	ctx.RegisterCloser(func() { n++ })
	ctx.RegisterCloser(func() { n++ })

	ctx.Close()
	require.Equal(t, 2, n, "Close 必须调用全部注册的 closer")

	// 幂等：二次 Close 不得重复调用。
	ctx.Close()
	require.Equal(t, 2, n, "Close 幂等，不得重复调用 closer")
}

func TestServiceContext_RegisterCloserNilSafe(t *testing.T) {
	var nilCtx *ServiceContext
	require.NotPanics(t, func() { nilCtx.RegisterCloser(func() {}) })

	ctx := &ServiceContext{}
	require.NotPanics(t, func() { ctx.RegisterCloser(nil) })
	require.NotPanics(t, func() { ctx.Close() })
}
