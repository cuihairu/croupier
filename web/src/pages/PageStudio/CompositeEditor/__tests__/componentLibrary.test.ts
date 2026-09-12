import {
  instantiateTemplate,
  instantiateTemplateDetailed,
  reconnectTemplateRefs,
  type ComponentTemplateDTO,
} from '../ComponentLibrary';

function tpl(tree: ComponentTemplateDTO['tree']): ComponentTemplateDTO {
  return {
    key: 'player-management',
    name: { 'zh-CN': '玩家管理' },
    requiredFunctions: ['player.list'],
    tree,
    builtin: false,
  };
}

describe('instantiateTemplate（实例化，U5）', () => {
  it('复制子树 + 重分配 id + 重映射内部引用', () => {
    const table = {
      id: 'fnTable-orig',
      type: 'fnTable' as const,
      props: {
        functionId: 'player.list',
        refreshOnNode: ['fnForm-orig'],
        rowActions: [{ label: '发邮件', targetSection: 'fnForm-orig' }],
      },
    };
    const nodes = instantiateTemplate(
      tpl([
        table,
        { id: 'fnForm-orig', type: 'fnForm' as const, props: { functionId: 'mail.send' } },
      ]),
    );
    expect(nodes).toHaveLength(2);
    expect(nodes[0].id).not.toBe('fnTable-orig');
    expect(nodes[1].id).not.toBe('fnForm-orig');
    // 内部引用重映射到新 id
    expect(nodes[0].props.refreshOnNode).toEqual([nodes[1].id]);
    expect((nodes[0].props.rowActions as Array<{ targetSection: string }>)[0].targetSection).toBe(
      nodes[1].id,
    );
  });

  it('模板内 sectionKey 不随实例复制（多实例各自分配，避免冲突）', () => {
    const nodes = instantiateTemplate(
      tpl([
        {
          id: 'fnTable-orig',
          type: 'fnTable' as const,
          props: { functionId: 'player.list', sectionKey: 'gold-rank' },
        },
      ]),
    );
    expect(nodes[0].props.sectionKey).toBeUndefined();
  });

  it('参数应用（U6）：值覆盖白名单 prop，缺省回退 default，未参数化不动', () => {
    const template = tpl([
      {
        id: 'fnTable-orig',
        type: 'fnTable' as const,
        props: { functionId: 'player.list', title: '玩家列表', autoRun: false },
      },
      {
        id: 'modal-orig',
        type: 'modal' as const,
        props: { title: '弹窗' },
        children: [
          {
            id: 'fnForm-orig',
            type: 'fnForm' as const,
            props: { functionId: 'mail.send', title: '发邮件' },
          },
        ],
      },
    ]);
    template.params = [
      { key: 'table.title', nodeId: 'fnTable-orig', prop: 'title', default: '默认列表' },
      { key: 'table.autoRun', nodeId: 'fnTable-orig', prop: 'autoRun', default: true },
      { key: 'form.title', nodeId: 'fnForm-orig', prop: 'title', default: '默认表单' },
    ];
    // 显式值覆盖；未提供 key（form.title）回退 default；引用重映射不受影响
    const nodes = instantiateTemplate(template, { 'table.title': 'VIP 列表' });
    expect(nodes[0].props.title).toBe('VIP 列表');
    expect(nodes[0].props.autoRun).toBe(true);
    const form = nodes[1].children![0];
    expect(form.props.title).toBe('默认表单');
    expect(form.props.functionId).toBe('mail.send');
  });

  it('无参数模板实例化行为不变', () => {
    const nodes = instantiateTemplate(
      tpl([
        {
          id: 'fnTable-orig',
          type: 'fnTable' as const,
          props: { functionId: 'player.list', title: '列表' },
        },
      ]),
    );
    expect(nodes[0].props.title).toBe('列表');
  });
});

describe('scanParamCandidates（参数化候选扫描，U6）', () => {
  it('title 恒列出，span/autoRun 存在才列，容器/文本跳过，子树递归', async () => {
    const { scanParamCandidates } = await import('../types');
    const nodes = [
      {
        id: 't1',
        type: 'fnTable' as const,
        props: { functionId: 'player.list', title: '列表', span: 24, autoRun: true },
      },
      {
        id: 'm1',
        type: 'modal' as const,
        props: { title: '弹窗' },
        children: [
          { id: 'f1', type: 'fnForm' as const, props: { functionId: 'mail.send', title: '表单' } },
        ],
      },
      { id: 'x1', type: 'text' as const, props: { content: 'hi' } },
    ];
    const candidates = scanParamCandidates(nodes);
    // t1: title+span+autoRun；modal 容器跳过但子节点 f1: title
    expect(candidates.map((c) => c.key)).toEqual(['t1.title', 't1.span', 't1.autoRun', 'f1.title']);
    expect(candidates[0].current).toBe('列表');
    expect(candidates[2].current).toBe(true);
  });
});

