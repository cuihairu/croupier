/** 组合页编辑器 V3：页面组件树模型（编辑视图）与纯函数树操作。 */

export type ComponentType =
  'fnTable' | 'fnForm' | 'fnFields' | 'staticForm' | 'button' | 'modal' | 'container' | 'text';

/** 事件动作：目标一律是节点 id（openModal→modal 节点；runBinding/refreshNode→fn* 节点）。 */
export type ActionSpec =
  | { kind: 'openModal'; target: string }
  | { kind: 'runBinding'; target: string }
  | { kind: 'refreshNode'; target: string };

export type PageNode = {
  id: string;
  type: ComponentType;
  props: Record<string, unknown>;
  /** 仅 container/modal 有 children（V1：container 一层、modal 只装一个 fnForm）。 */
  children?: PageNode[];
};

let counter = 0;
/** 稳定且可读的节点 id（测试友好；不依赖 nanoid 依赖）。 */
export function nodeId(prefix = 'n'): string {
  counter += 1;
  return `${prefix}-${Date.now().toString(36)}${counter.toString(36)}`;
}

export function findNode(nodes: PageNode[], id: string): PageNode | undefined {
  for (const n of nodes) {
    if (n.id === id) return n;
    if (n.children) {
      const hit = findNode(n.children, id);
      if (hit) return hit;
    }
  }
  return undefined;
}

export function findParent(nodes: PageNode[], id: string): PageNode[] | undefined {
  for (const n of nodes) {
    if (n.id === id) return nodes;
    if (n.children) {
      const hit = findParent(n.children, id);
      if (hit) return hit;
    }
  }
  return undefined;
}

/** 插入到指定父节点 children 末尾（parentId 为空则插入根级）。 */
export function insertNode(nodes: PageNode[], node: PageNode, parentId?: string): PageNode[] {
  if (!parentId) return [...nodes, node];
  const walk = (list: PageNode[]): PageNode[] =>
    list.map((n) => {
      if (n.id === parentId) return { ...n, children: [...(n.children ?? []), node] };
      if (n.children) return { ...n, children: walk(n.children) };
      return n;
    });
  return walk(nodes);
}

/** 插入到同级指定节点之后（全树搜索）。 */
export function insertAfter(nodes: PageNode[], node: PageNode, afterId: string): PageNode[] {
  const walk = (list: PageNode[]): PageNode[] => {
    const idx = list.findIndex((n) => n.id === afterId);
    if (idx !== -1) {
      const next = [...list];
      next.splice(idx + 1, 0, node);
      return next;
    }
    return list.map((n) => (n.children ? { ...n, children: walk(n.children) } : n));
  };
  return walk(nodes);
}

/** 删除节点（含子树），并清理树内指向被删节点的悬空绑定：
 * 事件动作（onClick/onSuccess 等 ActionSpec 值）target 失效 → 移除该动作；
 * 动作链 chain 中失效步骤剔除；表格 rowActions 的 targetSection 失效 → 移除该行操作。
 * 返回 [新树, 是否删除]；no-op 保持原引用。 */
export function removeNode(nodes: PageNode[], id: string): [PageNode[], boolean] {
  if (!findNode(nodes, id)) return [nodes, false];
  const walk = (list: PageNode[]): PageNode[] =>
    list
      .filter((n) => n.id !== id)
      .map((n) => (n.children ? { ...n, children: walk(n.children) } : n));
  return [pruneDanglingBindings(walk(nodes)), true];
}

