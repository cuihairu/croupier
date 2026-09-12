package hotreload

// handle_summary_gap_test.go 组 K 验收补充：Handle 错误汇总路径的行为
// 回归用例（ScriptHandler 对未注册解释器扩展返回错误，Handle 必须聚合
// 为 "reload handler errors" 返回）。
//
// Handle 93.3% 的唯一残留缺口是 handlers.go:320-322（assetHandler 错误
// append）：AssetHandler.Handle 恒返回 nil——asset hook 失败只记日志
// 不上抛——该分支在当前契约下为死分支，不可覆盖。

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerManagerHandleAggregatesSubHandlerError_K(t *testing.T) {
	m := NewHandlerManager(slog.New(slog.NewTextHandler(&strings.Builder{}, nil)))

	// NewHandlerManager 未注册任何解释器，.unknownext 直接报
	// "no interpreter registered"，驱动 Handle 走 errors 聚合返回。
	err := m.Handle(context.Background(), ReloadEvent{
		Type: ReloadTypeScript,
		Path: "/tmp/k-gap/script.unregisteredext",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reload handler errors")
	assert.Contains(t, err.Error(), "no interpreter registered")

	// 已知类型成功路径与未知类型 warn 路径均已覆盖，此处补齐的
	// 仅是汇总返回分支。
}
