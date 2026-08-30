import type { PageNode } from './model';

/** 事件动作：目标一律是节点 id。 */
export type ActionKind =
  'openModal' | 'closeModal' | 'runBinding' | 'refreshNode' | 'navigate' | 'showMessage';

export type ActionSpec = {
  kind: ActionKind;
  /** 目标节点 id（openModal=弹窗；run/refresh=函数组件；其余可为空）。 */
  target: string;
  /** 动作参数：navigate={url}；showMessage={message}；run/refresh=参数映射。 */
  params?: Record<string, string>;
  chain?: ActionStep[];
};

/** 动作链后续步骤。 */
export type ActionStep = {
  kind: ActionKind;
  target: string;
  params?: Record<string, string>;
};

export interface ActionDef {
  kind: ActionKind;
  label: string;
  /** 可选目标节点（用于属性面板目标下拉过滤）。 */
  targetFilter: (nodes: PageNode[]) => PageNode[];
}

/** 动作注册表：label/目标过滤/参数字段声明/是否需要目标。 */
export const ACTIONS: Record<
  ActionKind,
  ActionDef & {
    /** 动作自身的参数字段（如 navigate 的 url）。 */
    paramFields?: Array<{ key: string; label: string; placeholder: string }>;
    needsTarget: boolean;
  }
> = {
  openModal: {
    kind: 'openModal',
    label: '打开弹窗',
    targetFilter: (nodes) => nodes.filter((n) => n.type === 'modal'),
    needsTarget: true,
  },
  closeModal: {
    kind: 'closeModal',
    label: '关闭弹窗',
    targetFilter: () => [],
    needsTarget: false,
  },
  runBinding: {
    kind: 'runBinding',
    label: '执行',
    targetFilter: (nodes) => nodes.filter((n) => n.type.startsWith('fn')),
    needsTarget: true,
  },
  refreshNode: {
    kind: 'refreshNode',
    label: '刷新',
    targetFilter: (nodes) => nodes.filter((n) => n.type.startsWith('fn')),
    needsTarget: true,
  },
  navigate: {
    kind: 'navigate',
    label: '跳转链接',
    targetFilter: () => [],
    needsTarget: false,
    paramFields: [{ key: 'url', label: '地址', placeholder: 'https://… 或 /页面路径' }],
  },
  showMessage: {
    kind: 'showMessage',
    label: '提示消息',
    targetFilter: () => [],
    needsTarget: false,
    paramFields: [{ key: 'message', label: '文案', placeholder: '提示内容' }],
  },
};

/** 组件事件注册（WinForms 式：每组件声明自己的事件集）。 */
export interface ComponentEvent {
  /** 事件名（props 键）。 */
  name: string;
  label: string;
}

/** 内置事件 → 各组件声明（builtin 注册处引用）。 */
export const EVENTS = {
  onClick: { name: 'onClick', label: '点击' },
  onRowClick: { name: 'onRowClick', label: '行点击' },
  onRowSelected: { name: 'onRowSelected', label: '行选中' },
  onSuccess: { name: 'onSuccess', label: '执行成功后' },
  onError: { name: 'onError', label: '执行失败时' },
} satisfies Record<string, ComponentEvent>;

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
