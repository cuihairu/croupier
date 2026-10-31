package assignment

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

// EnsureFunctionAssigned 实施分配闸门（OPEN-ISSUES #36 / BUG-032）：
//   - scope（gameID|env）无分配记录 → 默认开放（向后兼容：未配置分配的
//     部署行为与闸门上线前完全一致，存量 invoke 用例即隐式回归）
//   - 有记录 → 白名单：函数必须在列表内，否则 403 function_not_assigned
//     「函数未开放执行权限」；空列表是白名单空集（全拒），清空保存时
//     Update 会删除记录回到默认开放
//
// 与 EnsureFunctionEnabled 同位（functionInvoke / task Start 双入口），
// 拒绝发生在遥测/审计之前——被拒的调用不是一次执行，不污染执行留痕。
// 文件读取失败按错误透传（与 List 的真值表一致，损坏的 assignments.json
// 页面同样不可用），不静默 fail-open。边界：闸门覆盖 dashboard HTTP 调用
// 路径；SDK/agent 走 TCP 直连不经此闸门（transport 层鉴权独立）。
// scope 匹配大小写不敏感（与 filterAssignments 一致）；函数 ID 精确匹配
// （注册表标识符大小写敏感）。
//
// ARCH-019：闸门从 internal/api/assignment 下沉到领域包
// internal/logic/assignment，执行链（functioninvoke）不再 import internal/api，
// 只依赖领域包。真值读取复用本包 LoadAllAssignments。
func EnsureFunctionAssigned(ctx context.Context, svcCtx *svc.ServiceContext, functionID, gameID, env string) error {
	if svcCtx == nil {
		return nil
	}
	assignments, err := LoadAllAssignments(svcCtx)
	if err != nil {
		return err
	}
	functions, ok := lookupScopeList(assignments, gameID, env)
	if !ok {
		return nil // 无分配记录：默认开放
	}
	fnID := strings.TrimSpace(functionID)
	for _, fn := range functions {
		if strings.TrimSpace(fn) == fnID {
			return nil
		}
	}
	return errorx.NewForbiddenWithCode(
		"function_not_assigned",
		"函数未开放执行权限：当前游戏/环境未分配该函数（可在「函数分配」页添加）",
		map[string]any{"functionId": fnID, "gameId": gameID, "env": env},
	)
}

// lookupScopeList 按 EqualFold 匹配 scope key 两段，命中返回列表与 true。
func lookupScopeList(assignments map[string][]string, gameID, env string) ([]string, bool) {
	for key, functions := range assignments {
		g, e := splitAssignmentKey(key)
		if strings.EqualFold(g, strings.TrimSpace(gameID)) && strings.EqualFold(e, strings.TrimSpace(env)) {
			return functions, true
		}
	}
	return nil, false
}
