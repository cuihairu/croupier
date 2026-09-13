import {
  countNodes,
  duplicateNode,
  findInsertedSubtree,
  findNode,
  findParent,
  insertAfter,
  insertNode,
  moveNode,
  nodeId,
  pruneDanglingBindings,
  removeNode,
  replaceSubtree,
  scaffoldTabsNode,
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

  it('removeNode 清理指向被删节点的悬空绑定', () => {
    const withModal: PageNode[] = [
      n('modal', 'm1', [n('fnForm', 'ff1')]),
      {
        ...n('button', 'b1'),
        props: {
          title: 'b1',
          onClick: { kind: 'openModal', target: 'm1' },
        },
      },
      {
        ...n('button', 'b2'),
        props: {
          title: 'b2',
          // 动作链部分步骤悬空 → 仅剔除悬空步骤
          onClick: {
            kind: 'runBinding',
            target: 'ff1',
            chain: [
              { kind: 'refreshNode', target: 'f1' },
              { kind: 'refreshNode', target: 'm1' },
            ],
          },
        },
      },
      {
        ...n('fnTable', 't2'),
        props: {
          title: 't2',
          functionId: 'player.list',
          rowActions: [
            { label: '发邮件', targetSection: 'm1', params: {}, danger: false },
            { label: '详情', targetSection: '', params: {}, danger: false },
          ],
        },
      },
    ];
    const [next, removed] = removeNode(withModal, 'm1');
    expect(removed).toBe(true);
    // 弹窗及其子树消失
    expect(findNode(next, 'm1')).toBeUndefined();
    expect(findNode(next, 'ff1')).toBeUndefined();
    // b1 的 onClick 目标悬空 → 动作被清除（徽标恢复「点击绑定动作」）
    expect(findNode(next, 'b1')?.props.onClick).toBeUndefined();
    // b2 的 onClick target=ff1 也随子树悬空 → 清除
    expect(findNode(next, 'b2')?.props.onClick).toBeUndefined();
    // 行操作 targetSection 悬空项被移除，无目标项保留
    const ra = findNode(next, 't2')?.props.rowActions as Array<{ label: string }>;
    expect(ra.map((a) => a.label)).toEqual(['详情']);
  });

  it('removeNode 保留未悬空的绑定', () => {
    const t: PageNode[] = [
      n('fnForm', 'f9'),
      {
        ...n('button', 'b9'),
        props: {
          title: 'b9',
          onClick: {
            kind: 'runBinding',
            target: 'f9',
            chain: [{ kind: 'refreshNode', target: 'f9' }],
          },
        },
      },
      n('text', 'x9'),
    ];
    const [next] = removeNode(t, 'x9');
    const act = findNode(next, 'b9')?.props.onClick as {
      kind: string;
      target: string;
      chain?: unknown[];
    };
    expect(act.kind).toBe('runBinding');
    expect(act.target).toBe('f9');
    expect(act.chain).toHaveLength(1);
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

  it('duplicateNode 重映射子树内部 id 引用，外部引用保持原样', () => {
    const withRefs: PageNode[] = [
      n('fnTable', 'extTbl'), // 子树外部引用目标
      {
        ...n('container', 'grp', [
          {
            ...n('fnTable', 'tbl'),
            props: {
              title: 'tbl',
              sectionKey: 'tblVar',
              refreshOnNode: ['src'],
              rowActions: [{ label: '编辑', targetSection: 'mIn', params: {}, danger: false }],
            },
          },
          n('staticForm', 'src'),
          {
            ...n('button', 'go'),
            props: {
              title: 'go',
              onClick: {
                kind: 'openModal',
                target: 'mIn',
                chain: [{ kind: 'refreshNode', target: 'tbl' }],
              },
            },
          },
          {
            ...n('fnFields', 'det'),
            props: {
              title: 'det',
              inputAssignments: [
                { param: 'uid', kind: 'page_state', sourceNodeId: 'tbl', field: 'selectedRow/uid' },
                { param: 'kw', kind: 'page_state', sourceNodeId: 'extTbl', field: 'data/total' },
              ],
            },
          },
        ]),
      },
      n('modal', 'mIn', [n('fnForm', 'ffIn')]),
    ];
    const next = duplicateNode(withRefs, 'grp');
    const copyIdx = next.findIndex((x) => x.id === 'grp') + 1;
    const copy = next[copyIdx];
    const byType = (t: string) => copy.children!.find((c) => c.type === t)!;
    const tblCopy = byType('fnTable');
    const srcCopy = byType('staticForm');
    const goCopy = byType('button');
    const detCopy = byType('fnFields');

    expect(tblCopy.id).not.toBe('tbl');
    // 副本不继承声明 key（调用方 assignVarNames 重新命名）
    expect(tblCopy.props.sectionKey).toBeUndefined();
    // refreshOnNode 指向子树内节点 → 重映射到副本 id
    expect(tblCopy.props.refreshOnNode).toEqual([srcCopy.id]);
    // rowActions.targetSection 指向子树外弹窗 → 保持原样
    const ra = tblCopy.props.rowActions as Array<{ targetSection: string }>;
    expect(ra[0].targetSection).toBe('mIn');
    // 主动作 target 外部保持；链步骤 target 内部重映射
    const onClick = goCopy.props.onClick as {
      target: string;
      chain: Array<{ target: string }>;
    };
    expect(onClick.target).toBe('mIn');
    expect(onClick.chain[0].target).toBe(tblCopy.id);
    // inputAssignments.sourceNodeId：内部重映射、外部保持
    const ia = detCopy.props.inputAssignments as Array<{ sourceNodeId: string }>;
    expect(ia[0].sourceNodeId).toBe(tblCopy.id);
    expect(ia[1].sourceNodeId).toBe('extTbl');
    // 原节点引用不被改写
    expect(findNode(withRefs, 'tbl')?.props.refreshOnNode).toEqual(['src']);
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

  it('updateProps 无实际变更返回原引用（no-op 不污染撤销栈）', () => {
    // 空 patch
    expect(updateProps(tree, 'tbl1', {})).toBe(tree);
    // 值与现值相同
    expect(updateProps(tree, 'tbl1', { title: 'tbl1' })).toBe(tree);
    // 深层子节点命中但值不变 → 整树引用不变
    expect(updateProps(tree, 'btn1', { title: 'btn1' })).toBe(tree);
    // 目标 id 不存在
    expect(updateProps(tree, 'nope', { span: 12 })).toBe(tree);
  });

  it('updateProps 深层子节点真实变更返回新树', () => {
    const next = updateProps(tree, 'btn1', { title: 'btn1-x' });
    expect(next).not.toBe(tree);
    expect(findNode(next, 'btn1')?.props).toEqual({ title: 'btn1-x' });
    // 命中节点不变时其余层级保持引用（结构共享）
    expect(next[0]).toBe(tree[0]);
  });

  it('nodeId 唯一', () => {
    const ids = new Set(Array.from({ length: 100 }, () => nodeId('a')));
    expect(ids.size).toBe(100);
  });
});

describe('editor v3 model 追加覆盖', () => {
  it('nodeId 无参调用默认 n 前缀', () => {
    expect(nodeId()).toMatch(/^n-/);
  });

  it('scaffoldTabsNode：tabs 节点 + 两个空页签容器', () => {
    const node = scaffoldTabsNode();
    expect(node.type).toBe('tabs');
    expect(node.props).toEqual({ span: 24 });
    expect(node.children).toHaveLength(2);
    expect(node.children!.map((c) => c.type)).toEqual(['container', 'container']);
    expect(node.children![0].props).toEqual({ title: '页签 1', span: 24 });
    expect(node.children![1].props).toEqual({ title: '页签 2', span: 24 });
    expect(node.children!.every((c) => c.children)).toBe(true);
    expect(node.children!.every((c) => c.children!.length === 0)).toBe(true);
    const ids = [node.id, ...node.children!.map((c) => c.id)];
    expect(new Set(ids).size).toBe(3);
  });

  it('findInsertedSubtree：根级 / 嵌套 / 兄弟遍历 / 无新增', () => {
    const prev: PageNode[] = [n('text', 'a'), n('container', 'b', [n('text', 'b1')])];
    expect(findInsertedSubtree(prev, [...prev, n('text', 'new')])?.id).toBe('new');
    expect(findInsertedSubtree(prev, [n('text', 'new'), ...prev])?.id).toBe('new');
    // 嵌套子树内新增
    const nested: PageNode[] = [
      n('text', 'a'),
      n('container', 'b', [n('text', 'new'), n('text', 'b1')]),
    ];
    expect(findInsertedSubtree(prev, nested)?.id).toBe('new');
    // 第一个根的子树无新增 → 继续扫后续兄弟
    const sibling: PageNode[] = [...prev, n('text', 'later')];
    expect(findInsertedSubtree(prev, sibling)?.id).toBe('later');
    expect(findInsertedSubtree(prev, prev)).toBeUndefined();
    expect(findInsertedSubtree(prev, [...prev])).toBeUndefined();
  });

  it('replaceSubtree：根级与嵌套按 id 替换；目标不存在返回等价新数组', () => {
    const replacement = n('text', 't1');
    const nextRoot = replaceSubtree(tree, replacement);
    expect(nextRoot[0]).toBe(replacement);
    // 含 children 的兄弟节点被递归重建（结构等价，引用更新）
    expect(nextRoot[1]).toEqual(tree[1]);
    expect(nextRoot[1]).not.toBe(tree[1]);

    const inner = replaceSubtree(tree, n('button', 'btn1'));
    expect(findNode(inner, 'btn1')?.props).toEqual({ title: 'btn1' });
    expect(findNode(inner, 'btn1')).not.toBe(findNode(tree, 'btn1'));

    // 目标不存在：无节点被替换，返回内容等价的新数组（map 恒产新引用）
    const miss = replaceSubtree(tree, n('text', 'nope'));
    expect(miss).not.toBe(tree);
    expect(miss).toEqual(tree);
  });

  it('findParent 未命中返回 undefined', () => {
    expect(findParent(tree, 'nope')).toBeUndefined();
  });

  it('duplicateNode 嵌套目标：在父容器内复制并后插', () => {
    const next = duplicateNode(tree, 'btn1');
    const c1 = findNode(next, 'c1')!;
    expect(c1.children).toHaveLength(3);
    expect(c1.children!.slice(0, 2).map((c) => c.id)).toEqual(['tbl1', 'btn1']);
    const copy = c1.children![2];
    expect(copy.type).toBe('button');
    expect(copy.id).not.toBe('btn1');
    expect(countNodes(next)).toBe(countNodes(tree) + 1);
    // 原树不受影响
    expect(tree[1].children).toHaveLength(2);
  });

  it('moveNode 边界：负索引 / 原位 / 内层越界 no-op', () => {
    expect(moveNode(tree, 't1', -1)).toBe(tree);
    expect(moveNode(tree, 't1', 0)).toBe(tree);
    // 内层目标越界：外层容器仍被重建（walk 递归 map），但内容不变
    expect(moveNode(tree, 'tbl1', 9)).toEqual(tree);
  });

  it('pruneDanglingBindings：未知 kind / 空 target / 全有效链保持原值；rowActions 非数组走动作清洗', () => {
    const keepChain = {
      kind: 'runBinding',
      target: 'a',
      chain: [{ kind: 'refreshNode', target: 'a' }],
    };
    const rowActionsObj = { not: 'array' };
    const nodes: PageNode[] = [
      {
        ...n('button', 'a'),
        props: {
          title: 'a',
          unknownKind: { kind: 'teleport', target: 'ghost' },
          emptyTarget: { kind: 'openModal', target: '' },
          keepChain,
          rowActionsObj,
        },
      },
      { id: 'c', type: 'text', props: {} },
    ];
    const out = pruneDanglingBindings(nodes);
    expect(out[0].props.unknownKind).toBe(nodes[0].props.unknownKind);
    expect(out[0].props.emptyTarget).toEqual({ kind: 'openModal', target: '' });
    expect(out[0].props.keepChain).toBe(keepChain);
    // rowActions 非数组 → 按 props 普通值清洗（kind 非法 → 原样保留）
    expect(out[0].props.rowActionsObj).toBe(rowActionsObj);
    // 无变更无 children → 原节点引用
    expect(out[1]).toBe(nodes[1]);
  });

  it('pruneDanglingBindings：链部分悬空剔除步骤；链全部悬空置 undefined', () => {
    const partial = {
      kind: 'runBinding',
      target: 'b',
      chain: [{ target: 'b' }, { target: 'ghost' }],
    };
    const allGone = { kind: 'runBinding', target: 'b', chain: [{ target: 'ghost' }] };
    const nodes: PageNode[] = [
      {
        ...n('button', 'b'),
        props: { title: 'b', partial, allGone },
      },
      {
        ...n('fnTable', 't'),
        props: {
          title: 't',
          rowActions: [{ targetSection: 'ghost' }, { targetSection: 't' }, {}],
        },
      },
    ];
    const out = pruneDanglingBindings(nodes);
    expect(out[0].props.partial).toEqual({
      kind: 'runBinding',
      target: 'b',
      chain: [{ target: 'b' }],
    });
    expect(out[0].props.partial).not.toBe(partial);
    expect(out[0].props.allGone).toEqual({
      kind: 'runBinding',
      target: 'b',
      chain: undefined,
    });
    // 行操作：悬空 targetSection 剔除，空 targetSection 保留
    expect(out[1].props.rowActions).toEqual([{ targetSection: 't' }, {}]);
  });

  it('duplicateNode：chain 非数组不遍历；非 on 前缀对象与数组 props 原样保留', () => {
    const style = { color: 'red' };
    const src: PageNode[] = [
      {
        ...n('button', 'g'),
        props: {
          title: 'g',
          style,
          tags: ['a', 'b'],
          onClick: { kind: 'openModal', target: 'g', chain: 'oops' },
          refreshOnNode: ['g', 7] as unknown as string[],
        },
      },
    ];
    const next = duplicateNode(src, 'g');
    const copy = next[1];
    const onClick = copy.props.onClick as { target: string; chain: string };
    // 子树内部自引用 target 重映射；chain 非数组保持原样
    expect(onClick.target).toBe(copy.id);
    expect(onClick.chain).toBe('oops');
    expect(copy.props.style).toEqual(style);
    expect(copy.props.tags).toEqual(['a', 'b']);
    // refreshOnNode 混入非字符串：字符串重映射、非字符串原样
    expect(copy.props.refreshOnNode).toEqual([copy.id, 7]);
  });

  it('insertNode：父节点无 children / 父节点嵌套在容器内', () => {
    // 命中的父节点原本无 children → 追加为首个子节点
    const leafParent = insertNode(tree, n('text', 'x9'), 'f1');
    expect(findNode(leafParent, 'f1')?.children?.map((c) => c.id)).toEqual(['x9']);
    // 父节点嵌套：先递归进 c1 再命中 tbl1
    const nested = insertNode(tree, n('text', 'x8'), 'tbl1');
    expect(findNode(nested, 'tbl1')?.children?.map((c) => c.id)).toEqual(['x8']);
    // 兄弟容器结构不变
    expect(findNode(nested, 'c1')?.children).toHaveLength(2);
  });

  it('removeNode 嵌套目标：存活容器保留其余子节点', () => {
    const [next, removed] = removeNode(tree, 'btn1');
    expect(removed).toBe(true);
    expect(findNode(next, 'c1')?.children?.map((c) => c.id)).toEqual(['tbl1']);
    expect(countNodes(next)).toBe(4);
  });
});
