// Agent 显式归属（game/env）共通校验：单 agent 进程绑定单一 (gameId, env)
// scope（作用域规范 §14）。sidecar 入口在解析 YAML 后调用；四类 agent
// (sidecar/devops/capture/synthetics) 的口径一致性由
// agentscope_consistency_test.go 统一验证（todo.md:29）。
//
// 空白拒绝规则与四字段英文文案的单一实现在 core/agentscope；sidecar 侧只
// 校验 game/env 且保留入口自己的中文文案（各入口报错文案属入口职责）。
package agent

import (
	"fmt"

	"github.com/cuihairu/croupier/core/agentscope"
)

// ValidateAgentScope 要求 game/env 去空白后均非空。
func ValidateAgentScope(gameID, env string) error {
	if !agentscope.NonBlank(gameID) || !agentscope.NonBlank(env) {
		return fmt.Errorf("agent.gameId 和 agent.env 为必填（作用域规范 §14，单 agent 单 game+env）")
	}
	return nil
}