describe('悬空引用检出与重连（U7）', () => {
  /** 模板树：表格的各类引用指向「模板外」节点（outside-*，不在 tree 内）。 */
  const danglingTpl = () =>
    tpl([
      {
        id: 'tbl-orig',
        type: 'fnTable' as const,
        props: {
          title: '玩家列表',
          functionId: 'player.list',
          // 联动依赖指向模板外
          refreshOnNode: ['outside-filter', 'tbl-orig'],
          // 参数映射来源指向模板外
          inputAssignments: [
            {
              param: 'playerId',
              kind: 'page_state',
              sourceNodeId: 'outside-tbl',
              field: 'selectedRow/uid',
            },
          ],
          // 行操作弹窗目标指向模板外
          rowActions: [{ label: '发邮件', targetSection: 'outside-modal' }],
          // 动作主动作 + 链步骤目标指向模板外
          onRowClick: {
            kind: 'runBinding',
            target: 'outside-form',
            chain: [{ kind: 'refreshNode', target: 'outside-tbl' }],
          },
        },
      },
    ]);

  it('instantiateTemplateDetailed：四类悬空引用全部登记（nodeId=新 id、ref=旧值）', () => {
    const { nodes, dangling } = instantiateTemplateDetailed(danglingTpl());
    const newId = nodes[0].id;
    expect(dangling).toHaveLength(5);
    const by = (kind: string) => dangling.filter((d) => d.kind === kind);
    expect(by('refresh')).toHaveLength(1);
    expect(by('refresh')[0]).toMatchObject({
      nodeId: newId,
      prop: 'refreshOnNode',
      ref: 'outside-filter',
    });
    expect(by('assignment')).toHaveLength(1);
    expect(by('assignment')[0]).toMatchObject({ ref: 'outside-tbl' });
    expect(by('rowAction')).toHaveLength(1);
    expect(by('rowAction')[0]).toMatchObject({ ref: 'outside-modal' });
    expect(by('action')).toHaveLength(2);
    expect(by('action')[0]).toMatchObject({ prop: 'onRowClick', ref: 'outside-form' });
    expect(by('action')[1]).toMatchObject({ ref: 'outside-tbl', detail: 'chain 1' });
    // 实例内引用（tbl-orig 自身）正常重映射，不进悬空清单
    expect(nodes[0].props.refreshOnNode).toEqual(['outside-filter', newId]);
  });

  it('模板内引用不产生悬空（对照：全内部引用 dangling 为空）', () => {
    const { dangling } = instantiateTemplateDetailed(
      tpl([
        {
          id: 'a',
          type: 'fnTable' as const,
          props: { functionId: 'player.list', refreshOnNode: ['b'] },
        },
        { id: 'b', type: 'staticForm' as const, props: { title: '筛选' } },
      ]),
    );
    expect(dangling).toHaveLength(0);
  });

  it('reconnectTemplateRefs：按旧值替换四类引用，未选择的不动（纯函数）', () => {
    const { nodes } = instantiateTemplateDetailed(danglingTpl());
    const newId = nodes[0].id;
    const canvas = [
      { id: 'canvas-filter', type: 'staticForm' as const, props: { title: '筛选' } },
      ...nodes,
    ];
    const fixed = reconnectTemplateRefs(canvas, [
      {
        nodeId: newId,
        kind: 'refresh',
        prop: 'refreshOnNode',
        ref: 'outside-filter',
        target: 'canvas-filter',
      },
      {
        nodeId: newId,
        kind: 'assignment',
        prop: 'inputAssignments',
        ref: 'outside-tbl',
        target: 'canvas-filter',
      },
      {
        nodeId: newId,
        kind: 'rowAction',
        prop: 'rowActions',
        ref: 'outside-modal',
        target: 'canvas-filter',
      },
      {
        nodeId: newId,
        kind: 'action',
        prop: 'onRowClick',
        ref: 'outside-form',
        target: 'canvas-filter',
      },
    ]);
    const node = fixed.find((n) => n.id === newId)!;
    expect(node.props.refreshOnNode).toEqual(['canvas-filter', newId]);
    expect((node.props.inputAssignments as Array<{ sourceNodeId?: string }>)[0].sourceNodeId).toBe(
      'canvas-filter',
    );
    expect((node.props.rowActions as Array<{ targetSection?: string }>)[0].targetSection).toBe(
      'canvas-filter',
    );
    const action = node.props.onRowClick as { target: string; chain: Array<{ target: string }> };
    expect(action.target).toBe('canvas-filter');
    // 链步骤未重连（未在 fixes 里）：保留旧引用
    expect(action.chain[0].target).toBe('outside-tbl');
    // 纯函数：入参树未被修改
    expect(nodes[0].props.refreshOnNode).toEqual(['outside-filter', newId]);
  });

  it('reconnectTemplateRefs：空 fixes 返回原数组引用', () => {
    const { nodes } = instantiateTemplateDetailed(danglingTpl());
    expect(reconnectTemplateRefs(nodes, [])).toBe(nodes);
  });
});
