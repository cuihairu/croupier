/** 预览运行时共享类型与响应归一工具（引擎 PreviewRuntime 与渲染子组件
 * PreviewNode 公用）：响应归一 payloadOf、表格条目 itemsOf、树查找 findIn。 */

import type { PageNode } from './model';

/** 预览侧统一的对象记录形态（函数响应/行数据/表单值）。 */
export type JSONRecord = Record<string, unknown>;

/** 函数响应归一：FunctionInvokeResponse 的 result/data 才是函数输出。 */
export function payloadOf(resp: unknown): JSONRecord {
  const r = resp as JSONRecord | undefined;
  if (!r) return {};
  const inner = (r.result ?? r.data) as JSONRecord | undefined;
  return inner && typeof inner === 'object' ? inner : r;
}

/** 表格数据条目：payload.items（非数组输出为空表）。 */
export function itemsOf(payload: JSONRecord): JSONRecord[] {
  const items = payload.items;
  return Array.isArray(items) ? (items as JSONRecord[]) : [];
}

/** 动作步骤的宽松形态（主动作 ActionSpec 与链步骤 ActionStep 共用）。 */
export type StepLike = { kind?: string; target?: string; params?: Record<string, string> };

/** 按 id 递归查找节点（含弹窗/容器子树）。 */
export function findIn(nodes: PageNode[], id: string): PageNode | undefined {
  for (const n of nodes) {
    if (n.id === id) return n;
    if (n.children) {
      const hit = findIn(n.children, id);
      if (hit) return hit;
    }
  }
  return undefined;
}
