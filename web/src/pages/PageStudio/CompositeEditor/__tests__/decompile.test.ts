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
    // chain 仅含后续步骤（openModal 由 targetSection 隐式表达）
    expect(ta.chain).toEqual([{ kind: 'refreshNode', target: 'player.list' }]);
  });
});

describe('decompileToTree（区块 key 固化与参数映射反查，U5）', () => {
  it('sectionKey 固化：回读→再编译 key 逐项不变（含多实例）', () => {
    const spec: SpecSectionLike[] = [
      { key: 'player.list', functionId: 'player.list', view: 'table', autoRun: true },
      { key: 'player.list-2', functionId: 'player.list', view: 'table', autoRun: true },
      { key: 'vip.rank', functionId: 'player.list', view: 'fields' },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    // 再编译：三个 key 原样保持（不按树顺序重新分配）
    const { sections } = compileTree(tree);
    expect(sections.map((s) => s.key)).toEqual(['player.list', 'player.list-2', 'vip.rank']);
    // 删除首个实例后重编译：其余 key 不漂移
    const [, second, third] = tree;
    const pruned = compileTree([second, third]);
    expect(pruned.sections.map((s) => s.key)).toEqual(['player.list-2', 'vip.rank']);
  });

  it('inputAssignments 反查为上游节点 id（round-trip 不丢显式参数映射）', () => {
    const spec: SpecSectionLike[] = [
      { key: 'player.list', functionId: 'player.list', view: 'table' },
      {
        key: 'mail.send',
        functionId: 'mail.send',
        view: 'form',
        inputAssignments: [
          { target: '/playerId', kind: 'page_state', key: 'player.list', path: '/uid' },
          { target: '/reason', kind: 'literal', value: 'compensation' },
        ],
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const upstream = tree.find((n) => n.type === 'fnTable')!;
    const form = tree.find((n) => n.type === 'fnForm')!;
    const assignments = form.props.inputAssignments as Array<{
      param: string;
      sourceNodeId?: string;
      field?: string;
      kind: string;
    }>;
    // page_state 来源反查为真实节点 id（不再是恒等 key）
    expect(assignments[0].sourceNodeId).toBe(upstream.id);
    expect(assignments[0].field).toBe('uid');
    // 再编译：映射保留且 key 解析回 player.list
    const { sections } = compileTree(tree);
    expect(sections.find((s) => s.key === 'mail.send')!.inputAssignments).toEqual([
      { target: '/playerId', kind: 'page_state', key: 'player.list', path: '/uid' },
      { target: '/reason', kind: 'literal', value: 'compensation' },
    ]);
  });

  it('staticForm 回读固化 key 与 refreshOn，再编译保持', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'filter-panel',
        static: true,
        view: 'form',
        title: { 'zh-CN': '筛选' },
        refreshOn: ['player.list'],
        form: { jsonSchema: { type: 'object', properties: { kw: { type: 'string' } } } },
      },
      { key: 'player.list', functionId: 'player.list', view: 'table' },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const staticNode = tree.find((n) => n.type === 'staticForm')!;
    expect(staticNode.props.sectionKey).toBe('filter-panel');
    expect(staticNode.props.refreshOn).toEqual(['player.list']);
    const { sections } = compileTree(tree);
    const staticSection = sections.find((s) => s.static === true)!;
    expect(staticSection.key).toBe('filter-panel');
    expect(staticSection.refreshOn).toEqual(['player.list']);
  });

  it('参数映射来源 key 不存在时警告并保留字面引用', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'mail.send',
        functionId: 'mail.send',
        view: 'form',
        inputAssignments: [{ target: '/playerId', kind: 'page_state', key: 'ghost', path: '/id' }],
      },
    ];
    const [, warnings] = decompileToTree(spec);
    expect(warnings.some((w) => w.includes('ghost'))).toBe(true);
  });
});

/** 批次B回归：镜像后端 generator 真实变换（LocalizedText 包装 + 字段落位）
 * 的 round-trip——此前测试镜像只搬字段不包装，掩盖了三处数据毁坏：
 * 1) rowActions.label 回读→再编译变 "[object Object]"；
 * 2) dialog 区块（弹窗内 fnForm）events 回读静默丢失；
 * 3) onSuccess 双写为 events.success + onSuccessRefresh（发布端重复执行、
 *    round-trip 数组膨胀）。 */
