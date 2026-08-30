import {
  countNodes,
  duplicateNode,
  findNode,
  findParent,
  insertAfter,
  insertNode,
  moveNode,
  nodeId,
  removeNode,
  updateProps,
  type PageNode,
} from '../model';

function n(type: string, id: string, children?: PageNode[]): PageNode {
  return { id, type: type as PageNode['type'], props: { title: id }, children };
}

const tree: PageNode[] = [
  n('text', 't1'),
  n('container', 'c1', [n('fnTable', 'tbl1'), n('button', 'btn1')]),
  n('fnForm', 'f1'),
];

describe('editor v3 model', () => {
  it('findNode/findParent 深度查找', () => {
    expect(findNode(tree, 'tbl1')?.type).toBe('fnTable');
    expect(findNode(tree, 'nope')).toBeUndefined();
    expect(findParent(tree, 'btn1')).toEqual([tree[1].children![0], tree[1].children![1]]);
    expect(findParent(tree, 't1')).toBe(tree);
  });

  it('insertNode 根级与容器内', () => {
    const root = insertNode(tree, n('text', 'x1'));
    expect(root).toHaveLength(4);
    const inC1 = insertNode(tree, n('text', 'x2'), 'c1');
    expect(findNode(inC1, 'c1')?.children).toHaveLength(3);
  });

  it('insertAfter 同级定位（含子层）', () => {
    const afterBtn = insertAfter(tree, n('text', 'x3'), 'btn1');
    expect(findNode(afterBtn, 'c1')?.children?.map((c) => c.id)).toEqual(['tbl1', 'btn1', 'x3']);
    const afterT1 = insertAfter(tree, n('text', 'x4'), 't1');
    expect(afterT1.map((x) => x.id)).toEqual(['t1', 'x4', 'c1', 'f1']);
  });

  it('removeNode 含子树', () => {
    const [next, removed] = removeNode(tree, 'c1');
    expect(removed).toBe(true);
    expect(countNodes(next)).toBe(2);
    const [same, notRemoved] = removeNode(tree, 'nope');
    expect(notRemoved).toBe(false);
    expect(same).toBe(tree);
  });

  it('duplicateNode 深拷贝+新 id，插到原节点后', () => {
    const next = duplicateNode(tree, 'c1');
    const idx = next.findIndex((x) => x.id === 'c1');
    expect(next[idx + 1].id).not.toBe('c1');
    const copyKids = next[idx + 1].children?.map((c) => c.id) ?? [];
    expect(copyKids).toHaveLength(2);
    expect(copyKids).not.toContain('tbl1'); // 子节点 id 也已重生成
    // 原 id 不受污染
    expect(findNode(next, 'tbl1')).toBeDefined();
  });

  it('moveNode 同级重排（根级与容器内）', () => {
    expect(moveNode(tree, 'f1', 0).map((x) => x.id)).toEqual(['f1', 't1', 'c1']);
    const inner = moveNode(tree, 'btn1', 0);
    expect(findNode(inner, 'c1')?.children?.map((c) => c.id)).toEqual(['btn1', 'tbl1']);
    // 越界 no-op
    expect(moveNode(tree, 't1', 99)).toBe(tree);
  });

  it('updateProps 浅合并不动兄弟', () => {
    const next = updateProps(tree, 'tbl1', { span: 12 });
    expect(findNode(next, 'tbl1')?.props).toEqual({ title: 'tbl1', span: 12 });
    expect(findNode(next, 'btn1')?.props).toEqual({ title: 'btn1' });
  });

  it('nodeId 唯一', () => {
    const ids = new Set(Array.from({ length: 100 }, () => nodeId('a')));
    expect(ids.size).toBe(100);
  });
});
