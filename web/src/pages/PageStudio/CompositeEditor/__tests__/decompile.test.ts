import { compileTree, decompileToTree, type SpecSectionLike } from '../compiler';
import { nodeId, type PageNode } from '../model';

function fn(type: PageNode['type'], fid: string, extra: Record<string, unknown> = {}): PageNode {
  return { id: nodeId(type), type, props: { functionId: fid, title: fid, span: 24, ...extra } };
}

describe('decompileToTree（回读编辑：spec→树）', () => {
  const spec: SpecSectionLike[] = [
    {
      key: 'player.list',
      functionId: 'player.list',
      view: 'table',
      title: { 'zh-CN': '玩家列表' },
      span: 24,
      autoRun: true,
      table: {
        columns: [{ key: 'uid' }, { key: 'gold' }],
        rowActions: [{ label: '发邮件', targetSection: 'mail.send', params: { playerId: 'uid' } }],
      },
    },
    {
      key: 'mail.send',
      group: 'modal-abc123',
      functionId: 'mail.send',
      view: 'form',
      title: { 'zh-CN': '发邮件' },
      display: 'dialog',
      onSuccessRefresh: ['player.list'],
    },
    {
      key: 'player.get',
      functionId: 'player.get',
      view: 'fields',
      autoRun: true,
    },
  ];

  it('结构还原：dialog→弹窗(含表单)，inline→组件，引用映射回节点 id', () => {
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const table = tree.find((n) => n.type === 'fnTable');
    const modal = tree.find((n) => n.type === 'modal');
    const fields = tree.find((n) => n.type === 'fnFields');
    expect(table).toBeDefined();
    expect(modal).toBeDefined();
    expect(fields).toBeDefined();
    expect(modal!.children![0].type).toBe('fnForm');
    // 行操作目标指向 modal 节点 id（非 section key）
    const ras = table!.props.rowActions as Array<{
      targetSection: string;
      params: Record<string, string>;
    }>;
    expect(ras[0].targetSection).toBe(modal!.id); // group → modal 节点 id
    expect(ras[0].params).toEqual({ playerId: 'uid' });
    // 成功刷新指向表格节点 id
    expect(modal!.children![0].props.onSuccessRefresh).toEqual({
      kind: 'refreshNode',
      target: table!.id,
    });
    // 列还原
    expect(table!.props.columns).toEqual(['uid', 'gold']);
  });

  it('回读→再编译 等价（关键配置不丢）', () => {
    const [tree] = decompileToTree(spec);
    const { sections, warnings } = compileTree(tree);
    expect(warnings).toEqual([]);
    const t = sections.find((x) => x.key === 'player.list')!;
    const f = sections.find((x) => x.key === 'mail.send')!;
    expect(t.rowActions).toHaveLength(1);
    expect(t.rowActions![0]).toMatchObject({ label: '发邮件', params: { playerId: 'uid' } });
    expect(t.rowActions![0].targetSection).toMatch(/^modal-/);
    expect(f.display).toBe('dialog');
    expect(f.onSuccessRefresh).toEqual(['player.list']);
  });

  it('不可还原的引用降级为警告', () => {
    const broken: SpecSectionLike[] = [
      {
        key: 'a',
        functionId: 'a.list',
        view: 'table',
        table: { rowActions: [{ label: 'x', targetSection: 'gone' }] },
      },
    ];
    const [tree, warnings] = decompileToTree(broken);
    expect(tree).toHaveLength(1);
    expect(warnings.some((w) => w.includes('无法还原'))).toBe(true);
    expect((tree[0].props.rowActions as unknown[]).length).toBe(0);
  });
});

describe('decompileToTree V3.2：events 与顶部按钮还原', () => {
  it('spec.events 还原为节点事件 props（rowClick→onRowClick，target group→modal 节点 id）', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'player.list',
        functionId: 'player.list',
        view: 'table',
        events: [
          {
            event: 'rowClick',
            action: { kind: 'openModal', target: 'modal-g1' },
            chain: [{ kind: 'navigate', target: '', params: { url: '/x' } }],
          },
        ],
      },
      {
        key: 'mail.send',
        group: 'modal-g1',
        functionId: 'mail.send',
        view: 'form',
        display: 'dialog',
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const table = tree.find((n) => n.type === 'fnTable')!;
    const modal = tree.find((n) => n.type === 'modal')!;
    const ev = table.props.onRowClick as {
      kind: string;
      target: string;
      chain: Array<{ kind: string; params?: Record<string, string> }>;
    };
    expect(ev.kind).toBe('openModal');
    expect(ev.target).toBe(modal.id);
    expect(ev.chain[0]).toEqual({ kind: 'navigate', target: '', params: { url: '/x' } });
  });

  it('toolbar.actions 还原为独立按钮节点（含链），round-trip 再编译等价', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'player.list',
        functionId: 'player.list',
        view: 'table',
        toolbar: {
          actions: [
            {
              label: { 'zh-CN': '发邮件' },
              targetSection: 'modal-g2',
              chain: [{ kind: 'refreshNode', target: 'player.list' }],
            },
          ],
        },
      },
      {
        key: 'mail.send',
        group: 'modal-g2',
        functionId: 'mail.send',
        view: 'form',
        display: 'dialog',
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const btn = tree.find((n) => n.type === 'button')!;
    expect(String(btn.props.title)).toBe('发邮件');
    const onClick = btn.props.onClick as { kind: string; target: string };
    expect(onClick.kind).toBe('openModal');
    // 再编译：顶部按钮+链 round-trip
    const { sections, warnings: w2 } = compileTree(tree);
    expect(w2).toEqual([]);
    const ta = sections.find((x) => x.key === 'player.list')!.toolbarActions![0];
    expect(ta).toMatchObject({ label: '发邮件' });
    expect(ta.chain).toEqual([{ kind: 'refreshNode', target: 'player.list' }]);
  });
});
