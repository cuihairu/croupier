import type { PageNode } from './model';

/** 事件动作：目标一律是节点 id。 */
export type ActionKind = 'openModal' | 'runBinding' | 'refreshNode';

export type ActionSpec =
  | { kind: 'openModal'; target: string; chain?: ActionStep[] }
  | { kind: 'runBinding'; target: string; chain?: ActionStep[] }
  | { kind: 'refreshNode'; target: string; chain?: ActionStep[] };

/** 动作链后续步骤（仅 runBinding/refreshNode）。 */
export type ActionStep = {
  kind: 'runBinding' | 'refreshNode';
  target: string;
};

export interface ActionDef {
  kind: ActionKind;
  label: string;
  /** 可选目标节点（用于属性面板目标下拉过滤）。 */
  targetFilter: (nodes: PageNode[]) => PageNode[];
}

/** 动作注册表（V1 单动作；动作链 V1.1）。 */
export const ACTIONS: Record<ActionKind, ActionDef> = {
  openModal: {
    kind: 'openModal',
    label: '打开弹窗',
    targetFilter: (nodes) => nodes.filter((n) => n.type === 'modal'),
  },
  runBinding: {
    kind: 'runBinding',
    label: '执行',
    targetFilter: (nodes) => nodes.filter((n) => n.type.startsWith('fn')),
  },
  refreshNode: {
    kind: 'refreshNode',
    label: '刷新',
    targetFilter: (nodes) => nodes.filter((n) => n.type.startsWith('fn')),
  },
};

/** 解析 props 中的动作字段（非法/目标丢失返回 null）。 */
export function parseAction(v: unknown): ActionSpec | null {
  if (!v || typeof v !== 'object') return null;
  const a = v as { kind?: unknown; target?: unknown };
  if (typeof a.kind !== 'string' || !(a.kind in ACTIONS)) return null;
  if (typeof a.target !== 'string' || !a.target) return null;
  return { kind: a.kind as ActionKind, target: a.target };
}

/** 目标节点摘要（下拉 label）。 */
export function nodeSummary(n: PageNode): string {
  return String(n.props.title ?? n.props.functionId ?? n.type);
}
