package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// stubRunAgentFuncStuck 注入永不返回的 runAgent 替身：done 永不关闭，用于
// 驱动 Stop 的超时分支（runner 卡住）。goroutine 泄漏在本用例生命周期内
// 无害（不碰全局、不写 stdout）。
func stubRunAgentFuncStuck(t *testing.T) {
	t.Helper()
	old := runAgentFunc
	runAgentFunc = func() error { select {} }
	t.Cleanup(func() { runAgentFunc = old })
}

// Stop 超时（runner 卡住）：done 未在 stopTimeout 内关闭 → 返回可识别超时
// 错误并记录 runner 未退出，绝不伪装成干净停机（AUD-004）。
func TestAgentServiceStop_Timeout(t *testing.T) {
	stubRunAgentFuncStuck(t)

	svc := newAgentService("")
	svc.stopTimeout = 50 * time.Millisecond
	fake := newFakeService()
	require.NoError(t, svc.Start(fake))

	err := svc.Stop(fake)
	require.ErrorContains(t, err, "停止超时")
	require.ErrorContains(t, err, "后台 runner 未退出")

	// 重复 Stop：runner 仍卡住 → 仍如实报超时（不返回 nil）。
	require.ErrorContains(t, svc.Stop(fake), "停止超时")
}

// 正常完成：runAgent 返回 nil → done 关闭 → Stop 返回 nil；重复 Stop 也返回
// nil（done 已收口，快速路径立即返回，不等待）。
func TestAgentServiceStop_NormalAndRepeat(t *testing.T) {
	stubRunAgentFunc(t, nil)

	svc := newAgentService("")
	fake := newFakeService()
	require.NoError(t, svc.Start(fake))

	// 等后台 runner 收口（done 关闭）。
	select {
	case <-svc.done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Start.func1 to finish")
	}

	require.NoError(t, svc.Stop(fake))
	require.NoError(t, svc.Stop(fake)) // 重复 Stop
}
