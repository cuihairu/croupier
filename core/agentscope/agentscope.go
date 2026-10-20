// Package agentscope 集中四类 Agent 显式归属（game/env 等）注册身份字段的
// 空白拒绝规则：去空白即拒、按声明顺序报第一个缺项。各入口保留自己的文案
// 与调用时机；本包只提供单一规则实现（game-environment-scope §14.7）。
package agentscope

import (
	"fmt"
	"strings"
)

// NonBlank reports whether v keeps any non-whitespace content.
func NonBlank(v string) bool { return strings.TrimSpace(v) != "" }

// Identity 按声明顺序校验注册身份四字段（id/serverAddr/gameId/env），缺项报
// "agent.<field> is required"（devops/capture/synthetics 的既有英文文案口径）。
func Identity(agentID, serverAddr, gameID, env string) error {
	fields := []struct{ name, value string }{
		{"id", agentID},
		{"serverAddr", serverAddr},
		{"gameId", gameID},
		{"env", env},
	}
	for _, f := range fields {
		if !NonBlank(f.value) {
			return fmt.Errorf("agent.%s is required", f.name)
		}
	}
	return nil
}