/** 清理树内悬空动作绑定（目标 id 不在树中）。 */
export function pruneDanglingBindings(nodes: PageNode[]): PageNode[] {
  const alive = new Set<string>();
  const collect = (list: PageNode[]) => {
    for (const n of list) {
      alive.add(n.id);
      if (n.children) collect(n.children);
    }
  };
  collect(nodes);

  const cleanAction = (v: unknown): unknown => {
    if (!v || typeof v !== 'object') return v;
    const a = v as { kind?: unknown; target?: unknown; chain?: unknown };
    // 仅识别动作结构（kind 为已知动作类型；与 actions.ts ActionKind 保持一致）
    if (
      typeof a.kind !== 'string' ||
      !['openModal', 'closeModal', 'runBinding', 'refreshNode', 'navigate', 'showMessage'].includes(
        a.kind,
      )
    )
      return v;
    // 目标悬空 → 整个动作作废
    if (typeof a.target === 'string' && a.target && !alive.has(a.target)) return undefined;
    // 动作链剔除悬空步骤
    if (Array.isArray(a.chain)) {
      const chain = (a.chain as Array<{ target?: unknown }>).filter(
        (s) => typeof s?.target !== 'string' || !s.target || alive.has(s.target),
      );
      if (chain.length !== (a.chain as unknown[]).length) {
        return { ...a, chain: chain.length ? chain : undefined };
      }
    }
    return v;
  };

  const cleanNode = (n: PageNode): PageNode => {
    let props = n.props;
    let changed = false;
    const next: Record<string, unknown> = { ...props };
    for (const [k, v] of Object.entries(props)) {
      if (k === 'rowActions' && Array.isArray(v)) {
        // 行操作：targetSection 指向弹窗，悬空则移除该操作项
        const kept = (v as Array<{ targetSection?: string }>).filter(
          (a) => !a?.targetSection || alive.has(a.targetSection),
        );
        if (kept.length !== v.length) {
          next[k] = kept;
          changed = true;
        }
        continue;
      }
      const cleaned = cleanAction(v);
      if (cleaned !== v) {
        if (cleaned === undefined) delete next[k];
        else next[k] = cleaned;
        changed = true;
      }
    }
    if (changed) props = next as PageNode['props'];
    const children = n.children?.map(cleanNode);
    if (!changed && children === n.children) return n;
    return { ...n, props, ...(children ? { children } : {}) };
  };
  return nodes.map(cleanNode);
}

/** 复制节点（含子树），插入到原节点之后，新 id 统一重生成。 */
export function duplicateNode(nodes: PageNode[], id: string): PageNode[] {
  const walk = (list: PageNode[]): PageNode[] => {
    const idx = list.findIndex((n) => n.id === id);
    if (idx !== -1) {
      const copy = cloneWithNewIds(list[idx]);
      const next = [...list];
      next.splice(idx + 1, 0, copy);
      return next;
    }
    return list.map((n) => (n.children ? { ...n, children: walk(n.children) } : n));
  };
  return walk(nodes);
}

/** 复制子树：深拷贝后统一重生成 id，并重映射**子树内部**引用——
 * on* 事件动作主动作/链步骤的 target、refreshOnNode、inputAssignments.sourceNodeId、
 * rowActions.targetSection 指向副本节点；指向子树外部的引用保持原样
 * （idMap 未命中即保留原值）。声明 sectionKey 不继承（调用方 assignVarNames
 * 重新命名）；对变量的引用（表达式/裸引用）也保持指向原变量。 */
