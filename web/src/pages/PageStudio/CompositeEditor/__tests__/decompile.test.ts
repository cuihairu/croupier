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

describe('decompileToTree V2：display=tab 双层聚合（group→tabs，标签→页）', () => {
  const tabSpec: SpecSectionLike[] = [
    {
      key: 'player.list',
      functionId: 'player.list',
      view: 'table',
      title: { 'zh-CN': '玩家列表' },
      display: 'tab',
      group: 'main-tabs',
      tab: { 'zh-CN': '列表' },
      autoRun: true,
    },
    {
      key: 'mail.send',
      functionId: 'mail.send',
      view: 'form',
      title: { 'zh-CN': '发邮件' },
      display: 'tab',
      group: 'main-tabs',
      tab: { 'zh-CN': '操作' },
    },
    {
      key: 'player.get',
      functionId: 'player.get',
      view: 'fields',
      display: 'tab',
      group: 'other-tabs',
      tab: { 'zh-CN': '详情' },
    },
  ];

  it('按 group 聚 tabs 节点（sectionKey 回写组名）、按 tab 标签聚页 container', () => {
    const [tree, warnings] = decompileToTree(tabSpec);
    expect(warnings).toEqual([]);
    const main = tree.find((n) => n.type === 'tabs' && n.props.sectionKey === 'main-tabs')!;
    expect(main).toBeDefined();
    expect((main.children ?? []).map((p) => p.props.title)).toEqual(['列表', '操作']);
    expect(main.children![0].children![0].type).toBe('fnTable');
    expect(main.children![1].children![0].type).toBe('fnForm');
    const other = tree.find((n) => n.type === 'tabs' && n.props.sectionKey === 'other-tabs')!;
    expect(other.children![0].children![0].type).toBe('fnFields');
    expect(tree).toHaveLength(2); // 两个组 → 两个 tabs 节点，无根级散块
  });

  it('同标签区块聚进同一页、无标签兜底 section key', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'a.fn',
        functionId: 'a.fn',
        view: 'form',
        display: 'tab',
        group: 'g1',
        tab: { 'zh-CN': '同页' },
      },
      {
        key: 'b.fn',
        functionId: 'b.fn',
        view: 'fields',
        display: 'tab',
        group: 'g1',
        tab: { 'zh-CN': '同页' },
      },
      {
        key: 'c.fn',
        functionId: 'c.fn',
        view: 'form',
        display: 'tab',
        group: 'g1',
        // 无 tab：兜底用 section key 作页标签
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const tabs = tree.find((n) => n.type === 'tabs')!;
    expect(tabs.children).toHaveLength(2);
    expect(tabs.children![0].children).toHaveLength(2);
    expect(tabs.children![1].props.title).toBe('c.fn');
  });

  it('static tab 区块回读为 staticForm 并路由进页（round-trip：组名/标签/schema 保持）', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'filter-panel',
        static: true,
        view: 'form',
        title: { 'zh-CN': '筛选' },
        display: 'tab',
        group: 'main-tabs',
        tab: { 'zh-CN': '筛选页' },
        form: { jsonSchema: { type: 'object', properties: { kw: { type: 'string' } } } },
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const tabs = tree.find((n) => n.type === 'tabs')!;
    const page = tabs.children![0];
    expect(page.props.title).toBe('筛选页');
    expect(page.children![0].type).toBe('staticForm');
    expect(page.children![0].props.sectionKey).toBe('filter-panel');
    // 再编译：static tab 区块组名/标签稳定
    const { sections } = compileTree(tree);
    const sec = sections.find((s) => s.key === 'filter-panel')!;
    expect(sec.display).toBe('tab');
    expect(sec.group).toBe('main-tabs');
    expect(sec.tab).toBe('筛选页');
  });

  it('tab 页内表格的 toolbar 按钮还原插回页内（不漂移到根级）', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'player.list',
        functionId: 'player.list',
        view: 'table',
        title: { 'zh-CN': '玩家列表' },
        display: 'tab',
        group: 'main-tabs',
        tab: { 'zh-CN': '列表' },
        toolbar: { actions: [{ label: { 'zh-CN': '刷新' } }] },
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const tabs = tree.find((n) => n.type === 'tabs')!;
    const page = tabs.children![0];
    // 按钮在页内表格之后（owningList 修复前会漂移到根级末尾）
    expect(page.children!.map((c) => c.type)).toEqual(['fnTable', 'button']);
    expect(String(page.children![1].props.title)).toBe('刷新');
  });

  it('round-trip：编译→镜像 LocalizedText→回读→再编译 group/tab 稳定', () => {
    const tabs: PageNode = {
      id: 'rt-tabs',
      type: 'tabs',
      props: { sectionKey: 'rt-group' },
      children: [
        {
          id: nodeId('container'),
          type: 'container',
          props: { title: '页A', span: 24 },
          children: [fn('fnTable', 'player.list', { autoRun: true })],
        },
        {
          id: nodeId('container'),
          type: 'container',
          props: { title: '页B', span: 24 },
          children: [fn('fnFields', 'player.get')],
        },
      ],
    };
    const { sections: first } = compileTree([tabs]);
    // 镜像后端：title/tab 包装 LocalizedText（generator 落库形态）
    const stored = first.map((s) => ({
      ...s,
      ...(typeof s.title === 'string' ? { title: { 'zh-CN': s.title } } : {}),
      ...(s.tab ? { tab: { 'zh-CN': s.tab } } : {}),
    })) as unknown as SpecSectionLike[];
    const [tree, warnings] = decompileToTree(stored);
    expect(warnings).toEqual([]);
    const rt = tree.find((n) => n.type === 'tabs')!;
    expect(rt.props.sectionKey).toBe('rt-group');
    expect((rt.children ?? []).map((p) => p.props.title)).toEqual(['页A', '页B']);
    const { sections: again } = compileTree(tree);
    expect(again.map((s) => [s.key, s.group, s.tab, s.display])).toEqual([
      ['player.list', 'rt-group', '页A', 'tab'],
      ['player.get', 'rt-group', '页B', 'tab'],
    ]);
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

describe('decompileToTree 缺省与降级形态补测', () => {
  it('区块缺少函数绑定：警告并跳过（含 bindingId 回退侧）', () => {
    const spec: SpecSectionLike[] = [
      // functionId 缺、bindingId 在 → fid 走 bindingId（不跳过）
      { key: 'by-binding', bindingId: 'b.fn', view: 'form' },
      // 两者皆缺 → 警告跳过
      { key: 'no-fn', view: 'form', refreshOn: ['by-binding'], events: [] },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings.some((w) => w.includes('区块缺少函数绑定'))).toBe(true);
    expect(tree).toHaveLength(1);
    // bindingId 回退节点正常产出（fnForm）
    expect(tree[0].type).toBe('fnForm');
    expect(String(tree[0].props.functionId)).toBe('b.fn');
    // 被跳过的 section：refreshOn owner 查不到（continue）、events owner 同样跳过
  });

  it('裸 section（无 key/无 fid）带 events 与 refreshOn：全部查无 owner 静默跳过', () => {
    const spec: SpecSectionLike[] = [
      {
        view: 'form',
        refreshOn: ['ghost-dep'],
        events: [{ event: 'click', action: { kind: 'navigate', target: 'ghost' } }],
      } as unknown as SpecSectionLike,
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings.some((w) => w.includes('区块缺少函数绑定'))).toBe(true);
    expect(tree).toHaveLength(0);
  });

  it('key 缺省：函数区块 key 回退 fid；static 区块 key 空串兜底「常量表单」', () => {
    const spec: SpecSectionLike[] = [
      { functionId: 'x.fn', view: 'form' },
      // static 无 key/无 form/非数字 span：三重兜底
      { static: true, view: 'form', span: 'not-a-number' } as unknown as SpecSectionLike,
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    expect(String(tree[0].props.sectionKey)).toBe('x.fn');
    const sf = tree[1];
    expect(sf.type).toBe('staticForm');
    expect(String(sf.props.title)).toBe('常量表单');
    expect(sf.props.span).toBe(12);
    expect(JSON.parse(String(sf.props.staticSchema))).toEqual({ type: 'object', properties: {} });
  });

  it('static card 区块：回读为 publishAs=card 容器（cardTitle 还原）', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'filter-card',
        static: true,
        view: 'form',
        title: { 'zh-CN': '筛选' },
        display: 'card',
        group: 'card-g1',
        cardTitle: { 'zh-CN': '高级筛选' },
        form: { jsonSchema: { type: 'object', properties: { kw: { type: 'string' } } } },
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const card = tree.find((n) => n.type === 'container')!;
    expect(card.props.publishAs).toBe('card');
    expect(String(card.props.title)).toBe('高级筛选');
    expect(String(card.props.sectionKey)).toBe('card-g1');
    expect(card.children![0].type).toBe('staticForm');
  });

  it('函数 card 区块：按 group 聚卡片容器（group 缺省回退 key）', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'vip.rank',
        functionId: 'vip.rank',
        view: 'fields',
        display: 'card',
        // group 缺省 → 回退 key
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const card = tree.find((n) => n.type === 'container')!;
    expect(card.props.publishAs).toBe('card');
    expect(card.children![0].type).toBe('fnFields');
  });

  it('同组第二个 dialog 表单：append 进同一 modal（group 缺省回退 key）', () => {
    const spec: SpecSectionLike[] = [
      // group 为合法 sectionKey 形态 → modal 回写 sectionKey（true 侧）
      {
        key: 'mail.send',
        group: 'mailModal',
        functionId: 'mail.send',
        view: 'form',
        display: 'dialog',
      },
      // 同组第二表单（append 分支）+ group 缺省侧由上一例兜底
      {
        key: 'mail.cc',
        group: 'mailModal',
        functionId: 'mail.cc',
        view: 'form',
        display: 'dialog',
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    expect(tree).toHaveLength(1);
    const modal = tree[0];
    expect(modal.type).toBe('modal');
    expect(String(modal.props.sectionKey)).toBe('mailModal');
    expect(modal.children).toHaveLength(2);
  });

  it('dialog group 非 sectionKey 形态：不回写 sectionKey（false 侧）', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'a.fn',
        group: '弹窗 组',
        functionId: 'a.fn',
        view: 'form',
        display: 'dialog',
      },
    ];
    const [tree] = decompileToTree(spec);
    expect(tree[0].props.sectionKey).toBeUndefined();
  });

  it('refreshOn 混合依赖：节点 id 进 refreshOnNode、陈旧字面量保留 refreshOn', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'mail.send',
        functionId: 'mail.send',
        view: 'form',
        refreshOn: ['player.list', 'ghost-literal'],
      },
      { key: 'player.list', functionId: 'player.list', view: 'table' },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const form = tree.find((n) => n.type === 'fnForm')!;
    const table = tree.find((n) => n.type === 'fnTable')!;
    expect(form.props.refreshOnNode).toEqual([table.id]);
    expect(form.props.refreshOn).toEqual(['ghost-literal']);
  });

  it('view=table 无 table 字段：columns 兜底空数组；rowActions 无 params 还原', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'a.list',
        functionId: 'a.list',
        view: 'table',
        table: { rowActions: [{ label: '看', targetSection: 'd1' }] },
      },
      { key: 'd.fn', group: 'd1', functionId: 'd.fn', view: 'form', display: 'dialog' },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const table = tree.find((n) => n.type === 'fnTable')!;
    expect(table.props.columns).toEqual([]);
    const ra = (table.props.rowActions as Array<Record<string, unknown>>)[0];
    expect(ra.label).toBe('看');
    expect(ra.params).toBeUndefined();
  });

  it('inputAssignments：page_state 无 key 不警告 + 多段数字路径转表达式字面值', () => {
    const spec: SpecSectionLike[] = [
      { key: 'p.list', functionId: 'p.list', view: 'table' },
      {
        key: 'm.send',
        functionId: 'm.send',
        view: 'form',
        inputAssignments: [
          // 无 key：不警告（upstreamId 空 → sourceNodeId 空串）
          { target: '/a', kind: 'page_state', path: '/uid' },
          // 多段含数字：/data/0/id → {{p.list.data[0].id}}
          { target: '/b', kind: 'page_state', key: 'p.list', path: '/data/0/id' },
          // transform default → defaultValue（U8 round-trip）
          {
            target: '/c',
            kind: 'literal',
            value: 'v',
            transform: { type: 'default', params: { value: 'dv' } },
          },
        ],
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const form = tree.find((n) => n.type === 'fnForm')!;
    const asg = form.props.inputAssignments as Array<Record<string, unknown>>;
    expect(asg[0]).toMatchObject({
      param: 'a',
      kind: 'page_state',
      sourceNodeId: '',
      field: 'uid',
    });
    expect(asg[1]).toEqual({
      param: 'b',
      kind: 'literal',
      value: '{{p.list.data[0].id}}',
      sourceNodeId: '',
      field: undefined,
      defaultValue: undefined,
    });
    expect(asg[2]).toMatchObject({ param: 'c', kind: 'literal', defaultValue: 'dv' });
  });

  it('events：未知事件名跳过；目标空串保持空；target 走 dialog key 映射', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'p.list',
        functionId: 'p.list',
        view: 'table',
        events: [
          { event: 'unknownEvent', action: { kind: 'navigate', target: '' } },
          {
            event: 'click',
            action: { kind: 'openModal', target: 'd.fn' },
          },
        ],
      },
      // dialog：group 缺省 → group=key='d.fn'（非法 sectionKey → modal 不回写）
      { key: 'd.fn', functionId: 'd.fn', view: 'form', display: 'dialog' },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const table = tree.find((n) => n.type === 'fnTable')!;
    const modal = tree.find((n) => n.type === 'modal')!;
    // 未知事件名被跳过；click 还原：mapTarget 按 key 优先映射（dialog key →
    // 其 fnForm 节点 id；group 名才映射 modal id）
    expect(table.props.onRowClick).toBeUndefined();
    const onClick = table.props.onClick as { kind: string; target: string };
    expect(onClick.kind).toBe('openModal');
    expect(onClick.target).toBe(modal.children![0].id);
  });

  it('onSuccessRefresh 目标缺失：警告丢弃（key 缺省回退 functionId）', () => {
    const spec: SpecSectionLike[] = [
      // 无 key：key 回退 functionId 仍可定位源
      { functionId: 'm.send', view: 'form', onSuccessRefresh: ['ghost'] },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings.some((w) => w.includes('ghost') && w.includes('无法还原'))).toBe(true);
    expect(tree[0].props.onSuccessRefresh).toBeUndefined();
  });

  it('toolbar 按钮：danger 还原 btnStyle、target 走 dialog key 回退、目标丢失警告丢弃', () => {
    const base = (actions: unknown[]): SpecSectionLike[] => [
      // 表格无 table 字段 → columns 空数组（同前），toolbar 在顶层
      {
        key: 'p.list',
        functionId: 'p.list',
        view: 'table',
        toolbar: { actions },
      } as unknown as SpecSectionLike,
      { key: 'd.fn', group: 'd1', functionId: 'd.fn', view: 'form', display: 'dialog' },
    ];
    // danger + targetSection 用 dialog key（group map miss → dialogKey 回退命中）
    const [tree1, w1] = decompileToTree(
      base([{ label: '封禁', danger: true, targetSection: 'd.fn', params: { id: 'x' } }]),
    );
    expect(w1).toEqual([]);
    const btn1 = tree1.find((n) => n.type === 'button')!;
    expect(btn1.props.btnStyle).toBe('danger');
    const onClick1 = btn1.props.onClick as { kind: string; target: string; params?: unknown };
    expect(onClick1.kind).toBe('openModal');
    expect(onClick1.target).toBe(tree1.find((n) => n.type === 'modal')!.id);
    expect(onClick1.params).toEqual({ id: 'x' });

    // 目标丢失：警告并丢弃按钮
    const [tree2, w2] = decompileToTree(base([{ label: '坏', targetSection: 'gone' }]));
    expect(w2.some((w) => w.includes('坏') && w.includes('无法还原'))).toBe(true);
    expect(tree2.find((n) => n.type === 'button')).toBeUndefined();
  });

  it('toolbar 按钮无弹窗目标带链：onClick 取链首步骤', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'p.list',
        functionId: 'p.list',
        view: 'table',
        toolbar: {
          actions: [
            {
              label: '刷新',
              chain: [
                { kind: 'refreshNode', target: 'p.list' },
                { kind: 'navigate', target: '', params: { url: '/x' } },
              ],
            },
          ],
        },
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const btn = tree.find((n) => n.type === 'button')!;
    const onClick = btn.props.onClick as {
      kind: string;
      target: string;
      chain?: Array<{ kind: string }>;
    };
    expect(onClick.kind).toBe('refreshNode');
    expect(onClick.chain).toEqual([{ kind: 'navigate', target: '', params: { url: '/x' } }]);
  });

  it('U10 visibleWhen 回读：合法还原（含数字段/exists 无 value）与非法形态警告丢弃', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'v.ok',
        functionId: 'v.ok',
        view: 'fields',
        visibleWhen: { key: 'src', path: '/data/0/mode', kind: 'equals', value: 'gold' },
      },
      {
        key: 'v.exists',
        functionId: 'v.exists',
        view: 'fields',
        visibleWhen: { key: 'src', path: '/values/kw', kind: 'exists' },
      },
      // kind 非法 → 警告丢弃
      {
        key: 'v.bad-kind',
        functionId: 'v.bad',
        view: 'fields',
        visibleWhen: { key: 'src', path: '/values/kw', kind: 'weird' },
      },
      // path 缺省（非字符串）→ 警告丢弃
      {
        key: 'v.bad-path',
        functionId: 'v.bad2',
        view: 'fields',
        visibleWhen: { key: 'src', kind: 'equals', value: 'x' },
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toHaveLength(2);
    expect(warnings.every((w) => w.includes('显示条件无法还原'))).toBe(true);
    const ok = tree.find((n) => String(n.props.sectionKey) === 'v.ok')!;
    expect(ok.props.visibleWhen).toEqual({
      expr: '{{src.data[0].mode}}',
      op: 'equals',
      value: 'gold',
    });
    const exists = tree.find((n) => String(n.props.sectionKey) === 'v.exists')!;
    expect(exists.props.visibleWhen).toEqual({ expr: '{{src.values.kw}}', op: 'exists' });
    expect(
      tree.find((n) => String(n.props.sectionKey) === 'v.bad-kind')!.props.visibleWhen,
    ).toBeUndefined();
  });
});

