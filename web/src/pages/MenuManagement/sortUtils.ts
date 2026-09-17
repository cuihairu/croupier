import type { MenuItem } from '@/services/api/menu';

/**
 * 菜单树拖拽排序的纯逻辑计算：把 antd Tree 的 onDrop 语义翻译成
 * 受影响节点的新 (parentId, sortOrder) 列表，供 API 顺序提交。
 * 纯函数无副作用，便于单元测试。
 */

/** MenuTree 归一后的放置语义。 */
export type DropPosition = 'before' | 'after' | 'inside';

export interface DragDropPlan {
  dragKey: number;
  dropKey: number;
  position: DropPosition;
}

/** 单个节点的新位置（parentId=null 表示顶级）。 */
export interface SortUpdate {
  id: number;
  parentId: number | null;
  sortOrder: number;
}

interface FlatNode {
  node: MenuItem;
  parent: number | null;
}

function flatten(items: MenuItem[]): Map<number, FlatNode> {
  const out = new Map<number, FlatNode>();
  const walk = (nodes: MenuItem[], parent: number | null) => {
    for (const node of nodes) {
      out.set(node.id, { node, parent });
      walk(node.children, node.id);
    }
  };
  walk(items, null);
  return out;
}

/** dropKey 是否是 dragKey 的子孙（拖入自身子树会成环，必须拒绝）。 */
function isDescendant(items: MenuItem[], ancestorKey: number, targetKey: number): boolean {
  const ancestor = flatten(items).get(ancestorKey)?.node;
  if (!ancestor) return false;
  const walk = (nodes: MenuItem[]): boolean =>
    nodes.some((n) => n.id === targetKey || walk(n.children));
  return walk(ancestor.children);
}

/**
 * 计算拖拽后的位置更新。返回 null 表示非法或无操作；
 * 返回列表覆盖受影响父级的全部子节点（顺序号重排为 1..n）。
 */
export function computeDragUpdates(items: MenuItem[], plan: DragDropPlan): SortUpdate[] | null {
  const flat = flatten(items);
  const drag = flat.get(plan.dragKey);
  const drop = flat.get(plan.dropKey);
  if (!drag || !drop) return null;
  if (plan.dragKey === plan.dropKey) return null;
  if (isDescendant(items, plan.dragKey, plan.dropKey)) return null;

  // 以「父级 → 有序子节点列表」的快照模拟移动
  const childrenLists = new Map<number | null, MenuItem[]>();
  const collect = (nodes: MenuItem[], parent: number | null) => {
    childrenLists.set(parent, [...nodes]);
    for (const node of nodes) collect(node.children, node.id);
  };
  collect(items, null);
  if (!childrenLists.has(null)) childrenLists.set(null, []);

  const oldParent = drag.parent;
  const targetParent = plan.position === 'inside' ? plan.dropKey : drop.parent;

  const oldList = childrenLists.get(oldParent) ?? [];
  childrenLists.set(
    oldParent,
    oldList.filter((n) => n.id !== plan.dragKey),
  );

  const targetList = childrenLists.get(targetParent) ?? [];
  const dropIndex = targetList.findIndex((n) => n.id === plan.dropKey);
  const insertAt =
    plan.position === 'inside'
      ? targetList.length
      : plan.position === 'before'
        ? dropIndex
        : dropIndex + 1;
  targetList.splice(insertAt < 0 ? targetList.length : insertAt, 0, drag.node);
  childrenLists.set(targetParent, targetList);

  const updates: SortUpdate[] = [];
  const emit = (parent: number | null) => {
    const list = childrenLists.get(parent) ?? [];
    list.forEach((node, index) => {
      const next: SortUpdate = { id: node.id, parentId: parent, sortOrder: index + 1 };
      const before = flat.get(node.id);
      // 过滤无变化节点：parentId 与 sortOrder 均未变则无需提交
      if (before && before.parent === next.parentId && before.node.sortOrder === next.sortOrder) {
        return;
      }
      updates.push(next);
    });
  };
  emit(oldParent);
  if (targetParent !== oldParent) emit(targetParent);
  return updates;
}