function cloneWithNewIds(n: PageNode): PageNode {
  const copy = structuredCloneCompat(n);
  const idMap = new Map<string, string>();
  const preassign = (node: PageNode) => {
    idMap.set(node.id, nodeId(node.type));
    node.children?.forEach(preassign);
  };
  preassign(copy);
  /** id 引用重映射：子树内 → 副本 id；子树外 → 原样保留。 */
  const remapId = (v: unknown): unknown => (typeof v === 'string' ? (idMap.get(v) ?? v) : v);
  const remap = (node: PageNode) => {
    // 剥离声明 key（副本不继承，与原节点冲突；调用方重新命名）
    const { sectionKey: _dropped, ...rest } = node.props as Record<string, unknown>;
    void _dropped;
    node.props = rest as PageNode['props'];
    node.id = idMap.get(node.id) ?? nodeId(node.type);
    const props = node.props as Record<string, unknown>;
    for (const key of Object.keys(props)) {
      const val = props[key];
      if (!val || typeof val !== 'object' || Array.isArray(val)) continue;
      // on* 事件动作：主动作 target + 链步骤 target
      if (key.startsWith('on')) {
        const act = val as { target?: unknown; chain?: Array<{ target?: unknown }> };
        if (typeof act.target === 'string') act.target = remapId(act.target);
        for (const step of Array.isArray(act.chain) ? act.chain : []) {
          if (typeof step.target === 'string') step.target = remapId(step.target);
        }
      }
    }
    // 数组形态引用（refreshOnNode / inputAssignments / rowActions）
    if (Array.isArray(props.refreshOnNode)) {
      props.refreshOnNode = (props.refreshOnNode as unknown[]).map(remapId);
    }
    if (Array.isArray(props.inputAssignments)) {
      props.inputAssignments = (props.inputAssignments as Array<Record<string, unknown>>).map(
        (m) => ({ ...m, sourceNodeId: remapId(m.sourceNodeId) }),
      );
    }
    if (Array.isArray(props.rowActions)) {
      props.rowActions = (props.rowActions as Array<Record<string, unknown>>).map((ra) => ({
        ...ra,
        targetSection: remapId(ra.targetSection),
      }));
    }
    node.children?.forEach(remap);
  };
  remap(copy);
  return copy;
}
/** 定位 prev→next 之间新插入的子树根（复制/外部插入场景；先序首个新节点）。 */
export function findInsertedSubtree(prev: PageNode[], next: PageNode[]): PageNode | undefined {
  const ids = new Set<string>();
  const collect = (l: PageNode[]) =>
    l.forEach((n) => {
      ids.add(n.id);
      if (n.children) collect(n.children);
    });
  collect(prev);
  let found: PageNode | undefined;
  const walk = (l: PageNode[]): boolean => {
    for (const n of l) {
      if (!ids.has(n.id)) {
        found = n;
        return true;
      }
      if (n.children && walk(n.children)) return true;
    }
    return false;
  };
  walk(next);
  return found;
}

/** 按 id 替换子树（引用相等即整树返回）。 */
export function replaceSubtree(nodes: PageNode[], next: PageNode): PageNode[] {
  const walk = (l: PageNode[]): PageNode[] =>
    l.map((n) => (n.id === next.id ? next : n.children ? { ...n, children: walk(n.children) } : n));
  const out = walk(nodes);
  return out === nodes ? [...out] : out;
}

/** 同级移动（drag 重排）。 */
export function moveNode(nodes: PageNode[], id: string, toIndex: number): PageNode[] {
  const walk = (list: PageNode[]): PageNode[] => {
    const idx = list.findIndex((n) => n.id === id);
    if (idx !== -1) {
      if (toIndex < 0 || toIndex >= list.length || idx === toIndex) return list;
      const next = [...list];
      const [item] = next.splice(idx, 1);
      next.splice(toIndex, 0, item);
      return next;
    }
    return list.map((n) => (n.children ? { ...n, children: walk(n.children) } : n));
  };
  return walk(nodes);
}

/** 更新节点 props（浅合并）。无实际变更时返回原引用——setTree 以引用相等
 * 短路 no-op，避免空 patch 污染撤销栈（历史快照被挤出）。 */
export function updateProps(
  nodes: PageNode[],
  id: string,
  patch: Record<string, unknown>,
): PageNode[] {
  const walk = (list: PageNode[]): PageNode[] => {
    let changed = false;
    const next = list.map((n) => {
      if (n.id === id) {
        let hit = false;
        const props = { ...(n.props as Record<string, unknown>) };
        for (const [k, v] of Object.entries(patch)) {
          if (!Object.is(props[k], v)) {
            props[k] = v;
            hit = true;
          }
        }
        if (!hit) return n;
        changed = true;
        return { ...n, props: props as PageNode['props'] };
      }
      if (n.children) {
        const children = walk(n.children);
        if (children !== n.children) {
          changed = true;
          return { ...n, children };
        }
      }
      return n;
    });
    return changed ? next : list;
  };
  return walk(nodes);
}

export function countNodes(nodes: PageNode[]): number {
  return nodes.reduce((acc, n) => acc + 1 + (n.children ? countNodes(n.children) : 0), 0);
}

/** 供 duplicateNode 使用的结构拷贝（避免直接依赖 structuredClone 的兼容性假设）。 */
function structuredCloneCompat<T>(v: T): T {
  return JSON.parse(JSON.stringify(v)) as T;
}