describe('decompileToTree 缺省与降级形态补测（二）', () => {
  it('tab group 非法 sectionKey 形态：tabs 不回写 sectionKey（仍聚组）', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'a.fn',
        functionId: 'a.fn',
        view: 'form',
        display: 'tab',
        group: 'tab 组 一',
        tab: { 'zh-CN': '页A' },
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const tabs = tree.find((n) => n.type === 'tabs')!;
    expect(tabs.props.sectionKey).toBeUndefined();
    expect(tabs.children![0].children![0].type).toBe('fnForm');
  });

  it('static tab group 缺省：空串兜底（tabs 不回写 sectionKey）', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'filter-panel',
        static: true,
        view: 'form',
        title: { 'zh-CN': '筛选' },
        display: 'tab',
        tab: { 'zh-CN': '筛选页' },
        form: { jsonSchema: { type: 'object', properties: { kw: { type: 'string' } } } },
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const tabs = tree.find((n) => n.type === 'tabs')!;
    expect(tabs.props.sectionKey).toBeUndefined();
    expect(tabs.children![0].children![0].type).toBe('staticForm');
  });

  it('card group 非法形态与 static card group 缺省：卡片容器不回写 sectionKey', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'vip.rank',
        functionId: 'vip.rank',
        view: 'fields',
        display: 'card',
        group: '卡片 组',
        cardTitle: { 'zh-CN': 'VIP' },
      },
      {
        key: 'filter-card',
        static: true,
        view: 'form',
        title: { 'zh-CN': '筛选' },
        display: 'card',
        cardTitle: { 'zh-CN': '高级筛选' },
        form: { jsonSchema: { type: 'object' } },
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const cards = tree.filter((n) => n.type === 'container');
    expect(cards).toHaveLength(2);
    expect(cards.every((c) => c.props.sectionKey === undefined)).toBe(true);
    expect(cards.map((c) => String(c.props.title))).toEqual(['VIP', '高级筛选']);
  });

  it('函数区块 span 非数字：整行兜底 24', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'a.fn',
        functionId: 'a.fn',
        view: 'form',
        span: 'not-a-number' as unknown as number,
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    expect(tree[0].props.span).toBe(24);
  });

  it('columns 项缺 key：过滤空列名', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'a.list',
        functionId: 'a.list',
        view: 'table',
        table: { columns: [{ key: 'uid' }, {}] },
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    expect(tree[0].props.columns).toEqual(['uid']);
  });

  it('static 区块带 visibleWhen：回读进 staticForm props', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'filter-panel',
        static: true,
        view: 'form',
        form: { jsonSchema: { type: 'object', properties: { kw: { type: 'string' } } } },
        visibleWhen: { kind: 'equals', key: 'src', path: '/values/mode', value: 'gold' },
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    expect(tree[0].props.visibleWhen).toEqual({
      expr: '{{src.values.mode}}',
      op: 'equals',
      value: 'gold',
    });
  });

  it('visibleWhen equals 缺 value：比较值兜底空串；无 key 警告参数空串', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'v.nov',
        functionId: 'v.nov',
        view: 'fields',
        visibleWhen: { kind: 'equals', key: 'src', path: '/a' },
      },
      {
        functionId: 'v.nok',
        view: 'fields',
        visibleWhen: { kind: 'weird', key: 's', path: '/a' },
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toHaveLength(1);
    // 警告 key 参数回退空串（区块「」）
    expect(warnings[0]).toContain('区块「」');
    const nov = tree.find((n) => String(n.props.sectionKey) === 'v.nov')!;
    expect(nov.props.visibleWhen).toEqual({ expr: '{{src.a}}', op: 'equals', value: '' });
  });

  it('同 key 重复区块 refreshOn：refreshOnNode 合并去重（existing 合并侧）', () => {
    const spec: SpecSectionLike[] = [
      { key: 'dup', functionId: 'a.fn', view: 'form', refreshOn: ['up.fn'] },
      { key: 'dup', functionId: 'a.fn', view: 'form', refreshOn: ['up.fn', 'up2.fn'] },
      { key: 'up.fn', functionId: 'up.fn', view: 'table' },
      { key: 'up2.fn', functionId: 'up2.fn', view: 'table' },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const up = tree.find((n) => n.type === 'fnTable' && n.props.sectionKey === 'up.fn')!;
    const up2 = tree.find((n) => n.type === 'fnTable' && n.props.sectionKey === 'up2.fn')!;
    const dupForms = tree.filter((n) => n.type === 'fnForm');
    expect(dupForms).toHaveLength(2);
    // 两轮都定位 keyToNodeId['dup']（后写覆盖 → dup2）：第二轮走 existing 合并
    const owner = dupForms.find((n) => n.props.refreshOnNode !== undefined)!;
    expect(new Set(owner.props.refreshOnNode as string[])).toEqual(new Set([up.id, up2.id]));
  });

  it('static refreshOn 含查无节点字面量：字面量保留合并（existing 侧）', () => {
    const spec: SpecSectionLike[] = [
      { key: 'f-panel', static: true, view: 'form', refreshOn: ['ghost-lit'] },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    // static props.refreshOn 初始即数组 → existingLiterals 合并去重后保持
    expect(tree[0].props.refreshOn).toEqual(['ghost-lit']);
  });

  it('inputAssignments 项缺 target：param 空串（单段与多段路径两侧）', () => {
    type AssignmentLike = NonNullable<SpecSectionLike['inputAssignments']>[number];
    const noTarget = (a: Omit<AssignmentLike, 'target'>) => a as AssignmentLike;
    const spec: SpecSectionLike[] = [
      { key: 'p.list', functionId: 'p.list', view: 'table' },
      {
        key: 'm.send',
        functionId: 'm.send',
        view: 'form',
        inputAssignments: [
          // 多段路径（表达式字面值分支）+ 无 target
          noTarget({ kind: 'page_state', key: 'p.list', path: '/data/0/id' }),
          // 单段路径 + 无 target
          noTarget({ kind: 'page_state', key: 'p.list', path: '/uid' }),
        ],
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const form = tree.find((n) => n.type === 'fnForm')!;
    const asg = form.props.inputAssignments as Array<Record<string, unknown>>;
    expect(asg[0]).toMatchObject({ param: '', kind: 'literal', value: '{{p.list.data[0].id}}' });
    expect(asg[1]).toMatchObject({ param: '', kind: 'page_state', field: 'uid' });
  });

  it('events 目标为未知 key：映射空串保留（不警告）', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'p.list',
        functionId: 'p.list',
        view: 'table',
        events: [{ event: 'click', action: { kind: 'navigate', target: 'ghost-key' } }],
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const onClick = tree[0].props.onClick as { kind: string; target: string };
    expect(onClick.target).toBe('');
  });

  it('rowActions 项缺 targetSection：警告丢弃', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'a.list',
        functionId: 'a.list',
        view: 'table',
        table: { rowActions: [{ label: '孤儿' }] },
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings.some((w) => w.includes('孤儿') && w.includes('无法还原'))).toBe(true);
    expect((tree[0].props.rowActions as unknown[]).length).toBe(0);
  });

  it('函数 tab 区块 group 缺省：空串兜底（tabs 不回写 sectionKey）', () => {
    const spec: SpecSectionLike[] = [
      { key: 'a.fn', functionId: 'a.fn', view: 'form', display: 'tab', tab: { 'zh-CN': '页A' } },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const tabs = tree.find((n) => n.type === 'tabs')!;
    expect(tabs.props.sectionKey).toBeUndefined();
    expect(tabs.children![0].children![0].type).toBe('fnForm');
  });

  it('无 key 区块带 events/refreshOn：owner 定位回退 functionId / bindingId', () => {
    const spec: SpecSectionLike[] = [
      // 无 key 有 fid：pendingEvents 与 refreshOn owner 都按 fid 定位
      {
        functionId: 'a.fn',
        view: 'form',
        refreshOn: ['up.fn'],
        events: [{ event: 'success', action: { kind: 'refreshNode', target: 'up.fn' } }],
      },
      // 无 key 无 fid：fid 回退 bindingId → key=b.fn
      { bindingId: 'b.fn', view: 'form', refreshOn: ['up.fn'] },
      { key: 'up.fn', functionId: 'up.fn', view: 'table' },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const forms = tree.filter((n) => n.type === 'fnForm');
    expect(forms).toHaveLength(2);
    const up = tree.find((n) => n.type === 'fnTable')!;
    // 两个 form 都还原 refreshOnNode（owner key 分别回退 fid/bindingId）
    expect(forms.every((f) => (f.props.refreshOnNode as string[]).length === 1)).toBe(true);
    // events owner 也按 fid 命中 → onSuccess 还原（target 映射 up 节点 id）
    const withEvent = forms.find((f) => f.props.onSuccess !== undefined)!;
    expect(withEvent.props.onSuccess).toEqual({ kind: 'refreshNode', target: up.id });
  });
});