describe('批次B：后端形态 round-trip（LocalizedText/dialog events/onSuccess 单一路径）', () => {
  const table: PageNode = {
    id: 'btbl',
    type: 'fnTable',
    props: {
      sectionKey: 'playerListTable',
      functionId: 'player.list',
      title: '玩家列表',
      span: 24,
      rowActions: [{ label: '发邮件', targetSection: 'bmodal', params: { playerId: 'row.uid' } }],
    },
  };
  const table2: PageNode = {
    id: 'btbl2',
    type: 'fnTable',
    props: {
      sectionKey: 'backupTable',
      functionId: 'player.list',
      title: '备份表',
      span: 24,
    },
  };
  const buildTree = (formExtra: Record<string, unknown>): PageNode[] => {
    const form: PageNode = {
      id: 'bform',
      type: 'fnForm',
      props: {
        sectionKey: 'mailSendForm',
        functionId: 'mail.send',
        title: '发邮件',
        display: 'dialog',
        ...formExtra,
      },
    };
    const modal: PageNode = {
      id: 'bmodal',
      type: 'modal',
      props: { sectionKey: 'mailSendModal', title: '发邮件弹窗', width: 'medium' },
      children: [form],
    };
    return [table, table2, modal];
  };

  /** 镜像后端变换：title/label 包装 LocalizedText（DefaultLocale=zh-CN）、
   * 请求顶层 rowActions/toolbarActions 落位 spec 的 table.rowActions /
   * toolbar.actions（generator 落库形态）。 */
  function mirrorBackend(sections: Array<Record<string, unknown>>): SpecSectionLike[] {
    return sections.map((s) => {
      const { rowActions, toolbarActions, ...rest } = s;
      const next: Record<string, unknown> = { ...rest };
      if (typeof next.title === 'string') next.title = { 'zh-CN': next.title };
      if (Array.isArray(rowActions) && rowActions.length) {
        next.table = {
          ...((next.table as Record<string, unknown>) ?? {}),
          rowActions: rowActions.map((ra) => ({
            ...(ra as Record<string, unknown>),
            label: { 'zh-CN': (ra as { label?: string }).label },
          })),
        };
      }
      if (Array.isArray(toolbarActions) && toolbarActions.length) {
        next.toolbar = {
          actions: toolbarActions.map((ta) => ({
            ...(ta as Record<string, unknown>),
            label: { 'zh-CN': (ta as { label?: string }).label },
          })),
        };
      }
      return next;
    }) as never;
  }

  it('rowActions.label 经 LocalizedText 包装后回读→再编译不变形（非 "[object Object]"）', () => {
    const { sections } = compileTree(buildTree({}));
    const stored = mirrorBackend(sections as unknown as Array<Record<string, unknown>>);
    const [tree] = decompileToTree(stored);
    const tbl = tree.find((n) => n.type === 'fnTable' && n.props.sectionKey === 'playerListTable')!;
    expect((tbl.props.rowActions as Array<{ label: unknown }>)[0].label).toBe('发邮件');
    const { sections: again } = compileTree(tree);
    const sec = again.find((s) => s.key === 'playerListTable')!;
    expect(sec.rowActions?.[0].label).toBe('发邮件');
  });

  it('dialog 内 fnForm 的 events 回读不丢（onSuccess 还原为深层节点 props）', () => {
    const { sections } = compileTree(
      buildTree({ onSuccess: { kind: 'refreshNode', target: 'btbl' } }),
    );
    const formSec = sections.find((s) => s.key === 'mailSendForm')!;
    expect(formSec.events?.some((e) => e.event === 'success')).toBe(true);
    const stored = mirrorBackend(sections as unknown as Array<Record<string, unknown>>);
    const [tree] = decompileToTree(stored);
    const modal = tree.find((n) => n.type === 'modal')!;
    const form = modal.children?.[0]!;
    const onSuccess = form.props.onSuccess as { kind: string; target: string } | undefined;
    expect(onSuccess?.kind).toBe('refreshNode');
    expect(onSuccess?.target).toBe(tree.find((n) => n.props.sectionKey === 'playerListTable')!.id);
  });

  it('onSuccess 只走 events——产物无双写，round-trip 不膨胀', () => {
    const tree = buildTree({ onSuccess: { kind: 'refreshNode', target: 'btbl' } });
    const { sections } = compileTree(tree);
    const formSec = sections.find((s) => s.key === 'mailSendForm')!;
    expect(formSec.onSuccessRefresh).toBeUndefined();
    expect(formSec.events?.filter((e) => e.event === 'success')).toHaveLength(1);
    // round-trip ×2 不膨胀
    let cur = tree;
    for (let i = 0; i < 2; i += 1) {
      const compiled = compileTree(cur);
      const mirrored = mirrorBackend(
        compiled.sections as unknown as Array<Record<string, unknown>>,
      );
      const sec = compiled.sections.find((s) => s.key === 'mailSendForm')!;
      expect(sec.onSuccessRefresh).toBeUndefined();
      expect(sec.events?.filter((e) => e.event === 'success')).toHaveLength(1);
      [cur] = decompileToTree(mirrored);
    }
  });

  it('遗留 onSuccessRefresh：未被 success 事件覆盖的目标保留、覆盖的去重', () => {
    // 无 onSuccess（纯遗留）：保留
    const legacyOnly = compileTree(
      buildTree({ onSuccessRefresh: { kind: 'refreshNode', target: 'btbl' } }),
    );
    expect(legacyOnly.sections.find((s) => s.key === 'mailSendForm')!.onSuccessRefresh).toEqual([
      'playerListTable',
    ]);
    // 与 success 事件同目标：去重（不再双写）
    const sameTarget = compileTree(
      buildTree({
        onSuccess: { kind: 'refreshNode', target: 'btbl' },
        onSuccessRefresh: { kind: 'refreshNode', target: 'btbl' },
      }),
    );
    expect(
      sameTarget.sections.find((s) => s.key === 'mailSendForm')!.onSuccessRefresh,
    ).toBeUndefined();
    // 不同目标：未覆盖者保留
    const diffTarget = compileTree(
      buildTree({
        onSuccess: { kind: 'refreshNode', target: 'btbl' },
        onSuccessRefresh: { kind: 'refreshNode', target: 'btbl2' },
      }),
    );
    expect(diffTarget.sections.find((s) => s.key === 'mailSendForm')!.onSuccessRefresh).toEqual([
      'backupTable',
    ]);
  });
});
