import type { MenuItem } from '@/services/api/menu';
import { computeDragUpdates } from '../sortUtils';
import { buildDropHandler } from '../MenuTree';

function node(id: number, children: MenuItem[] = [], parentId: number | null = null): MenuItem {
  return {
    id,
    parentId,
    menuKey: `key-${id}`,
    labels: { 'zh-CN': `菜单${id}` },
    sortOrder: 0,
    isVisible: true,
    children,
  };
}

function withParent(nodes: MenuItem[], parent: number | null): MenuItem[] {
  return nodes.map((n) => ({ ...n, parentId: parent, children: withParent(n.children, n.id) }));
}

// 结构：
// 顶级: A(1) B(2)
// A 下: C(3) D(4)
function buildTree(): MenuItem[] {
  return withParent([node(1, [node(3), node(4)]), node(2)], null);
}

describe('computeDragUpdates', () => {
  it('同级内 before 重排：顺序号整体重写', () => {
    const items = buildTree();
    // 把 D(4) 拖到 C(3) 之前（同级）
    const updates = computeDragUpdates(items, { dragKey: 4, dropKey: 3, position: 'before' });
    expect(updates).toEqual([
      { id: 4, parentId: 1, sortOrder: 1 },
      { id: 3, parentId: 1, sortOrder: 2 },
    ]);
  });

  it('同级内 after 重排', () => {
    const items = buildTree();
    // 把 C(3) 拖到 D(4) 之后 → D C
    const updates = computeDragUpdates(items, { dragKey: 3, dropKey: 4, position: 'after' });
    expect(updates).toEqual([
      { id: 4, parentId: 1, sortOrder: 1 },
      { id: 3, parentId: 1, sortOrder: 2 },
    ]);
  });

  it('inside：拖入节点成为其子级（parentId 变化 + 两级顺序重写）', () => {
    const items = buildTree();
    // 把顶级 B(2) 拖进 A(1) 内部
    const updates = computeDragUpdates(items, { dragKey: 2, dropKey: 1, position: 'inside' });
    expect(updates).toEqual([
      // 原父级（顶级）剩余节点重排
      { id: 1, parentId: null, sortOrder: 1 },
      // 目标父级子级重排 + 移入节点排末尾
      { id: 3, parentId: 1, sortOrder: 1 },
      { id: 4, parentId: 1, sortOrder: 2 },
      { id: 2, parentId: 1, sortOrder: 3 },
    ]);
    const moved = updates?.find((u) => u.id === 2);
    expect(moved?.parentId).toBe(1);
  });

  it('跨父级 before：新旧父级顺序都重写', () => {
    const items = buildTree();
    // 把 A 下的 C(3) 拖到顶级 B(2) 之前
    const updates = computeDragUpdates(items, { dragKey: 3, dropKey: 2, position: 'before' });
    expect(updates).toEqual([
      { id: 4, parentId: 1, sortOrder: 1 },
      { id: 1, parentId: null, sortOrder: 1 },
      { id: 3, parentId: null, sortOrder: 2 },
      { id: 2, parentId: null, sortOrder: 3 },
    ]);
  });

  it('拖到自身子孙上返回 null（防环）', () => {
    const items = buildTree();
    expect(computeDragUpdates(items, { dragKey: 1, dropKey: 3, position: 'inside' })).toBeNull();
    expect(computeDragUpdates(items, { dragKey: 1, dropKey: 4, position: 'after' })).toBeNull();
  });

  it('拖到自身返回 null；未知 key 返回 null', () => {
    const items = buildTree();
    expect(computeDragUpdates(items, { dragKey: 1, dropKey: 1, position: 'after' })).toBeNull();
    expect(computeDragUpdates(items, { dragKey: 99, dropKey: 1, position: 'after' })).toBeNull();
    expect(computeDragUpdates(items, { dragKey: 1, dropKey: 99, position: 'inside' })).toBeNull();
  });

  it('不影响未受影响的兄弟分支', () => {
    const items = buildTree();
    // C(3) 在 D(4) 后 → 同级交换，B(2) 不出现在更新里
    const updates = computeDragUpdates(items, { dragKey: 3, dropKey: 4, position: 'after' });
    expect(updates?.map((u) => u.id)).not.toContain(2);
  });
});

describe('buildDropHandler（MenuTree onDrop 胶水）', () => {
  it('dropToGap + dropPosition 相对节点 pos 解析 before/after', () => {
    const items = buildTree();
    const onMove = jest.fn();
    const handler = buildDropHandler(items, onMove);

    // 节点 D(4) pos '0-0-1'：dropPosition = 1 → relative = 1-1 = 0 → after
    handler({
      node: { key: '4', pos: '0-0-1' },
      dragNode: { key: '3' },
      dropPosition: 1,
      dropToGap: true,
    });
    expect(onMove).toHaveBeenCalledWith(
      expect.arrayContaining([
        { id: 4, parentId: 1, sortOrder: 1 },
        { id: 3, parentId: 1, sortOrder: 2 },
      ]),
    );
    onMove.mockClear();

    // dropPosition = 0 → relative = 0-1 = -1 → before
    handler({
      node: { key: '4', pos: '0-0-1' },
      dragNode: { key: '3' },
      dropPosition: 0,
      dropToGap: true,
    });
    expect(onMove).toHaveBeenCalledWith(
      expect.arrayContaining([
        { id: 3, parentId: 1, sortOrder: 1 },
        { id: 4, parentId: 1, sortOrder: 2 },
      ]),
    );
  });

  it('dropToGap=false 归一为 inside；非法拖拽不触发 onMove', () => {
    const items = buildTree();
    const onMove = jest.fn();
    const handler = buildDropHandler(items, onMove);

    // 拖到自身子孙（把 A 拖到 C 上）→ 拒绝
    handler({
      node: { key: '3', pos: '0-0-0' },
      dragNode: { key: '1' },
      dropPosition: 0,
      dropToGap: false,
    });
    expect(onMove).not.toHaveBeenCalled();

    // 拖到自身 → 拒绝
    handler({
      node: { key: '1', pos: '0-0' },
      dragNode: { key: '1' },
      dropPosition: 1,
      dropToGap: true,
    });
    expect(onMove).not.toHaveBeenCalled();

    // inside：把 B(2) 拖进 A(1)
    handler({
      node: { key: '1', pos: '0-0' },
      dragNode: { key: '2' },
      dropPosition: 0,
      dropToGap: false,
    });
    expect(onMove).toHaveBeenCalledTimes(1);
    const updates = onMove.mock.calls[0][0];
    expect(updates).toEqual(
      expect.arrayContaining([expect.objectContaining({ id: 2, parentId: 1 })]),
    );
  });

  it('无变化（放在原位）不触发 onMove', () => {
    const items = buildTree();
    // 夹具 sortOrder 归位为真实值 1..2（过滤逻辑比较原 sortOrder）
    items[0].children[0].sortOrder = 1;
    items[0].children[1].sortOrder = 2;
    const onMove = jest.fn();
    const handler = buildDropHandler(items, onMove);
    // C(3) 拖到 D(4) 之前 = 原位
    handler({
      node: { key: '4', pos: '0-0-1' },
      dragNode: { key: '3' },
      dropPosition: 0,
      dropToGap: true,
    });
    expect(onMove).not.toHaveBeenCalled();
  });
});
