// Agent 显式归属（game/env）共通校验：单 agent 进程绑定单一 (gameId, env)
// scope（作用域规范 §14）。sidecar 入口在解析 YAML 后调用；四类 agent
// (sidecar/devops/capture/synthetics) 的口径一致性由
// agentscope_consistency_test.go 统一验证（todo.md:29）。
//
// agents/* 不 import internal/ 的配置栈（与 agents/*/config 同构口径），
// 各自的 Validate 独立实现同一约束：缺失/空白即确定拒绝，不隐式回落
// dev/prod/default。
package agent

import (
	"fmt"
	"strings"
)

// ValidateAgentScope 要求 game/env 去空白后均非空。
func ValidateAgentScope(gameID, env string) error {
	if strings.TrimSpace(gameID) == "" || strings.TrimSpace(env) == "" {
		return fmt.Errorf("agent.gameId 和 agent.env 为必填（作用域规范 §14，单 agent 单 game+env）")
	}
	return nil
}
