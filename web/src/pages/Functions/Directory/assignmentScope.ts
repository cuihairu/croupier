/**
 * 开放范围统计（纯函数）：把 assignments 白名单（scope key "gameId|env" →
 * 函数列表）折算成逐函数的开放环境计数。
 *
 * 语义（与后端执行闸门 EnsureFunctionAssigned 一致）：
 * - total = 该游戏已保存白名单的环境数（fetchAssignments 按 scope 头过滤，
 *   返回的 map 只含当前游戏的环境键）；
 * - open = 其中开放了该函数（出现在白名单内）的环境数；不在任何白名单
 *   内且 total>0 的函数是 0/total（未开放）；
 * - 传入 undefined（拉取失败）→ 返回 undefined，调用方显示未知态；
 * - 空表（该游戏从未保存过白名单）→ total=0，表示「默认开放」。
 */
export type AssignmentScopeIndex = {
  /** 已保存白名单的环境数 */
  total: number;
  /** 函数 ID → 开放该函数的环境数 */
  openByFunction: Map<string, number>;
};

export function computeAssignmentScopes(
  assignments: Record<string, string[]> | undefined,
): AssignmentScopeIndex | undefined {
  if (!assignments) return undefined;
  const keys = Object.keys(assignments);
  const openByFunction = new Map<string, number>();
  for (const functions of Object.values(assignments)) {
    if (!Array.isArray(functions)) continue;
    for (const fnId of functions) {
      if (!fnId) continue;
      openByFunction.set(fnId, (openByFunction.get(fnId) || 0) + 1);
    }
  }
  return { total: keys.length, openByFunction };
}
